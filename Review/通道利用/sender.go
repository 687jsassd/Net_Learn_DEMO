package main

import (
	"fmt"
	"math/rand"
	"time"
)

// 每0.2-2秒发送一个数据到通道,秒数随机
func send_data() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("send_data panic:", err)
		}
	}()

	for i := 0; i < 100; i++ {
		ch <- i
		time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)
	}

}
