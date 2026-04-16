package session

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"goim/internal/cache"
	"goim/internal/protocol"
	"goim/internal/svcapi"
)

type User struct {
	Name            string
	Addr            string
	C               chan string
	Conn            net.Conn
	Server          svcapi.ServerAPI
	Authenticated   bool
	closeOnce       sync.Once
	logoutOnce      sync.Once
	mu              sync.Mutex
	isClosed        bool
	heartbeatCancel context.CancelFunc
}

func NewUser(conn net.Conn, srv svcapi.ServerAPI) *User {
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
	_, err := u.Conn.Write([]byte(u.Name + "> "))
	return err
}

func (u *User) ListenMessage() {
	defer func() { recover() }()
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
	if u.Server != nil {
		u.Server.RegisterOnline(u.Name, u, u.Addr)
		u.Server.EnqueuePendingForUser(u.Name, 500)
		serverMsgID, seq := u.Server.BroadcastSystemEvent(u.Name+" ✅ 已上线！", u.Name)
		u.SendJSON(&protocol.Message{
			Type:        protocol.TypeDeliver,
			ServerMsgID: serverMsgID,
			From:        "system",
			ChatID:      "system",
			Seq:         seq,
			Body:        "----------------👏欢迎来到 8H-Go 聊天室----------------",
		})
	}
	u.startOnlineHeartbeat()
}

func (u *User) Offline() {
	u.stopOnlineHeartbeat()
	if u.Server != nil {
		u.Server.UnregisterOnline(u.Name)
		serverMsgID, seq := u.Server.BroadcastSystemEvent(u.Name+" ❌ 已下线！", u.Name)
		u.SendJSON(&protocol.Message{
			Type:        protocol.TypeDeliver,
			ServerMsgID: serverMsgID,
			From:        "system",
			ChatID:      "system",
			Seq:         seq,
			Body:        "你已成功下线，欢迎下次再来。",
		})
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

func (u *User) GetName() string { return u.Name }

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

func (u *User) startOnlineHeartbeat() {
	u.mu.Lock()
	if u.heartbeatCancel != nil {
		u.heartbeatCancel()
		u.heartbeatCancel = nil
	}
	c := cache.Client()
	if c == nil || u.Name == "" {
		u.mu.Unlock()
		return
	}
	ttl := cache.OnlineTTL()
	key := cache.OnlineKey(u.Name)
	val := cache.GatewayID()
	_ = c.Set(context.Background(), key, val, ttl).Err()
	ctx, cancel := context.WithCancel(context.Background())
	u.heartbeatCancel = cancel
	u.mu.Unlock()

	go func() {
		ticker := time.NewTicker(ttl / 2)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = c.Set(context.Background(), key, val, ttl).Err()
			}
		}
	}()
}

func (u *User) stopOnlineHeartbeat() {
	u.mu.Lock()
	if u.heartbeatCancel != nil {
		u.heartbeatCancel()
		u.heartbeatCancel = nil
	}
	u.mu.Unlock()
	if c := cache.Client(); c != nil && u.Name != "" {
		_ = c.Del(context.Background(), cache.OnlineKey(u.Name)).Err()
	}
}
