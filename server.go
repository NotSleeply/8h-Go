package main

import (
	"fmt"
	"io"
	"net"
	"sync"
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

func (s *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	s.Message <- sendMsg
}

func (s *Server) UserOffline(user *User) {
	s.mapLock.Lock()
	delete(s.OnlineMap, user.Addr)
	s.mapLock.Unlock()
	s.BroadCast(user, "已下线！")
}

/*
创建 4096 字节缓冲区，适配常规消息大小；
conn.Read阻塞读取客户端消息，无消息时等待；
读取长度为 0：代表客户端主动断开连接，触发下线；
读取异常：打印错误并退出协程，防护服务端崩溃；
格式化用户消息，调用广播方法转发给所有在线用户。
*/
func (s *Server) ManagerMessage(user *User) {
	buf := make([]byte, 4096)
	for {
		n, err := user.conn.Read(buf)
		if n == 0 {
			s.UserOffline(user)
			return
		}
		if err != nil && err != io.EOF {
			fmt.Println("conn.Read err:", err)
			return
		}
		msg := fmt.Sprintf("[%s]:%s", user.Addr, string(buf[:n-1]))
		s.BroadCast(user, msg)
	}
}

/*
创建新用户实例：基于客户端连接初始化用户对象；
加锁将新用户加入 OnlineMap，完成「上线注册」；
调用 BroadCast 方法，向所有在线用户推送该用户的上线通知；
select {} 让协程永久阻塞（避免连接处理协程退出），保证用户连接持续有效。
*/
func (s *Server) Handler(conn net.Conn) {
	user := NewUser(conn)

	s.mapLock.Lock()
	s.OnlineMap[user.Name] = user
	s.mapLock.Unlock()
	s.BroadCast(user, "已上线！")

	go s.ManagerMessage(user)

	select {}
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
