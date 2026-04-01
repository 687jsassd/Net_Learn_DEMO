package main

import (
	"fmt"
	"strconv"
)

var A = 100            //可略type
var B string = "hello" //完整声明

var ( //全局变量用，多变量声明
	C = 1000
	D = "hello world"
)
var E, F, G uint64 = 1000, 20000, 999999999999 //多变量声明，加type就相同类型

func main() {
	a := 100              //局部变量用海象可行
	b, c := "hello", true //局部变量不能不使用，但是全局变量可以声明但不使用,不加type类型可以不同
	fmt.Printf(strconv.Itoa(a))
	if c {
		fmt.Printf(b)
	}

	a, C = C, a //相同类型，可以交换值

}
