package server

import (
	"time"
)

func (s *Server) EnqueueServerMsg(serverMsgID string) {
	if serverMsgID == "" {
		return
	}
	if s.bus != nil {
		s.bus.Publish(serverMsgID, s.pushDeliverQueue)
		return
	}
	s.pushDeliverQueue(serverMsgID)
}

func (s *Server) pushDeliverQueue(serverMsgID string) {
	if serverMsgID == "" {
		return
	}
	select {
	case s.DeliverQueue <- serverMsgID:
	default:
		// drop when full
	}
}

func (s *Server) EnqueuePendingForUser(username string, limit int) {
	ids := s.store.ListPendingServerMsgIDsByUser(username, limit)
	for _, id := range ids {
		s.EnqueueServerMsg(id)
	}
}

func (s *Server) RecoverPendingDeliveries(limit int) {
	ids := s.store.RecoverPendingServerMsgIDs(limit)
	for _, id := range ids {
		s.EnqueueServerMsg(id)
	}
}

func (s *Server) RetryWorker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ids := s.store.GetDueRetryServerMsgIDs(500)
		for _, id := range ids {
			s.EnqueueServerMsg(id)
		}
	}
}

func (s *Server) GetC2CHistory(userA, userB string, limit int) []*Message {
	return s.store.GetC2CHistory(userA, userB, limit)
}
