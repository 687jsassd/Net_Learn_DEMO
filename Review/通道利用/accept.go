package main

import (
	"fmt"
	"time"
)

var ch = make(chan int, 100)

const TIMELIMIT = 100 * time.Second //超时时间，单位秒
const LOGINTERVAL = 4 * time.Second //日志间隔，单位秒

// 等待通道数据，同时用计时机制，超时就关闭通道
func main() {
	accepted_data_num := 0
	timeTicker := time.NewTicker(TIMELIMIT)
	logTicker := time.NewTicker(LOGINTERVAL)
	defer timeTicker.Stop()
	defer logTicker.Stop()

	go send_data()

	for {
		select {
		case data := <-ch:
			fmt.Printf("Accepted data:%d\n", data)
			accepted_data_num++

		case <-timeTicker.C:
			fmt.Printf("Timeout\n")
			close(ch)
			return

		case <-logTicker.C:
			fmt.Printf("Log:%d, Accepted data num:%d\n", time.Now().Unix(), accepted_data_num)
		}

	}
}
