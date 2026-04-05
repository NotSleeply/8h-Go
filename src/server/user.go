package server

import (
	"net"
	"sync"
)

type User struct {
	Name      string
	Addr      string
	C         chan string
	Conn      net.Conn
	Server    *Server
	closeOnce sync.Once
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

// 监听用户信息
func (u *User) ListenMessage() {
	for msg := range u.C {
		u.Conn.Write([]byte(msg + "\n"))
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
	u.Offline()
	u.Close()
}

// 私发消息
func (u *User) SendMsg(msg string) {
	u.Conn.Write([]byte(msg))
}

// user层处理信息
func (u *User) DoMessage(msg string) {
	if msg == "exit" {
		u.useExit(msg)
	} else if msg == "who" {
		u.useWho()
	} else if len(msg) > 7 && msg[:7] == "rename|" { // rename|msg
		u.useRename(msg)
	} else if len(msg) > 4 && msg[:3] == "to|" { // to|toName|msg
		u.useChat(msg)
	} else {
		u.Server.BoradCast(u, msg)
	}
}
