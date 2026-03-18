package main

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Server struct {
	Ip        string
	Port      int
	OnlineMap map[string]*User
	mapLock   sync.RWMutex
	Message   chan string
}

func NewServer(ip string, port int) *Server {
	return &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}
}

// 监听Message广播消息channel的goroutine，一旦有消息就发送给全部在线User
func (s *Server) ListenMessager() {
	for {
		msg := <-s.Message
		s.mapLock.Lock()
		for _, cli := range s.OnlineMap {
			cli.C <- msg
		}
		s.mapLock.Unlock()
	}
}

// 广播消息
func (s *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	s.Message <- sendMsg
}

// 处理用户消息
func (s *Server) ManagerMessage(user *User, isLive chan bool) {
	buf := make([]byte, 4096)
	for {
		n, err := user.conn.Read(buf)
		if n == 0 {
			user.UserOffline()
			return
		}
		if err != nil && err != io.EOF {
			fmt.Println("conn.Read err:", err)
			return
		}
		raw := string(buf[:n])
		for len(raw) > 0 && (raw[len(raw)-1] == '\n' || raw[len(raw)-1] == '\r') {
			raw = raw[:len(raw)-1]
		}
		user.DoMessage(raw)

		isLive <- true
	}
}

// 处理用户上线和下线
func (s *Server) Handler(conn net.Conn) {
	user := NewUser(conn, s)
	user.UserOnline()

	isLive := make(chan bool)

	go s.ManagerMessage(user, isLive)

	for {
		select {
		case <-isLive:
		case <-time.After(time.Second * 300):
			user.SendMes("你被踢了")

			close(user.C)
			conn.Close()

			return
		}
	}
}

func (s *Server) Start() {
	// 监听端口
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	if err != nil {
		fmt.Println("监听端口失败:", err)
		return
	}
	fmt.Printf("服务器在%s:%d启动成功\n", s.Ip, s.Port)

	// 关闭端口
	defer listener.Close()

	go s.ListenMessager()

	for {
		// accept连接
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("接受连接失败:", err)
			continue
		}

		// 处理连接
		go s.Handler(conn)
	}

}
