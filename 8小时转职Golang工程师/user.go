package main

import "net"

type User struct {
	Name string
	Addr string
	C    chan string
	Conn net.Conn
}

// 初始化
func NewUser(conn net.Conn) *User {
	userAddr := conn.RemoteAddr().String()
	user := &User{
		Name: userAddr,
		Addr: userAddr,
		C:    make(chan string),
		Conn: conn,
	}
	go user.ListenMessage()
	return user
}

// 监听用户信息
func (u User) ListenMessage() {
	for {
		msg := <-u.C
		u.Conn.Write([]byte(msg + "\n"))
	}
}
