package server

import (
	"time"
)

func (s *Server) EnqueueServerMsg(serverMsgID string) {
	if serverMsgID == "" {
		return
	}
	s.pushDeliverQueue(serverMsgID)
}

func (s *Server) pushDeliverQueue(serverMsgID string) {
	if serverMsgID == "" {
		return
	}
	// dedupe: avoid enqueueing same serverMsgID while it's already in-flight
	s.deliverInFlightMu.Lock()
	if _, ok := s.deliverInFlight[serverMsgID]; ok {
		s.deliverInFlightMu.Unlock()
		return
	}
	s.deliverInFlight[serverMsgID] = struct{}{}
	s.deliverInFlightMu.Unlock()

	select {
	case s.DeliverQueue <- serverMsgID:
		// successfully enqueued
	default:
		// drop when full and clear in-flight marker so future attempts can enqueue
		s.deliverInFlightMu.Lock()
		delete(s.deliverInFlight, serverMsgID)
		s.deliverInFlightMu.Unlock()
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
