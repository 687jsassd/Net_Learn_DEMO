package main

import (
	"demo1-server/ingame"
	"demo1-server/model"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

func handle_connection(conn net.Conn) {
	defer func() {
		//清理读map
		cilents_Mu.Lock()
		delete(cilents, conn)
		cilents_Mu.Unlock()

		//清理写map并关通道
		writing_chans_Mu.Lock()
		if ch, ok := writing_chans[conn]; ok {
			close(ch)
			delete(writing_chans, conn)
		}
		writing_chans_Mu.Unlock()

		//关链接
		conn.Close()
		fmt.Println("Client offline:", conn.RemoteAddr())
	}()

	//获取ID
	newID := ingame.GetBallID()
	defer ingame.ReturnBallID(newID)
	if newID == 0 {
		fmt.Println("No available ball ID")
		return
	}

	//加Ball相关映射到map
	cilents_Mu.Lock()
	cilents[conn] = &ingame.BallObj{ID: newID}
	cilents_Mu.Unlock()

	//开一个写chan
	writing_chans_Mu.Lock()
	writing_chans[conn] = make(chan []byte, 32) //32是元素条数，不是字节数。
	writing_chans_Mu.Unlock()

	//等协程退出
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		receive_client_msg(conn)
	}()
	go func() {
		defer wg.Done()
		handle_write_to_client(conn)
	}()
	wg.Wait()
}

func receive_client_msg(conn net.Conn) {
	for {
		buf := make([]byte, 3) //消息头和长度信息
		//读1字节uint8的消息类型
		n, err := io.ReadFull(conn, buf[0:1])
		if err != nil || n != 1 {
			return
		}
		msgType := buf[0]
		//读2字节uint16的消息长度
		n, err = io.ReadFull(conn, buf[1:3])
		if err != nil || n != 2 {
			return
		}
		msgLen := binary.LittleEndian.Uint16(buf[1:3])
		//剩下的是消息体
		msgData := make([]byte, msgLen)
		n, err = io.ReadFull(conn, msgData)
		if err != nil || n != int(msgLen) {
			return
		}
		//构造InGameMsg,送chan
		msg := model.InGameMsg{
			Conn:    conn,
			MsgType: msgType,
			MsgData: msgData,
		}
		//放不进去就不放了，算丢弃
		select {
		case ingame_process_msg_chan <- &msg:
		default:
		}
	}
}

func handle_write_to_client(conn net.Conn) {
	for text := range writing_chans[conn] {
		if _, err := conn.Write(text); err != nil {
			return
		}
		//fmt.Println("Sent message:", text, "to", conn.RemoteAddr())
	}
}
