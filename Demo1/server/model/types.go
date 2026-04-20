package model

type InGameMsg struct {
	BallID  uint16
	MsgType uint8
	MsgData []byte
}
