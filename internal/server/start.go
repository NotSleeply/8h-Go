package server

import (
	"fmt"
	"net"
)

func (s *Server) Start() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	if err != nil {
		fmt.Println("启动失败:", err)
		return
	}
	fmt.Println("启动成功---", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	defer listener.Close()

	s.worker.Start()
	s.worker.RecoverPendingDeliveries(2000)
	s.acceptLoop(listener)
}
