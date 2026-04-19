package model

import "net"

type InGameMsg struct {
	Conn    net.Conn
	MsgType uint8
	MsgData []byte
}
