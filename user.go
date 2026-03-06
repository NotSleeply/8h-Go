package main

import "net"

type User struct {
	Name string
	Addr string
	C    chan string
	conn net.Conn
}

func NewUser(conn net.Conn) *User {
	userAddr := conn.RemoteAddr().String()
	user := &User{
		Name: userAddr,
		Addr: userAddr,
		C:    make(chan string),
		conn: conn,
	}
	go ListenMessage(user)
	return user
}
func ListenMessage(user *User) {
	for {
		msg := <-user.C
		user.conn.Write([]byte(msg + "\n"))
	}
}
