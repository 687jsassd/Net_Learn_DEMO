package main

import (
	"demo1-server/config"
	"demo1-server/ingame"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // 下划线表示仅导入初始化，无需调用
	"runtime"
)

func main() {
	// 开启阻塞采样 (1=记录所有阻塞)
	runtime.SetBlockProfileRate(1)
	// 开启锁竞争采样 (1=记录所有锁竞争)
	runtime.SetMutexProfileFraction(1)
	go func() {
		fmt.Println("pprof性能分析服务启动：http://127.0.0.1:6060/debug/pprof/")
		_ = http.ListenAndServe(":6060", nil)
	}()
	listener, err := net.Listen("tcp", config.Port)
	if err != nil {
		fmt.Println("Server start error:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Listening port", config.Port)

	go ingame.Ingame_process_loop()

	//接收处理链接

	for {
		conn, err := listener.Accept()

		if err != nil {
			fmt.Println("Accept connection error:", err)
			continue
		}
		fmt.Println("Accept connection success:", conn.RemoteAddr())
		go ingame.Handle_connection(conn)
	}

}
