package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

func handle_position(conn net.Conn) {
	defer func() {
		ballPool.Put(cilents[conn])
		cilents_Mu.Lock()
		delete(cilents, conn)
		cilents_Mu.Unlock()
		conn.Close()
		fmt.Println("Client offline:", conn.RemoteAddr())
	}()

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("cilent offline OR read line error:", err)
			return
		}
		//fmt.Println("Received line:", line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		//解析json
		if err := json.Unmarshal([]byte(line), cilents[conn]); err != nil {
			fmt.Printf("JSON解析失败: %v, 数据: %s\n", err, line)
			continue
		}
		//fmt.Printf("客户端 %s 位置更新: X=%.2f, Y=%.2f\n",
		//	conn.RemoteAddr().String(), cilents[conn].X, cilents[conn].Y)
	}

}
