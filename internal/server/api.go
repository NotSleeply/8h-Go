package server

import (
	"fmt"
	"time"

	"goim/internal/protocol"
)

func (s *Server) RegisterOnline(name string, sender Sender, addr string) {
	if name == "" || sender == nil {
		return
	}
	s.MapLock.Lock()
	s.OnlineMap[name] = OnlineInfo{Sender: sender, Addr: addr}
	s.MapLock.Unlock()
}

func (s *Server) UnregisterOnline(name string) {
	if name == "" {
		return
	}
	s.MapLock.Lock()
	delete(s.OnlineMap, name)
	s.MapLock.Unlock()
}

func (s *Server) ProcessSend(req *protocol.Message, recipients []string) (*protocol.Message, *protocol.Message, error) {
	if s.logic == nil {
		return nil, nil, fmt.Errorf("logic not initialized")
	}
	return s.logic.ProcessSend(req, recipients)
}

func (s *Server) HandleDeliverAck(username, serverMsgID string) {
	if s.logic != nil {
		s.logic.HandleDeliverAck(username, serverMsgID)
	}
}

func (s *Server) HandleReadAck(username, serverMsgID string) {
	if s.logic != nil {
		s.logic.HandleReadAck(username, serverMsgID)
	}
}

func (s *Server) EnqueueServerMsg(serverMsgID string) {
	if s.worker != nil {
		s.worker.EnqueueServerMsg(serverMsgID)
	}
}

func (s *Server) EnqueuePendingForUser(username string, limit int) {
	if s.worker != nil {
		s.worker.EnqueuePendingForUser(username, limit)
	}
}

func (s *Server) GetC2CHistory(userA, userB string, limit int) []*protocol.Message {
	return s.store.GetC2CHistory(userA, userB, limit)
}

func (s *Server) SnapshotStatsText() string {
	st := s.SnapshotStats()
	return fmt.Sprintf(
		"Server Stats\nstart_at: %s\nuptime: %s\nonline_users: %d\nactive_conn: %d\ntotal_conn: %d\nrejected_conn: %d\ninbound_msgs: %d\noutbound_msgs: %d\nthroughput: %.2f msg/s\ndelivery_pending: %d\ndelivery_delivered: %d\ndelivery_read: %d\ndead_letter: %d",
		st.StartAt.Format(time.RFC3339),
		st.Uptime.Truncate(time.Second),
		st.OnlineUsers,
		st.ActiveConn,
		st.TotalConnections,
		st.RejectedConn,
		st.InboundMessages,
		st.OutboundMessages,
		st.MsgPerSec,
		st.PendingDeliveries,
		st.DeliveredCount,
		st.ReadCount,
		st.DeadLetterCount,
	)
}

func (s *Server) MarkOutbound() { s.markOutboundMessage() }

func (s *Server) ListOnlineUsers() []OnlineUserInfo {
	s.MapLock.RLock()
	defer s.MapLock.RUnlock()
	out := make([]OnlineUserInfo, 0, len(s.OnlineMap))
	for name, info := range s.OnlineMap {
		out = append(out, OnlineUserInfo{Name: name, Addr: info.Addr})
	}
	return out
}

func (s *Server) GroupManager() GroupManagerAPI {
	return s.groupManager
}
