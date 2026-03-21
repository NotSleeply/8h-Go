package main

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
)

type Server struct {
	Ip   string
	Port int

	// 上线列表
	OnlineMap map[string]*User
	MapLock   sync.RWMutex // 读多写少 锁
	// 消息
	Message chan string
}

// 初始化
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}
	return server
}

// 广播消息 格式
func (s *Server) BoradCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	s.Message <- sendMsg
}

// 消息广播分发
func (s *Server) ListenMessager() {
	for {
		msg := <-s.Message
		s.MapLock.Lock()
		for _, cli := range s.OnlineMap {
			cli.C <- msg
		}
		s.MapLock.Unlock()
	}
}

// 消息处理
func (s *Server) ManagerMessage(user *User) {
	buf := make([]byte, 4096)
	for {
		n, err := user.Conn.Read(buf)
		if n == 0 {
			user.Offline()
			return
		}
		if err != nil && err != io.EOF {
			println("ManagerMessage:", err)
			return
		}
		rawMsg := string(buf[:n])
		rawMsg = strings.TrimSpace(rawMsg)
		user.DoMessage(rawMsg)
	}
}

// 处理链接
func (s *Server) Handler(conn net.Conn) {
	user := NewUser(conn, s)
	user.Online()

	go s.ManagerMessage(user)

	select {}
}

// 启动
func (s *Server) Start() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	if err != nil {
		fmt.Println("启动失败")
		return
	}
	fmt.Println("启动成功---", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	defer listener.Close()

	go s.ListenMessager()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept,接受客户端的连接请求出现问题")
			continue
		}
		go s.Handler(conn)
	}
}
