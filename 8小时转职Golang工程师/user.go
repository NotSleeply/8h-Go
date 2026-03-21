package main

import "net"

type User struct {
	Name string
	Addr string
	C    chan string
	Conn net.Conn
	Server *Server
}

// 初始化
func NewUser(conn net.Conn,s *Server) *User {
	userAddr := conn.RemoteAddr().String()
	user := &User{
		Name: userAddr,
		Addr: userAddr,
		C:    make(chan string),
		Conn: conn,
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
func (u *User) Online(){
	u.Server.MapLock.Lock()
	u.Server.OnlineMap[u.Name] = u
	u.Server.MapLock.Unlock()

	u.Server.BoradCast(u,"已上线！")
}

// 下线
func (u *User) Offline(){
	u.Server.MapLock.Lock()
	delete(u.Server.OnlineMap,u.Name)
	u.Server.MapLock.Unlock()

	u.Server.BoradCast(u,"已下线！")
}

// user层处理信息
func (u *User)DoMessage(msg string){
	u.Server.BoradCast(u,msg);
}