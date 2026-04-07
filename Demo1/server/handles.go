package main

import (
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
	cur_max_ball_id_Mu.Lock()
	cur_max_ball_id++ //先加，因为ID=0被认定为无效
	newID := cur_max_ball_id
	cur_max_ball_id_Mu.Unlock()

	//加Ball相关映射到map
	cilents_Mu.Lock()
	cilents[conn] = &BallObj{ID: newID}
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
		handle_position(conn)
	}()
	go func() {
		defer wg.Done()
		handle_write_to_client(conn)
	}()
	wg.Wait()
}

func handle_position(conn net.Conn) {
	buf := make([]byte, 4) // 固定读4字节（2+2 uint16）
	for {
		n, err := io.ReadFull(conn, buf)
		if err != nil || n != 4 {
			return
		}
		x := binary.LittleEndian.Uint16(buf[0:2])
		y := binary.LittleEndian.Uint16(buf[2:4])

		cilents_Mu.RLock()
		cilents[conn].SetXY(x, y)
		cilents_Mu.RUnlock()
	}
}

func handle_write_to_client(conn net.Conn) {
	for text := range writing_chans[conn] {
		if _, err := conn.Write(text); err != nil {
			return
		}
	}
}
