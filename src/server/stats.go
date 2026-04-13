package server

import (
	"sync/atomic"
	"time"
)

type StatsSnapshot struct {
	StartAt           time.Time
	Uptime            time.Duration
	MQMode            string
	OnlineUsers       int
	ActiveConn        int64
	TotalConnections  uint64
	RejectedConn      uint64
	InboundMessages   uint64
	OutboundMessages  uint64
	MsgPerSec         float64
	DeliverQueueLen   int
	PendingDeliveries int64
	DeliveredCount    int64
	ReadCount         int64
	DeadLetterCount   int64
}

func (s *Server) SnapshotStats() StatsSnapshot {
	uptime := time.Since(s.startAt)
	if uptime < time.Second {
		uptime = time.Second
	}
	inbound := atomic.LoadUint64(&s.inboundMessages)
	outbound := atomic.LoadUint64(&s.outboundMessages)
	totalMsg := inbound + outbound

	s.MapLock.RLock()
	online := len(s.OnlineMap)
	s.MapLock.RUnlock()
	mqMode := "local"
	if s.bus != nil {
		mqMode = s.bus.Mode()
	}
	deliveryStats := s.store.DeliveryStats()

	return StatsSnapshot{
		StartAt:           s.startAt,
		Uptime:            uptime,
		MQMode:            mqMode,
		OnlineUsers:       online,
		ActiveConn:        atomic.LoadInt64(&s.activeConn),
		TotalConnections:  atomic.LoadUint64(&s.totalConnections),
		RejectedConn:      atomic.LoadUint64(&s.rejectedConn),
		InboundMessages:   inbound,
		OutboundMessages:  outbound,
		MsgPerSec:         float64(totalMsg) / uptime.Seconds(),
		DeliverQueueLen:   len(s.DeliverQueue),
		PendingDeliveries: deliveryStats.Pending,
		DeliveredCount:    deliveryStats.Delivered,
		ReadCount:         deliveryStats.Read,
		DeadLetterCount:   deliveryStats.Dead,
	}
}

func (s *Server) markInboundMessage() {
	atomic.AddUint64(&s.inboundMessages, 1)
}

func (s *Server) markOutboundMessage() {
	atomic.AddUint64(&s.outboundMessages, 1)
}
