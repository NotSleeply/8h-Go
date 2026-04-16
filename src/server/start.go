package server

import (
	"fmt"
	"net"
)

// 启动
func (s *Server) Start() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	if err != nil {
		fmt.Println("启动失败:", err)
		return
	}
	fmt.Println("启动成功---", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	defer listener.Close()
	go s.DeliverWorker()
	go s.RetryWorker()
	s.RecoverPendingDeliveries(2000)
	s.acceptLoop(listener)
}
