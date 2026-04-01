package main

const AA = "你好"  //常量
var BB = "hello" //变量

func fun1() {
	//AA+="1" //常量不可修改
	BB += "1" //变量可以修改
}

/*
cannot assign to AA (neither addressable nor a map index expression)

报错:
常量不可寻址，这是设计。


关于map index：
是map 的索引表达式（m[key]）
不可寻址，也就是说&m[999]报错，但可以赋值。
map是哈希表，底层结构在运行时动态调整，因此不可寻址，不然找到的指针会随时变化，导致错误。
赋值是瞬间操作，不会导致后继问题。

注：Go的哈希表可以动态扩容，这与我们自己实现的静态固定大小的哈希表有不同，造成动态与静态的区别，
*/
