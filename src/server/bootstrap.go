package server

import (
	"os"
	"strconv"
	"strings"
	"time"

	group "tet/src/group"
	logicpkg "tet/src/logic"
	storepkg "tet/src/store"
)

// 初始化并返回 Server 实例
func NewServer(ip string, port int) *Server {
	blacklist := loadBlacklistFromEnv()
	rateWindow := 1 * time.Minute
	rateLimit := 30
	if v := strings.TrimSpace(os.Getenv("IM_CONN_RATE_WINDOW_SEC")); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			rateWindow = time.Duration(sec) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("IM_CONN_RATE_LIMIT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rateLimit = n
		}
	}
	maxDeliverRetry := 5
	if v := strings.TrimSpace(os.Getenv("IM_DELIVER_MAX_RETRY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxDeliverRetry = n
		}
	}
	retryBaseDelay := 2 * time.Second
	if v := strings.TrimSpace(os.Getenv("IM_DELIVER_RETRY_BASE_SEC")); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			retryBaseDelay = time.Duration(sec) * time.Second
		}
	}

	store := storepkg.NewInMemoryStore()
	server := &Server{
		Ip:              ip,
		Port:            port,
		OnlineMap:       make(map[string]OnlineInfo),
		DeliverQueue:    make(chan string, 1024),
		store:           store,
BlacklistIPs:    blacklist,
		rateWindow:      rateWindow,
		rateLimit:       rateLimit,
		attempts:        make(map[string][]time.Time),
		startAt:         time.Now(),
		maxDeliverRetry: maxDeliverRetry,
		retryBaseDelay:  retryBaseDelay,
		deliverInFlight: make(map[string]struct{}),
	}
	server.logic = logicpkg.NewLogicService(store)
	server.groupManager = group.NewGroupManager()
	return server
}
