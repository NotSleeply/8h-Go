package server

import (
	"os"
	"strconv"
	"strings"
	"time"

	"goim/internal/store"
)

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

	s := store.NewInMemoryStore()
	srv := &Server{
		Ip:           ip,
		Port:         port,
		OnlineMap:    make(map[string]OnlineInfo),
		store:        s,
		BlacklistIPs: blacklist,
		rateWindow:   rateWindow,
		rateLimit:    rateLimit,
		attempts:     make(map[string][]time.Time),
		startAt:      time.Now(),
	}
	srv.logic = newLogicService(s)
	srv.groupManager = NewGroupManager()
	return srv
}

func (s *Server) Store() store.Store {
	return s.store
}

func (s *Server) SetWorker(w DeliveryWorker) {
	s.worker = w
}
