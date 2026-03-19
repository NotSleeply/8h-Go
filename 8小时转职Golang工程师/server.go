package main

import (
	"fmt"
	"net"
)

type Server struct {
	Ip   string
	Port int
}

// 初始化
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:   ip,
		Port: port,
	}
	return server
}

// 处理链接
func (s *Server) Handler(conn net.Conn) {
	fmt.Println("有客户端链接成功")
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
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept,接受客户端的连接请求出现问题")
			continue
		}
		go s.Handler(conn)
	}
}
