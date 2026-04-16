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
	// OnlineMap maps username -> OnlineInfo
	OnlineMap map[string]OnlineInfo
	MapLock   sync.RWMutex // 读多写少 锁

	// Structured delivery queue and in-memory store for ACKs
	DeliverQueue chan string
	store        iface.Store
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
	// in-flight deliver dedupe: serverMsgID -> struct{}
	deliverInFlight   map[string]struct{}
	deliverInFlightMu sync.Mutex
}

type OnlineInfo struct {
	Sender iface.Sender
	Addr   string
}

const MaxMessageLength = 1024 // 定义最大消息长度
