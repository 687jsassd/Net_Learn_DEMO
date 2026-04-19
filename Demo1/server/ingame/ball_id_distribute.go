package ingame

import (
	"sync"
)

//本文件用于小球ID的分配和重用
//主要采用递增的cur_max_ball_id和重用栈的方式完成。

var (
	cur_max_ball_id        uint16
	reusable_ball_id_stack []uint16
	id_distribute_Mu       sync.Mutex
)

func GetBallID() uint16 {
	id_distribute_Mu.Lock()
	defer id_distribute_Mu.Unlock()

	if len(reusable_ball_id_stack) > 0 {
		id := reusable_ball_id_stack[len(reusable_ball_id_stack)-1]
		reusable_ball_id_stack = reusable_ball_id_stack[:len(reusable_ball_id_stack)-1]
		return id
	} else {
		if cur_max_ball_id == 65535 {
			return 0 //0是无效ID，用于表示没有可用ID
		}
		cur_max_ball_id++
		return cur_max_ball_id //不返回0，至少返回1,有效值1-65535
	}
}

func ReturnBallID(id uint16) {
	id_distribute_Mu.Lock()
	defer func() {
		id_distribute_Mu.Unlock()
	}()
	if id > 0 {
		reusable_ball_id_stack = append(reusable_ball_id_stack, id)
	}
}
