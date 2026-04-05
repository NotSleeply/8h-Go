package server

import (
	"net"
	"strings"
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
		close(u.C)
		u.Conn.Close()
	})
}

// 上线
func (u *User) Online() {
	u.Server.MapLock.Lock()
	u.Server.OnlineMap[u.Name] = u
	u.Server.MapLock.Unlock()
	u.SendMsg("----------------👏欢迎来到 8H-Go 聊天室----------------\n")

	u.Server.BoradCast(u, "✅已上线！")
}

// 下线
func (u *User) Offline() {
	u.Server.MapLock.Lock()
	delete(u.Server.OnlineMap, u.Name)
	u.Server.MapLock.Unlock()

	u.Server.BoradCast(u, "❌已下线！")
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
	defer func() {
		if r := recover(); r != nil {
			// 忽略向已关闭 channel 发送导致的 panic
		}
	}()
	select {
	case u.C <- msg:
	case <-time.After(2 * time.Second):
		go u.Logout()
	}
}

// user层处理信息
func (u *User) DoMessage(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	} else if msg == "exit" {
		u.useExit(msg)
	} else if msg == "who" {
		u.useWho()
	} else if strings.HasPrefix(msg, "rename|") { // rename|msg
		u.useRename(msg)
	} else if strings.HasPrefix(msg, "to|") { // to|toName|msg
		u.useChat(msg)
	} else if strings.Contains(msg, "|") {
		u.useIllegal(msg)
	} else {
		u.Server.BoradCast(u, msg)
	}
}
