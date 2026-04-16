package server

import (
	"sync"
	"time"

	"goim/internal/store"
)

type Server struct {
	Ip   string
	Port int

	OnlineMap map[string]OnlineInfo
	MapLock   sync.RWMutex

	store        store.Store
	logic        *LogicService
	groupManager *GroupManager

	// delivery worker (set after construction)
	worker DeliveryWorker

	// connection security
	BlacklistIPs map[string]struct{}
	rateWindow   time.Duration
	rateLimit    int
	attempts     map[string][]time.Time
	attemptsMu   sync.Mutex

	// runtime metrics
	startAt          time.Time
	totalConnections uint64
	rejectedConn     uint64
	inboundMessages  uint64
	outboundMessages uint64
	activeConn       int64
}

// DeliveryWorker is the interface the server uses to enqueue messages.
type DeliveryWorker interface {
	EnqueueServerMsg(serverMsgID string)
	EnqueuePendingForUser(username string, limit int)
	RecoverPendingDeliveries(limit int)
	Start()
}

type OnlineInfo struct {
	Sender Sender
	Addr   string
}

const MaxMessageLength = 1024
