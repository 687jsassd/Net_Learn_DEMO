package main

import (
	"fmt"
	"vars/sp_var"
)

func main() {
	fmt.Println(A_support_vars) //这个变量在support_vars.go中定义
	fmt.Println(b)              //同理

	fmt.Println(sp_var.A_sp_var) //这个变量从sp_var包中导入
	//fmt.Println(sp_var.b_sp_var) //这个变量从sp_var包中导入，但是不可导出，所以不能直接访a问

}

//这里可用是因为，两个go文件都在一个包main中，文件和包有差异
