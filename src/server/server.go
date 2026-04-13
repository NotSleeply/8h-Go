package server

import (
	"sync"
	"time"

	group "tet/src/group"
	iface "tet/src/iface"
	logicpkg "tet/src/logic"
)

type Server struct {
	Ip   string
	Port int

	// 上线列表
	OnlineMap map[string]*User
	MapLock   sync.RWMutex // 读多写少 锁

	// Structured delivery queue and in-memory store for ACKs
	DeliverQueue chan string
	store        iface.Store
	bus          iface.MessageBusIface
	logic        *logicpkg.LogicService
	groupManager *group.GroupManager

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

	// deliver retry policy
	maxDeliverRetry int
	retryBaseDelay  time.Duration
}

const MaxMessageLength = 1024 // 定义最大消息长度

// NewServer, parse/load/allowConnection and metric counters moved to bootstrap.go / conn_security.go / stats.go

// HandleMessage dispatches incoming protocol messages from clients.
// message handlers moved to messages.go

// enqueue and retry related implementations moved to enqueue.go

// 在系统范围发送一条消息（可排除某个用户名），并返回 serverMsgID 与 seq
// BroadcastSystemEvent moved to broadcast.go

// c2cChatID moved/kept in logic package to avoid duplication

// GenerateServerMsgID exposes a stable server-level id generator interface.
// Start moved to start.go
