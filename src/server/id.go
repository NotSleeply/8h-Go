package server

import (
	"fmt"
	"time"

	logicpkg "tet/src/logic"
)

// GenerateServerMsgID exposes a stable server-level id generator interface.
func (s *Server) GenerateServerMsgID() string {
	if s.logic == nil {
		return fmt.Sprintf("s-%d", time.Now().UnixNano())
	}
	return s.logic.GenerateServerMsgID("s")
}

// NextSeq exposes per-chat sequence generation for sync/replay scenarios.
func (s *Server) NextSeq(chatID string) uint64 {
	seq, _ := s.store.NextSeq(chatID)
	return seq
}

func (s *Server) Logic() *logicpkg.LogicService {
	return s.logic
}
