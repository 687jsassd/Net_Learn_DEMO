package sp_var

var A_sp_var = 1000 //可导出

var b_sp_var = 2000 //不可导出

//关于包导入问题
/*
必须先在项目根目录用go mod init 进行模块初始化，才能在项目中导入其他包。


在Go 1.11+中，Go使用模块（Module）系统来管理依赖和导入路径。当你在项目根目录创建了go.mod文件后：

模块名称成为导入路径的前缀
相对路径从模块根目录开始计算
GOPATH不再是必须的，Go会根据go.mod文件定位包
*/
