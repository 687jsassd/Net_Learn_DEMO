package ingame

import (
	"bytes"
	"demo1-server/model"
	"encoding/binary"
	"fmt"
)

func process_ingame_msg(msg model.InGameMsg) {
	//根据BallID获取ClientSession
	session, ok := GetSession(msg.BallID)
	if !ok || session == nil {
		return
	}
	sessionBallID := session.GetBallID()
	if msg.BallID != sessionBallID {
		return
	}
	ball := session.GetBall()
	if ball == nil {
		return
	}

	switch msg.MsgType {
	case 1: //玩家加入
		// 消息体为空，返回给玩家消息格式如下：
		// - msgtype = 2为玩家进入
		// 消息体为：
		// - 玩家ID(服务端权威ID) uint16 2字节
		// - 是否是客户端自身的加入 bool 1字节
		// - X轴坐标 uint16 2字节 默认是6400(注意由于最后一位表示小数，实际是640.0)
		// - Y轴坐标 uint16 2字节 默认是3600
		// 消息体长:7字节

		//构造对客户端自身的消息，因为包含是否是客户端自身的加入，所以需要区分
		//通过ball坐标检验初始化情况
		if x, y := ball.GetXY(); x == 0 && y == 0 {
			ball.SetXY(6400, 3600)
		}
		x, y := ball.GetXY()
		buf := new(bytes.Buffer)
		_ = binary.Write(buf, binary.LittleEndian, uint8(2))
		_ = binary.Write(buf, binary.LittleEndian, uint16(7))
		_ = binary.Write(buf, binary.LittleEndian, sessionBallID)
		_ = binary.Write(buf, binary.LittleEndian, true)
		_ = binary.Write(buf, binary.LittleEndian, uint16(x))
		_ = binary.Write(buf, binary.LittleEndian, uint16(y))
		binData := buf.Bytes()
		buf_other := new(bytes.Buffer)
		_ = binary.Write(buf_other, binary.LittleEndian, uint8(2))
		_ = binary.Write(buf_other, binary.LittleEndian, uint16(7))
		_ = binary.Write(buf_other, binary.LittleEndian, sessionBallID)
		_ = binary.Write(buf_other, binary.LittleEndian, false)
		_ = binary.Write(buf_other, binary.LittleEndian, uint16(x))
		_ = binary.Write(buf_other, binary.LittleEndian, uint16(y))
		other_binData := buf_other.Bytes()
		session.SwitchEntered(true)

		Write_diff_msg_for_spc_client(session, binData, other_binData)
	case 2: //移动指令
		// - msgtype = 2为小球移动
		// 消息体包括：
		// - 玩家ID uint16 2字节
		// - 移动方式 uint8 1字节
		// 消息体长:3字节
		// 检查是否是客户端自身的移动指令
		if !session.IsEntered() {
			return
		}
		// 检查玩家ID是否匹配
		player_id := binary.LittleEndian.Uint16(msg.MsgData[0:2])
		if player_id != sessionBallID {
			return
		}

		move_direction := msg.MsgData[2]
		ball.Move(move_direction)
	default:
		fmt.Println("Unknown message type:", msg.MsgType)
	}
}
