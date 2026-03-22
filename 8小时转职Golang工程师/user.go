package main

import (
	"net"
	"strings"
)

type User struct {
	Name   string
	Addr   string
	C      chan string
	Conn   net.Conn
	Server *Server
}

// 初始化
func NewUser(conn net.Conn, s *Server) *User {
	userAddr := conn.RemoteAddr().String()
	user := &User{
		Name:   userAddr,
		Addr:   userAddr,
		C:      make(chan string),
		Conn:   conn,
		Server: s,
	}
	go user.ListenMessage()
	return user
}

// 监听用户信息
func (u *User) ListenMessage() {
	for {
		msg := <-u.C
		u.Conn.Write([]byte(msg + "\n"))
	}
}

// 上线
func (u *User) Online() {
	u.Server.MapLock.Lock()
	u.Server.OnlineMap[u.Name] = u
	u.Server.MapLock.Unlock()

	u.Server.BoradCast(u, "已上线！")
}

// 下线
func (u *User) Offline() {
	u.Server.MapLock.Lock()
	delete(u.Server.OnlineMap, u.Name)
	u.Server.MapLock.Unlock()

	u.Server.BoradCast(u, "已下线！")
}

// 私发消息
func (u *User) SendMsg(msg string) {
	u.Conn.Write([]byte(msg))
}

// 查询在线用户
func (u *User) useWho() {
	u.Server.MapLock.Lock()
	for _, user := range u.Server.OnlineMap {
		msg := "[" + user.Addr + "]" + user.Name + ":" + "在线中…" + "\n"
		u.SendMsg(msg)
	}
	u.Server.MapLock.Unlock()
}

// 更改姓名
func (u *User) useRename(msg string) {
	newName := strings.Split(msg, "|")[1]
	_, ok := u.Server.OnlineMap[newName]
	if ok {
		u.SendMsg("此名称已被占用!")
	} else {
		oldName := u.Name
		u.Server.MapLock.Lock()
		delete(u.Server.OnlineMap, u.Name)
		u.Server.OnlineMap[newName] = u
		u.Server.MapLock.Unlock()

		u.Name = newName
		newMsg := "已将" + oldName + "更改为:" + newName + "\n"
		u.SendMsg(newMsg)
	}
}

// user层处理信息
func (u *User) DoMessage(msg string) {
	if msg == "who" {
		u.useWho()
	} else if len(msg) > 7 && msg[:7] == "rename|" {
		u.useRename(msg)
	} else {
		u.Server.BoradCast(u, msg)
	}
}
