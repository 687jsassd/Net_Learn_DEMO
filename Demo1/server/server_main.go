package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // 下划线表示仅导入初始化，无需调用
	"sync"
	"time"
)

const (
	Port                    = ":16543"
	GameLoopInterval        = 16 * time.Millisecond //近似64tick
	Single_BallPostion_Size = 6                     //2字节X，2字节Y，2字节ID

	BallRadius = 200   //小球半径 20*10 =200
	Width      = 12800 //游戏窗口宽度 1280
	Height     = 8000  //游戏窗口高度 800
)

var (
	cur_max_ball_id    uint16 //当前最大球ID，暂时用这个
	cur_max_ball_id_Mu sync.Mutex

	broadcast_chan = make(chan []byte, 2000) //collect_positions的数据，通过该chan移交给spread_positions协程

	cilents    = make(map[net.Conn]*BallObj)
	cilents_Mu sync.RWMutex

	writing_chans    = make(map[net.Conn]chan []byte) //为每个链接开一个chan，以写入数据
	writing_chans_Mu sync.RWMutex

	ingame_process_msg_chan = make(chan *InGameMsg, 2000)
)

type InGameMsg struct {
	Conn    net.Conn
	MsgType uint8
	MsgData []byte
}

type BallObj struct {
	X  uint16
	Y  uint16
	ID uint16 //分配一个球ID，方便识别，而且避免传输时直接传送IP地址
	mu sync.Mutex
}

func (b *BallObj) GetXY() (uint16, uint16) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.X, b.Y
}
func (b *BallObj) SetXY(x, y uint16) { //虽然单个是原子操作，但是俩就不是了，得加锁！
	b.mu.Lock()
	defer b.mu.Unlock()
	b.X = x
	b.Y = y
}

func main() {
	go func() {
		fmt.Println("pprof性能分析服务启动：http://127.0.0.1:6060/debug/pprof/")
		_ = http.ListenAndServe(":6060", nil)
	}()
	listener, err := net.Listen("tcp", Port)
	if err != nil {
		fmt.Println("Server start error:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Listening port", Port)

	go ingame_process_loop()

	//接收处理链接

	for {
		conn, err := listener.Accept()

		if err != nil {
			fmt.Println("Accept connection error:", err)
			continue
		}
		fmt.Println("Accept connection success:", conn.RemoteAddr())
		go handle_connection(conn)
	}

}

func ingame_process_loop() {
	//64tick处理一次
	//先处理消息，再收集位置，再广播，完成一次循环(1帧)
	for {
		time.Sleep(GameLoopInterval)

		// 只处理2000条消息，其他的下一帧再处理
		for i := 0; i < 2000; i++ {
			select {
			case msg := <-ingame_process_msg_chan:
				// 处理消息
				process_ingame_msg(*msg)
			default:
			}
		}
		// 收集位置
		collect_positions()
		// 广播位置
		write_chan_msg_for_all_clients()
	}
}

func process_ingame_msg(msg InGameMsg) {
	switch msg.MsgType {
	case 1: //玩家加入
		// 消息体为空，返回给玩家消息格式如下：
		// - msgtype = 2为玩家进入
		// 消息体为：
		// - 玩家ID uint16 2字节 （从cilents里面获取BallObj的ID就行）
		// - 是否是客户端自身的加入 bool 1字节
		// - X轴坐标 uint16 2字节 默认是6400(注意由于最后一位表示小数，实际是640.0)
		// - Y轴坐标 uint16 2字节 默认是3600
		// 消息体长:7字节

		//构造对客户端自身的消息，因为包含是否是客户端自身的加入，所以需要区分
		cilents_Mu.RLock()
		buf := new(bytes.Buffer)
		_ = binary.Write(buf, binary.LittleEndian, uint8(2))
		_ = binary.Write(buf, binary.LittleEndian, uint16(7))
		_ = binary.Write(buf, binary.LittleEndian, cilents[msg.Conn].ID)
		_ = binary.Write(buf, binary.LittleEndian, true)
		_ = binary.Write(buf, binary.LittleEndian, uint16(6400))
		_ = binary.Write(buf, binary.LittleEndian, uint16(3600))
		binData := buf.Bytes()
		buf_other := new(bytes.Buffer)
		_ = binary.Write(buf_other, binary.LittleEndian, uint8(2))
		_ = binary.Write(buf_other, binary.LittleEndian, uint16(7))
		_ = binary.Write(buf_other, binary.LittleEndian, cilents[msg.Conn].ID)
		_ = binary.Write(buf_other, binary.LittleEndian, false)
		_ = binary.Write(buf_other, binary.LittleEndian, uint16(6400))
		_ = binary.Write(buf_other, binary.LittleEndian, uint16(3600))
		other_binData := buf_other.Bytes()
		cilents_Mu.RUnlock()
		cilents_Mu.Lock()
		cilents[msg.Conn].SetXY(6400, 3600)
		cilents_Mu.Unlock()
		write_diff_msg_for_spc_client(msg.Conn, binData, other_binData)
	case 2: //移动指令
		// - msgtype = 2为小球移动
		// 消息体包括：
		// - 移动方式 uint8 1字节
		// 消息体长:1字节

		//无需发送给客户端消息，这里根据收到的做同步即可
		move_direction := msg.MsgData[0]
		cilents_Mu.RLock()
		cur_x, cur_y := cilents[msg.Conn].GetXY()
		cilents_Mu.RUnlock()
		//使用函数作安全加减，避免溢出问题
		SafeSub := func(x, n, min uint16) uint16 {
			if x <= n || x-n < min {
				return min
			}
			return x - n
		}
		SafeAdd := func(x, n, max uint16) uint16 {
			if x+n > max {
				return max
			}
			return x + n
		}
		switch move_direction {
		case 1: //上
			cur_y = SafeSub(cur_y, 50, BallRadius)
		case 2: //下
			cur_y = SafeAdd(cur_y, 50, Height-BallRadius)
		case 3: //左
			cur_x = SafeSub(cur_x, 50, BallRadius)
		case 4: //右
			cur_x = SafeAdd(cur_x, 50, Width-BallRadius)
		case 5: //左上
			cur_x = SafeSub(cur_x, 50, BallRadius)
			cur_y = SafeSub(cur_y, 50, BallRadius)
		case 6: //右上
			cur_x = SafeAdd(cur_x, 50, Width-BallRadius)
			cur_y = SafeSub(cur_y, 50, BallRadius)
		case 7: //左下
			cur_x = SafeSub(cur_x, 50, BallRadius)
			cur_y = SafeAdd(cur_y, 50, Height-BallRadius)
		case 8: //右下
			cur_x = SafeAdd(cur_x, 50, Width-BallRadius)
			cur_y = SafeAdd(cur_y, 50, Height-BallRadius)
		case 11: //加速上
			cur_y = SafeSub(cur_y, 100, BallRadius)
		case 12: //加速下
			cur_y = SafeAdd(cur_y, 100, Height-BallRadius)
		case 13: //加速左
			cur_x = SafeSub(cur_x, 100, BallRadius)
		case 14: //加速右
			cur_x = SafeAdd(cur_x, 100, Width-BallRadius)
		case 15: //加速左上
			cur_x = SafeSub(cur_x, 100, BallRadius)
			cur_y = SafeSub(cur_y, 100, BallRadius)
		case 16: //加速右上
			cur_x = SafeAdd(cur_x, 100, Width-BallRadius)
			cur_y = SafeSub(cur_y, 100, BallRadius)
		case 17: //加速左下
			cur_x = SafeSub(cur_x, 100, BallRadius)
			cur_y = SafeAdd(cur_y, 100, Height-BallRadius)
		case 18: //加速右下
			cur_x = SafeAdd(cur_x, 100, Width-BallRadius)
			cur_y = SafeAdd(cur_y, 100, Height-BallRadius)
		}
		cilents_Mu.Lock()
		cilents[msg.Conn].SetXY(cur_x, cur_y)
		cilents_Mu.Unlock()

	default:
		fmt.Println("Unknown message type:", msg.MsgType)
	}
}

func collect_positions() {
	buf := new(bytes.Buffer)
	cilents_Mu.RLock()
	clients_nums := len(cilents)
	if clients_nums == 0 {
		cilents_Mu.RUnlock()
		return
	}
	//快速收集拷贝一遍所有客户端的数据
	var positions []struct {
		X, Y uint16
		ID   uint16
	}
	for _, ball := range cilents {
		if ball.ID == 0 {
			continue
		}
		x, y := ball.GetXY()
		positions = append(positions, struct {
			X, Y uint16
			ID   uint16
		}{x, y, ball.ID})
	}
	cilents_Mu.RUnlock()

	//写消息类型1,表示坐标同步
	msgType := uint8(1) //1字节
	_ = binary.Write(buf, binary.LittleEndian, msgType)

	//写消息长度
	msgLen := uint16(len(positions) * Single_BallPostion_Size) //2字节
	_ = binary.Write(buf, binary.LittleEndian, msgLen)

	//遍历positions，按小端序写
	for _, pos := range positions {
		_ = binary.Write(buf, binary.LittleEndian, pos.ID)
		_ = binary.Write(buf, binary.LittleEndian, pos.X)
		_ = binary.Write(buf, binary.LittleEndian, pos.Y)
	}
	binData := buf.Bytes()

	select {
	case broadcast_chan <- binData:
	default:
		select {
		case <-broadcast_chan:
		default:
		}
		broadcast_chan <- binData
	}

}

func write_chan_msg_for_all_clients() {
	//这是通用的全量广播函数，不作区分
	for i := 0; i < 125; i++ {
		select {
		case binData := <-broadcast_chan:
			writing_chans_Mu.RLock()
			for conn, ch := range writing_chans {
				select {
				case ch <- binData:
				default:
					fmt.Println("Write to client_w_chan failed:", conn.RemoteAddr())
				}
			}
			writing_chans_Mu.RUnlock()
		default:
		}
	}
}

func write_message_for_client(conn net.Conn, binData []byte) {
	//这是通用的单客户端广播函数
	writing_chans_Mu.RLock()
	select {
	case writing_chans[conn] <- binData:
	default:
		fmt.Println("Write to client_w_chan failed:", conn.RemoteAddr())
	}
	writing_chans_Mu.RUnlock()
}

func write_diff_msg_for_spc_client(spc_conn net.Conn, spc_binData []byte, other_binData []byte) {
	//为指定连接写指定消息，其他的写其他消息
	writing_chans_Mu.RLock()
	for conn, ch := range writing_chans {
		if conn == spc_conn {
			select {
			case ch <- spc_binData:
			default:
				fmt.Println("Write to spc_conn failed:", conn.RemoteAddr())
			}
		} else {
			select {
			case ch <- other_binData:
			default:
				fmt.Println("Write to other_conn failed:", conn.RemoteAddr())
			}
		}
	}
	writing_chans_Mu.RUnlock()
}
