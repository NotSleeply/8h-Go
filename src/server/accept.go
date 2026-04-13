package server

import (
	"fmt"
	"net"
	"sync/atomic"
)

func (s *Server) acceptLoop(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept,接受客户端的连接请求出现问题:", err)
			continue
		}
		ip := s.parseIP(conn.RemoteAddr())
		ok, reason := s.allowConnection(ip)
		if !ok {
			atomic.AddUint64(&s.rejectedConn, 1)
			_, _ = conn.Write([]byte("连接被拒绝: " + reason + "\n"))
			_ = conn.Close()
			continue
		}
		go s.Handler(conn)
	}
}
