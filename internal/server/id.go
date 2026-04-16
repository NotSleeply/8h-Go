package server

import (
	"fmt"
	"time"
)

func (s *Server) GenerateServerMsgID() string {
	if s.logic == nil {
		return fmt.Sprintf("s-%d", time.Now().UnixNano())
	}
	return s.logic.GenerateServerMsgID("s")
}

func (s *Server) NextSeq(chatID string) uint64 {
	seq, _ := s.store.NextSeq(chatID)
	return seq
}
