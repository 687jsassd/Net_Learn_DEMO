package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
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
	newID := cur_max_ball_id.Add(1)

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
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		//fmt.Println("Received line:", line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		//解析json,从中获取X和Y字段，然后使用带锁的SetXY方法更新
		var pos struct {
			X float64 `json:"X"`
			Y float64 `json:"Y"`
		}
		if err := json.Unmarshal([]byte(line), &pos); err != nil {
			fmt.Printf("JSON解析失败: %v, 数据: %s\n", err, line)
			continue
		}

		cilents_Mu.RLock()
		cilents[conn].SetXY(pos.X, pos.Y)
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
