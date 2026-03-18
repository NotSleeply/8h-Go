package main

import (
	"net"
	"strings"
)

type User struct {
	Name   string
	Addr   string
	C      chan string
	conn   net.Conn
	server *Server
}

func NewUser(conn net.Conn, server *Server) *User {
	userAddr := conn.RemoteAddr().String()
	user := &User{
		Name:   userAddr,
		Addr:   userAddr,
		C:      make(chan string),
		conn:   conn,
		server: server,
	}
	go user.ListenMessage()
	return user
}

// 用户的上线业务
func (user *User) UserOnline() {
	user.server.mapLock.Lock()
	user.server.OnlineMap[user.Name] = user
	user.server.mapLock.Unlock()
	user.server.BroadCast(user, "已上线！")
}

// 用户的下线业务
func (user *User) UserOffline() {
	user.server.mapLock.Lock()
	delete(user.server.OnlineMap, user.Name)
	user.server.mapLock.Unlock()
	user.server.BroadCast(user, "已下线！")
}

// 用户处理消息的业务
func (user *User) DoMessage(msg string) {
	if msg == "who" {
		user.server.mapLock.Lock()
		for _, onlineUser := range user.server.OnlineMap {
			// send via channel; ListenMessage will append newline
			onlineMsg := "[" + onlineUser.Addr + "]" + onlineUser.Name + ":在线..."
			user.SendMes(onlineMsg)
		}
		user.server.mapLock.Unlock()

	} else if len(msg) > 7 && msg[:7] == "rename|" {
		newName := msg[7:]
		_, ok := user.server.OnlineMap[newName]
		if ok {
			user.SendMes("当前用户名被占用！")
		} else {
			user.server.mapLock.Lock()
			delete(user.server.OnlineMap, user.Name)
			user.server.OnlineMap[newName] = user
			user.server.mapLock.Unlock()
			user.Name = newName
			user.SendMes("您已更新用户名：" + user.Name)
		}
	} else if len(msg) > 4 && msg[:3] == "to|" {
		// 消息格式：to|用户名|消息内容
		// 1. 获取用户名
		remoteName := strings.Split(msg[3:], "|")[0]
		if remoteName == "" {
			user.SendMes("消息格式不正确，请使用 \"to|张三|你好啊\" 格式。\n")
			return
		}
		// 2. 寻找OnlineMap对应用户
		remoteUser, ok := user.server.OnlineMap[remoteName]
		if !ok {
			println("用户不存在！")
			return
		}
		// 3. 获取消息
		content := strings.Split(msg, "|")[2]
		if content == "" {
			user.SendMes("无消息内容，请重发\n")
			return
		}
		// 4. 发送消息
		remoteUser.SendMes("您对[" + remoteUser.Name + "]说：" + content)

	} else {
		user.server.BroadCast(user, msg)
	}
}

// 给当前用户对应的客户端发送消息
func (user *User) SendMes(msg string) {
	user.C <- msg
}

// 用户的消息广播业务
func (user *User) ListenMessage() {
	for {
		msg := <-user.C
		_, err := user.conn.Write([]byte(msg + "\n"))
		if err != nil {
			// if write fails (client disconnected), stop this goroutine
			return
		}
	}
}
