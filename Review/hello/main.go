package main //包声明

import ( //引入包
	"fmt"
)

func main() { //函数
	fmt.Println("Hello") //语句
	println("World")
}

/*
Go 语言的基础组成有以下几个部分：
包声明
引入包
函数
变量
语句 & 表达式
注释

使用 go build 构建二进制文件,生成exe
使用 go run 直接运行代码

println不适用于正式环境，不保证跨平台一致性和格式化输出等
*/
