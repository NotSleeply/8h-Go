package session

import (
	"encoding/json"
	"net"
	"sync"
	"time"

	iface "tet/src/iface"
	"tet/src/protocol"
)

type User struct {
	Name       string
	Addr       string
	C          chan string
	Conn       net.Conn
	Server     iface.ServerAPI
	closeOnce  sync.Once
	logoutOnce sync.Once
	mu         sync.Mutex
	isClosed   bool
}

func NewUser(conn net.Conn, srv iface.ServerAPI) *User {
	userAddr := conn.RemoteAddr().String()
	user := &User{
		Name:   userAddr,
		Addr:   userAddr,
		C:      make(chan string, 10),
		Conn:   conn,
		Server: srv,
	}
	go user.ListenMessage()
	return user
}

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

func (u *User) ListenMessage() {
	defer func() {
		if r := recover(); r != nil {
		}
	}()
	for msg := range u.C {
		if err := u.writeWithPrompt(msg); err != nil {
			u.Logout()
			return
		}
	}
}

func (u *User) Close() {
	u.closeOnce.Do(func() {
		u.mu.Lock()
		u.isClosed = true
		u.mu.Unlock()
		close(u.C)
		u.Conn.Close()
	})
}

func (u *User) Online() {
	// register in server
	if u.Server != nil {
		u.Server.RegisterOnline(u.Name, u, u.Addr)
		u.Server.EnqueuePendingForUser(u.Name, 500)
		serverMsgID, seq := u.Server.BroadcastSystemEvent(u.Name+" ✅ 已上线！", u.Name)
		sysMsg := &protocol.Message{
			Type:        protocol.TypeDeliver,
			ServerMsgID: serverMsgID,
			From:        "system",
			ChatID:      "system",
			Seq:         seq,
			Body:        "----------------👏欢迎来到 8H-Go 聊天室----------------",
		}
		u.SendJSON(sysMsg)
	}
}

func (u *User) Offline() {
	if u.Server != nil {
		u.Server.UnregisterOnline(u.Name)
		serverMsgID, seq := u.Server.BroadcastSystemEvent(u.Name+" ❌ 已下线！", u.Name)
		sysMsg := &protocol.Message{
			Type:        protocol.TypeDeliver,
			ServerMsgID: serverMsgID,
			From:        "system",
			ChatID:      "system",
			Seq:         seq,
			Body:        "你已成功下线，欢迎下次再来。",
		}
		u.SendJSON(sysMsg)
	}
}

func (u *User) Logout() {
	u.logoutOnce.Do(func() {
		u.Offline()
		u.Close()
	})
}

func (u *User) SendMsg(msg string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.isClosed {
		return
	}
	select {
	case u.C <- msg:
		if u.Server != nil {
			u.Server.MarkOutbound()
		}
	default:
		go u.Logout()
	}
}

func (u *User) SendJSON(m *protocol.Message) {
	if m == nil {
		return
	}
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	u.SendMsg(string(b))
}
