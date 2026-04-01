package main

import (
	"fmt"
)

func main() {
	var a = 100
	var b string = "hello"

	var need_to_format = "你好:%s,数值是:%d \n"

	fmt.Printf(need_to_format, b, a)               //Printf格式化输出，而Println直接输出
	fmt.Println(fmt.Sprintf(need_to_format, b, a)) //Sprintf格式化输出，返回字符串

	fmt.Printf(" 分割 ")
	fmt.Printf(" 分割 ")   //注意，Printf不会自动换行，需要手动添加换行符\n
	fmt.Println(" 分割2 ") //Println会自动换行
	fmt.Println(" 分割2 ")
}

/*
Go 字符串格式化符号:

格  式	描  述
%v	按值的本来值输出
%+v	在 %v 基础上，对结构体字段名和值进行展开
%#v	输出 Go 语言语法格式的值
%T	输出 Go 语言语法格式的类型和值
%%	输出 % 本体
%b	整型以二进制方式显示
%o	整型以八进制方式显示
%d	整型以十进制方式显示
%x	整型以十六进制方式显示
%X	整型以十六进制、字母大写方式显示
%U	Unicode 字符
%f	浮点数
%p	指针，十六进制方式显示

%s 字符串
%.2f 浮点数，保留2位小数


%s：字符串格式，可以使用以下对齐参数：
%s：默认对齐方式，左对齐。
%10s：指定宽度为 10 的右对齐。
%-10s：指定宽度为 10 的左对齐。
%d：整数格式，可以使用以下对齐参数：
%d：默认对齐方式，右对齐。
%10d：指定宽度为 10 的右对齐。
%-10d：指定宽度为 10 的左对齐。
%f：浮点数格式，可以使用以下对齐参数：
%f：默认对齐方式，右对齐。
%10f：指定宽度为 10 的右对齐。
%-10f：指定宽度为 10 的左对齐。
*/
