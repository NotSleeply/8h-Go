package server

import (
	"encoding/json"
	"net"
	"sync"
	"time"
)

type User struct {
	Name       string
	Addr       string
	C          chan string
	Conn       net.Conn
	Server     *Server
	closeOnce  sync.Once
	logoutOnce sync.Once
	mu         sync.Mutex // 保护通道关闭状态
	isClosed   bool       // 标记通道是否关闭
}

// 初始化
func NewUser(conn net.Conn, s *Server) *User {
	userAddr := conn.RemoteAddr().String()
	user := &User{
		Name:   userAddr,
		Addr:   userAddr,
		C:      make(chan string, 10),
		Conn:   conn,
		Server: s,
	}
	go user.ListenMessage()
	return user
}

// 写入消息
func (u *User) writeWithPrompt(msg string) error {
	u.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := u.Conn.Write([]byte(msg + "\n")); err != nil {
		return err
	}
	if _, err := u.Conn.Write([]byte("> ")); err != nil {
		return err
	}
	return nil
}

// 监听用户信息
func (u *User) ListenMessage() {
	defer func() {
		if r := recover(); r != nil {
			// 忽略 panic（例如向已关闭的 channel 发送）
		}
	}()
	for msg := range u.C {
		if err := u.writeWithPrompt(msg); err != nil {
			u.Logout()
			return
		}
	}
}

// 退出
func (u *User) Close() {
	u.closeOnce.Do(func() {
		u.mu.Lock()
		u.isClosed = true
		u.mu.Unlock()
		close(u.C)
		u.Conn.Close()
	})
}

// 上线
func (u *User) Online() {
	u.Server.MapLock.Lock()
	u.Server.OnlineMap[u.Name] = u
	u.Server.MapLock.Unlock()
	u.Server.EnqueuePendingForUser(u.Name, 500)

	serverMsgID, seq := u.Server.BroadcastSystemEvent(u.Name+" ✅ 已上线！", u.Name)

	sysMsg := &Message{
		Type:        TypeDeliver,
		ServerMsgID: serverMsgID,
		From:        "system",
		ChatID:      "system",
		Seq:         seq,
		Body:        "----------------👏欢迎来到 8H-Go 聊天室----------------",
	}
	u.SendJSON(sysMsg)
}

// 下线
func (u *User) Offline() {
	u.Server.MapLock.Lock()
	delete(u.Server.OnlineMap, u.Name)
	u.Server.MapLock.Unlock()

	serverMsgID, seq := u.Server.BroadcastSystemEvent(u.Name+" ❌ 已下线！", u.Name)

	sysMsg := &Message{
		Type:        TypeDeliver,
		ServerMsgID: serverMsgID,
		From:        "system",
		ChatID:      "system",
		Seq:         seq,
		Body:        "你已成功下线，欢迎下次再来。",
	}
	// 触发者可能已断开（视调用时机决定是否发送）
	u.SendJSON(sysMsg)
}

// 注销用户：下线并释放资源
func (u *User) Logout() {
	u.logoutOnce.Do(func() {
		u.Offline()
		u.Close()
	})
}

// 私发消息
func (u *User) SendMsg(msg string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.isClosed {
		return
	}
	select {
	case u.C <- msg:
		u.Server.markOutboundMessage()
	default:
		// 缓冲已满跳过阻塞（队列头阻塞保护），网络拥塞视为异常，主动下线
		go u.Logout()
	}
}

// 对协议消息进行封送处理，并将其发送到用户的通道.
func (u *User) SendJSON(m *Message) {
	if m == nil {
		return
	}
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	u.SendMsg(string(b))
}
