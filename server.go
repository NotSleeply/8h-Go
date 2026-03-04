package main

import (
	"fmt"
	"net"
)

type Server struct {
	Ip   string
	Port int
}

func NewServer(ip string, port int) *Server {
	return &Server{
		Ip:   ip,
		Port: port,
	}
}

func (s *Server) Handler(conn net.Conn) {
	// 处理用户连接
	fmt.Printf("新连接来自: %s\n", conn.RemoteAddr().String())
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
