package server

import (
	"net"
	"os"
	"strings"
	"time"
)

func loadBlacklistFromEnv() map[string]struct{} {
	out := make(map[string]struct{})
	raw := strings.TrimSpace(os.Getenv("IM_BLACKLIST_IPS"))
	if raw == "" {
		return out
	}
	for _, item := range strings.Split(raw, ",") {
		ip := strings.TrimSpace(item)
		if ip == "" {
			continue
		}
		out[ip] = struct{}{}
	}
	return out
}

func (s *Server) parseIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func (s *Server) allowConnection(ip string) (bool, string) {
	if ip == "" {
		return true, ""
	}
	if _, blocked := s.BlacklistIPs[ip]; blocked {
		return false, "ip is blacklisted"
	}
	now := time.Now()
	cutoff := now.Add(-s.rateWindow)

	s.attemptsMu.Lock()
	defer s.attemptsMu.Unlock()

	records := s.attempts[ip]
	filtered := records[:0]
	for _, t := range records {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) >= s.rateLimit {
		s.attempts[ip] = filtered
		return false, "rate limit exceeded"
	}
	filtered = append(filtered, now)
	s.attempts[ip] = filtered
	return true, ""
}
