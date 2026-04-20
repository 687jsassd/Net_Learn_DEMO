package ingame

import (
	"demo1-server/config"
	"demo1-server/model"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

var Ingame_process_msg_chan chan *model.InGameMsg = make(chan *model.InGameMsg, config.Ingame_Message_Channel_Size)

func Handle_connection(conn net.Conn) {
	if conn == nil {
		return
	}
	remoteAddr := conn.RemoteAddr().String()
	fmt.Println("Client online:", remoteAddr)

	//校验
	ballID, is_new, err := readClientAuth(conn)
	if err != nil || (ballID == 0 && !is_new) {
		fmt.Println("Client auth failed:", err)
		conn.Close()
		return
	}

	//分配会话
	var session *ClientSession
	if ballID != 0 {
		var ok bool
		session, ok = GetSession(ballID)
		if !ok {
			fmt.Println("No session for ballID:", ballID)
			newID := GetBallID()
			if newID == 0 {
				fmt.Println("No available ball ID")
				conn.Close()
				return
			}
			ball := &BallObj{ID: newID}
			session = NewSession(newID, ball)
			fmt.Println("Started a new session for old client:", newID)
		}
		session.ReplaceConn(conn)
		session.SetRemainExpiredTime(-1) //上号后重置过期时间
		fmt.Println("Old Client Session success:", ballID)
	} else {
		newID := GetBallID()
		if newID == 0 {
			fmt.Println("No available ball ID")
			conn.Close()
			return
		}
		ball := &BallObj{ID: newID}
		session = NewSession(newID, ball)
		session.ReplaceConn(conn)
		fmt.Println("New Client Session success:", newID)
	}

	//开读写协程
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		receive_client_msg(session)
	}()
	go func() {
		defer wg.Done()
		handle_write_to_client(session)
	}()
	wg.Wait()

	//关闭会话
	session.SwitchEntered(false)
	session.ReplaceConn(nil)
	session.SetRemainExpiredTime(config.Ingame_Session_Expired_Time)
	conn.Close()
	fmt.Println("A Client offline:", remoteAddr)
}

func readClientAuth(conn net.Conn) (uint16, bool, error) {
	//认证协议：
	//新客户端发送 0x01 0x0000
	//旧客户端发送 0x02 + 小球ID
	buf := make([]byte, 3)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return 0, false, err
	}
	authType := buf[0]
	ballID := binary.LittleEndian.Uint16(buf[1:3])
	switch authType {
	case 0x01:
		return 0, true, nil
	case 0x02:
		return ballID, false, nil
	default:
		return 0, false, fmt.Errorf("unknown auth type: %d", authType)
	}
}

func receive_client_msg(session *ClientSession) {
	for {

		select {
		case <-session.done:
			return
		default:
		}

		conn := session.GetConn()
		if conn == nil {
			return
		}

		buf := make([]byte, 3) //消息头和长度信息
		//读1字节uint8的消息类型
		n, err := io.ReadFull(conn, buf[0:1])
		if err != nil || n != 1 {
			return
		}
		msgType := buf[0]
		//读2字节uint16的消息长度
		n, err = io.ReadFull(conn, buf[1:3])
		if err != nil || n != 2 {
			return
		}
		msgLen := binary.LittleEndian.Uint16(buf[1:3])
		//剩下的是消息体
		msgData := make([]byte, msgLen)
		n, err = io.ReadFull(conn, msgData)
		if err != nil || n != int(msgLen) {
			return
		}
		//构造InGameMsg,送chan
		msg := model.InGameMsg{
			BallID:  session.GetBallID(),
			MsgType: msgType,
			MsgData: msgData,
		}
		//放不进去就不放了，算丢弃
		select {
		case Ingame_process_msg_chan <- &msg:
		default:
		}
	}
}

func handle_write_to_client(session *ClientSession) {
	for {
		select {
		case data, ok := <-session.writeChan:
			if !ok {
				return
			}
			conn := session.GetConn()
			if conn == nil {
				return
			}
			if _, err := conn.Write(data); err != nil {
				return
			}
		case <-session.done:
			return
		}
	}
}

func Write_message_for_client(session *ClientSession, binData []byte) {
	//这是通用的单客户端广播函数
	if session.GetConn() == nil {
		return
	}
	select {
	case session.writeChan <- binData:
	default:
		fmt.Println("Write to client_w_chan failed:", session.GetBallID())
	}
}

func Write_diff_msg_for_spc_client(session *ClientSession, spc_binData []byte, other_binData []byte) {
	//为指定会话写指定消息，其他的写其他消息
	sessions := GetAllSessions()
	if len(sessions) == 0 {
		return
	}
	for _, s := range sessions {
		if s.GetConn() == nil {
			continue
		}
		if s == session {
			select {
			case s.writeChan <- spc_binData:
			default:
				fmt.Println("Write to spc_conn failed:", s.GetBallID())
			}
		} else {
			select {
			case s.writeChan <- other_binData:
			default:
				fmt.Println("Write to other_conn failed:", s.GetBallID())
			}
		}
	}
}

func Write_chan_msg_for_all_clients(bindata_chan *chan []byte) {
	//这是通用的全量广播函数，不作区分
	sessions := GetAllSessions()
	if len(sessions) == 0 {
		return
	}
	for i := 0; i < config.Ingame_BroadCast_Channel_Size; i++ {
		select {
		case binData := <-*bindata_chan:
			for _, session := range sessions {
				if session.GetConn() == nil {
					continue
				}
				select {
				case session.writeChan <- binData:
				default:
					fmt.Println("Write to client_w_chan failed:", session.GetBallID())
				}
			}
		default:
		}
	}
}
