package ingame

import (
	"bytes"
	"demo1-server/config"
	"encoding/binary"
	"time"
)

var (
	broadcast_chan = make(chan []byte, config.Ingame_BroadCast_Channel_Size) //collect_positions的数据，通过该chan移交给spread_positions协程
)

func Ingame_process_loop() {
	//64tick处理一次
	//先处理消息，再收集位置，再广播，完成一次循环(1帧)
	go counts_and_clear_expired_sessions()
	for {
		time.Sleep(config.GameLoopInterval)

		// 只处理配置的每tick处理的消息数量，其他的下一帧再处理
		for i := 0; i < config.Ingame_Process_Message_Count_Per_Tick; i++ {
			select {
			case msg := <-Ingame_process_msg_chan:
				// 处理消息
				process_ingame_msg(*msg)
			default:
			}
		}
		// 收集位置
		collect_positions()
		// 广播位置
		Write_chan_msg_for_all_clients(&broadcast_chan)
	}
}

func collect_positions() {
	buf := new(bytes.Buffer)
	sessions := GetAllSessions()
	if len(sessions) == 0 {
		return
	}
	//快速收集拷贝一遍所有客户端的数据
	var positions []struct {
		X, Y uint16
		ID   uint16
	}
	for _, session := range sessions {
		ball := session.GetBall()
		if ball == nil {
			continue
		}
		x, y := ball.GetXY()
		positions = append(positions, struct {
			X, Y uint16
			ID   uint16
		}{x, y, session.BallID})
	}

	//写消息类型1,表示坐标同步
	msgType := uint8(1) //1字节
	_ = binary.Write(buf, binary.LittleEndian, msgType)

	//写消息长度
	msgLen := uint16(len(positions) * config.Single_BallPostion_Size) //2字节
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

func counts_and_clear_expired_sessions() {
	for {
		time.Sleep(time.Second)
		sessions := GetAllSessions()
		for _, session := range sessions {
			session.mu.RLock()
			remain := session.GetRemainExpiredTime()
			session.mu.RUnlock()
			if remain > 0 {
				session.SetRemainExpiredTime(int16(remain - 1))
			} else if remain == 0 {
				DestroySession(session.GetBallID())
			}
		}
	}
}
