package ingame

import (
	"demo1-server/config"
	"demo1-server/practial_funcs"
	"sync"
)

type BallObj struct {
	X  uint16
	Y  uint16
	ID uint16 //分配一个球ID，方便识别，而且避免传输时直接传送IP地址
	mu sync.Mutex
}

func (b *BallObj) GetXY() (uint16, uint16) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.X, b.Y
}
func (b *BallObj) SetXY(x, y uint16) { //虽然单个是原子操作，但是俩就不是了，得加锁！
	b.mu.Lock()
	defer b.mu.Unlock()
	b.X = x
	b.Y = y
}

func (b *BallObj) Move(moveDirection uint8) { //移动球
	curX, curY := b.GetXY()

	ballRadius := uint16(config.BallRadius)
	width := uint16(config.Width)
	height := uint16(config.Height)

	step := uint16(50)
	if moveDirection >= 11 && moveDirection <= 18 {
		step = 100
	}
	switch moveDirection {
	case 1, 11: // 上 / 加速上
		curY = practial_funcs.SafeSub(curY, step, ballRadius)
	case 2, 12: // 下 / 加速下
		curY = practial_funcs.SafeAdd(curY, step, height-ballRadius)
	case 3, 13: // 左 / 加速左
		curX = practial_funcs.SafeSub(curX, step, ballRadius)
	case 4, 14: // 右 / 加速右
		curX = practial_funcs.SafeAdd(curX, step, width-ballRadius)
	case 5, 15: // 左上 / 加速左上
		curX = practial_funcs.SafeSub(curX, step, ballRadius)
		curY = practial_funcs.SafeSub(curY, step, ballRadius)
	case 6, 16: // 右上 / 加速右上
		curX = practial_funcs.SafeAdd(curX, step, width-ballRadius)
		curY = practial_funcs.SafeSub(curY, step, ballRadius)
	case 7, 17: // 左下 / 加速左下
		curX = practial_funcs.SafeSub(curX, step, ballRadius)
		curY = practial_funcs.SafeAdd(curY, step, height-ballRadius)
	case 8, 18: // 右下 / 加速右下
		curX = practial_funcs.SafeAdd(curX, step, width-ballRadius)
		curY = practial_funcs.SafeAdd(curY, step, height-ballRadius)
	default:
		return
	}
	b.SetXY(curX, curY)
}
