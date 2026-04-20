package config

import (
	"time"
)

const (
	//服务器配置相关
	Port             = ":16543"              //服务器端口
	GameLoopInterval = 16 * time.Millisecond //近似64tick

	Ingame_Message_Channel_Size           = 2000 //游戏消息通道大小
	Ingame_Process_Message_Count_Per_Tick = 2000 //每tick处理的消息数量
	Ingame_BroadCast_Channel_Size         = 2000 //广播消息通道大小,同时影响广播消息的循环次数
	Ingame_Session_Expired_Time           = 10   //会话过期清除时间，单位秒

	//消息相关
	Single_BallPostion_Size = 6 //2字节X，2字节Y，2字节ID

	//游戏场景相关
	BallRadius = 200   //小球半径 20*10 =200
	Width      = 12800 //游戏窗口宽度 1280
	Height     = 8000  //游戏窗口高度 800
)
