package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	Port = ":16543"
)

var (
	cilents    = make(map[net.Conn]*BallObj)
	cilents_Mu sync.RWMutex
)

// 对象池
var ballPool = sync.Pool{
	New: func() interface{} {
		return &BallObj{}
	},
}

type BallObj struct {
	X float64 `json:"X"`
	Y float64 `json:"Y"`
}

func main() {
	listener, err := net.Listen("tcp", Port)
	if err != nil {
		fmt.Println("Server start error:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Listening port", Port)

	go broadcast_position()

	//接收处理链接

	for {
		conn, err := listener.Accept()

		if err != nil {
			fmt.Println("Accept connection error:", err)
			continue
		}
		fmt.Println("Accept connection success:", conn.RemoteAddr())

		//加链接到map
		cilents_Mu.Lock()
		cilents[conn] = ballPool.Get().(*BallObj) //放回在handle_position中
		cilents_Mu.Unlock()

		go handle_position(conn)

	}

}

func broadcast_position() {
	for {
		time.Sleep(40 * time.Millisecond)
		cilents_Mu.RLock()
		if len(cilents) == 0 {
			cilents_Mu.RUnlock()
			continue
		}
		type clientInfo struct {
			conn net.Conn
			addr string
			X    int
			Y    int
		}
		var clientsToBroadcast []clientInfo
		for conn, ball := range cilents {
			clientsToBroadcast = append(clientsToBroadcast, clientInfo{
				conn: conn,
				addr: conn.RemoteAddr().String(),
				X:    int(ball.X),
				Y:    int(ball.Y),
			})
		}
		cilents_Mu.RUnlock() // 尽早释放锁

		positions := make(map[string]struct{ X, Y int })
		for _, c := range clientsToBroadcast {
			positions[c.addr] = struct{ X, Y int }{c.X, c.Y}
		}
		jsonStr, err := json.Marshal(positions)
		if err != nil {
			continue
		}
		jsonStr = append(jsonStr, '\n')

		for _, c := range clientsToBroadcast {
			if _, err := c.conn.Write(jsonStr); err != nil {
				// 处理错误，例如标记连接为关闭
			}
		}
	}
}
