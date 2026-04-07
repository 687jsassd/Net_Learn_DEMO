package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	Port                    = ":16543"
	CollectInterval         = 16 * time.Millisecond //近似64tick
	Single_BallPostion_Size = 6                     //2字节X，2字节Y，2字节ID
)

var (
	cur_max_ball_id    uint16 //当前最大球ID，暂时用这个
	cur_max_ball_id_Mu sync.Mutex

	broadcast_chan = make(chan []byte, 10) //collect_positions的数据，通过该chan移交给spread_positions协程

	cilents    = make(map[net.Conn]*BallObj)
	cilents_Mu sync.RWMutex

	writing_chans    = make(map[net.Conn]chan []byte) //为每个链接开一个chan，以写入数据
	writing_chans_Mu sync.RWMutex
)

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
	listener, err := net.Listen("tcp", Port)
	if err != nil {
		fmt.Println("Server start error:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Listening port", Port)

	go collect_positions() //收集需要广播的数据
	go spread_positions()  //分发数据到各个连接的写channel

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

func collect_positions() {
	buf := new(bytes.Buffer)
	for {
		time.Sleep(CollectInterval)
		buf.Reset()

		cilents_Mu.RLock()
		clients_nums := len(cilents) //收集一下数量，后面分配切片大小直接分配
		if clients_nums == 0 {
			cilents_Mu.RUnlock()
			continue
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
}

func spread_positions() {
	for binData := range broadcast_chan {
		writing_chans_Mu.Lock()
		for conn, ch := range writing_chans {
			select {
			case ch <- binData:
			default:
				fmt.Println("Write to client_w_chan failed:", conn.RemoteAddr())
			}
		}
		writing_chans_Mu.Unlock()
	}
}
