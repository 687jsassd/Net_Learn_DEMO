package config

import (
	"time"
)

const (
	//服务器配置相关
	Port             = ":16543"              //服务器端口
	GameLoopInterval = 16 * time.Millisecond //近似64tick

	//消息相关
	Single_BallPostion_Size = 6 //2字节X，2字节Y，2字节ID

	//游戏场景相关
	BallRadius = 200   //小球半径 20*10 =200
	Width      = 12800 //游戏窗口宽度 1280
	Height     = 8000  //游戏窗口高度 800
)
