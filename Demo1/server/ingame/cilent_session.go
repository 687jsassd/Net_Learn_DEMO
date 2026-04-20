package ingame

import (
	"fmt"
	"net"
	"sync"
)

type ClientSession struct {
	Conn                net.Conn //TCP连接
	BallID              uint16   //小球ID
	ball                *BallObj
	is_entered          bool  //是否已经加入战局
	remain_expired_time int16 //剩余过期时间(以秒计)
	writeChan           chan []byte
	done                chan struct{} //用于通知写协程退出
	mu                  sync.RWMutex
}

var (
	sessions    map[uint16]*ClientSession = make(map[uint16]*ClientSession) //使用小球ID作为key
	sessions_Mu sync.RWMutex
)

func GetSession(ballID uint16) (*ClientSession, bool) {
	sessions_Mu.RLock()
	defer sessions_Mu.RUnlock()
	session, ok := sessions[ballID]
	return session, ok
}
func NewSession(ballID uint16, ball *BallObj) *ClientSession {
	session := &ClientSession{
		BallID:              ballID,
		ball:                ball,
		writeChan:           make(chan []byte, 32), //32是元素条数，不是字节数。
		done:                make(chan struct{}),
		remain_expired_time: -1, //-1表示未设置过期时间
	}
	sessions_Mu.Lock()
	sessions[ballID] = session
	sessions_Mu.Unlock()
	return session
}

func DestroySession(ballID uint16) {
	sessions_Mu.Lock()
	defer sessions_Mu.Unlock()

	if s, ok := sessions[ballID]; ok {
		if s.writeChan != nil {
			select {
			case <-s.writeChan:
			default:
			}
			close(s.writeChan)
			s.writeChan = nil
		}
		if s.done != nil {
			close(s.done)
			s.done = nil
		}

		delete(sessions, ballID)
		ReturnBallID(ballID)
		fmt.Printf("DestroySession: %d\n", ballID)
	}
}

func GetAllSessions() []*ClientSession {
	sessions_Mu.RLock()
	defer sessions_Mu.RUnlock()
	sess := make([]*ClientSession, 0, len(sessions))
	for _, s := range sessions {
		sess = append(sess, s)
	}
	return sess
}

func (s *ClientSession) ReplaceConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Conn != nil {
		s.Conn.Close()
	}
	s.Conn = conn
}

func (s *ClientSession) GetConn() net.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Conn
}

func (s *ClientSession) GetBall() *BallObj {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ball
}

func (s *ClientSession) GetBallID() uint16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.BallID
}

func (s *ClientSession) IsEntered() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.is_entered
}

func (s *ClientSession) SwitchEntered(to bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.is_entered = to
}

func (s *ClientSession) SetRemainExpiredTime(interval int16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remain_expired_time = interval
}

func (s *ClientSession) GetRemainExpiredTime() int16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.remain_expired_time
}
