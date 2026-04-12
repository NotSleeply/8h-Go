package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	Ip   string
	Port int

	// 上线列表
	OnlineMap map[string]*User
	MapLock   sync.RWMutex // 读多写少 锁

	// Structured delivery queue and in-memory store for ACKs
	DeliverQueue chan string
	store        *InMemoryStore

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

const MaxMessageLength = 1024 // 定义最大消息长度

// 初始化
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

	server := &Server{
		Ip:           ip,
		Port:         port,
		OnlineMap:    make(map[string]*User),
		DeliverQueue: make(chan string, 1024),
		store:        NewInMemoryStore(),
		BlacklistIPs: blacklist,
		rateWindow:   rateWindow,
		rateLimit:    rateLimit,
		attempts:     make(map[string][]time.Time),
		startAt:      time.Now(),
	}
	return server
}

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

func (s *Server) markInboundMessage() {
	atomic.AddUint64(&s.inboundMessages, 1)
}

func (s *Server) markOutboundMessage() {
	atomic.AddUint64(&s.outboundMessages, 1)
}

type StatsSnapshot struct {
	StartAt          time.Time
	Uptime           time.Duration
	OnlineUsers      int
	ActiveConn       int64
	TotalConnections uint64
	RejectedConn     uint64
	InboundMessages  uint64
	OutboundMessages uint64
	MsgPerSec        float64
	DeliverQueueLen  int
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

	return StatsSnapshot{
		StartAt:          s.startAt,
		Uptime:           uptime,
		OnlineUsers:      online,
		ActiveConn:       atomic.LoadInt64(&s.activeConn),
		TotalConnections: atomic.LoadUint64(&s.totalConnections),
		RejectedConn:     atomic.LoadUint64(&s.rejectedConn),
		InboundMessages:  inbound,
		OutboundMessages: outbound,
		MsgPerSec:        float64(totalMsg) / uptime.Seconds(),
		DeliverQueueLen:  len(s.DeliverQueue),
	}
}

// 消息处理
func (s *Server) ManagerMessage(user *User, isLive chan bool) {
	defer func() {
		if r := recover(); r != nil {
			println("panic in ManagerMessage:", r)
			user.Logout()
		}
		close(isLive)
	}()

	reader := bufio.NewReader(user.Conn)
	for {
		// 设置读超时，避免被卡住（根据需求调整）
		user.Conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		var parts [][]byte
		total := 0
		for {
			chunk, isPrefix, err := reader.ReadLine()
			if err != nil {
				if err == io.EOF {
					user.Logout()
					return
				}
				println("ManagerMessage:", err.Error())
				user.Logout()
				return
			}

			total += len(chunk)
			if total > MaxMessageLength {
				// 如果当前行还未读完，继续读并丢弃直到行结束
				if isPrefix {
					for isPrefix {
						_, isPrefix, err = reader.ReadLine()
						if err != nil {
							if err == io.EOF {
								user.Logout()
								return
							}
							println("ManagerMessage:", err.Error())
							user.Logout()
							return
						}
					}
				}
				user.SendMsg(fmt.Sprintf("消息长度超限，最多 %d 字节，本条已丢弃。\n", MaxMessageLength))
				// 丢弃本条，进入下一条读取
				break
			}

			parts = append(parts, chunk)
			if !isPrefix {
				msgStr := strings.TrimSpace(string(bytes.Join(parts, nil)))
				if msgStr != "" {
					s.markInboundMessage()
					// 支持最小 JSON 协议：{...}
					if strings.HasPrefix(msgStr, "{") {
						var pm Message
						if err := json.Unmarshal([]byte(msgStr), &pm); err == nil {
							s.HandleMessage(user, &pm)
							isLive <- true
						} else {
							user.SendMsg("非法 JSON 协议: " + err.Error())
						}
					} else {
						user.DoMessage(msgStr)
						isLive <- true
					}
				}
				break
			}
			// 若 isPrefix 为 true，继续循环读取该行剩余部分
		}
	}
}

// 处理链接
func (s *Server) Handler(conn net.Conn) {
	atomic.AddUint64(&s.totalConnections, 1)
	atomic.AddInt64(&s.activeConn, 1)
	defer atomic.AddInt64(&s.activeConn, -1)

	user := NewUser(conn, s)
	user.Online()
	isLive := make(chan bool, 1) // 修复协程泄漏问题

	go s.ManagerMessage(user, isLive)
	for {
		select {
		case _, ok := <-isLive:
			if !ok {
				return
			}
		case <-time.After(time.Second * 300):
			user.SendMsg("你被踢了!\n")
			user.Logout()
			return
		}
	}
}

// HandleMessage dispatches incoming protocol messages from clients.
func (s *Server) HandleMessage(user *User, m *Message) {
	switch m.Type {
	case TypeSend:
		if m.From == "" {
			m.From = user.Name
		}
		s.HandleClientSend(user, m)
	case TypeDeliverAck:
		s.HandleDeliverAck(user, m)
	default:
		// ignore other types for now
	}
}

// HandleClientSend handles a client's send request: dedupe, persist in-memory and enqueue for delivery.
func (s *Server) HandleClientSend(u *User, req *Message) {
	// idempotency check by (from, client_msg_id)
	if existing := s.store.GetMessageByClientID(req.From, req.ClientMsgID); existing != nil {
		ack := &Message{Type: TypeSendAck, ClientMsgID: req.ClientMsgID, ServerMsgID: existing.ServerMsgID, Seq: existing.Seq}
		u.SendJSON(ack)
		return
	}

	seq, _ := s.store.NextSeq(req.ChatID)
	serverMsgID := fmt.Sprintf("s-%d", time.Now().UnixNano())
	msg := &Message{
		Type:        TypeDeliver,
		ServerMsgID: serverMsgID,
		ClientMsgID: req.ClientMsgID,
		ChatID:      req.ChatID,
		From:        req.From,
		To:          req.To,
		Seq:         seq,
		Body:        req.Body,
		Ts:          time.Now().Unix(),
	}
	s.store.SaveMessage(msg)

	// recipients: private or broadcast to all online except sender
	if req.To != "" {
		s.store.SaveDelivery(serverMsgID, req.To)
	} else {
		s.MapLock.RLock()
		for _, user := range s.OnlineMap {
			if user.Name == u.Name {
				continue
			}
			s.store.SaveDelivery(serverMsgID, user.Name)
		}
		s.MapLock.RUnlock()
	}

	// reply send_ack
	ack := &Message{Type: TypeSendAck, ClientMsgID: req.ClientMsgID, ServerMsgID: serverMsgID, Seq: seq}
	u.SendJSON(ack)

	// enqueue for delivery
	select {
	case s.DeliverQueue <- serverMsgID:
	default:
		// queue is full, drop (in this simple impl)
	}
}

// HandleDeliverAck marks a delivery as acknowledged by recipient.
func (s *Server) HandleDeliverAck(u *User, m *Message) {
	if m.ServerMsgID == "" {
		return
	}
	s.store.MarkDeliveryAcked(m.ServerMsgID, u.Name)
}

// 在系统范围发送一条消息（可排除某个用户名），并返回 serverMsgID 与 seq
func (s *Server) BroadcastSystemEvent(body string, exclude string) (serverMsgID string, seq uint64) {
	// 分配 seq（对 system 用一个独立 chat id，比如 "system"）
	seq, _ = s.store.NextSeq("system")
	serverMsgID = fmt.Sprintf("sys-%d", time.Now().UnixNano())

	msg := &Message{
		Type:        TypeDeliver,
		ServerMsgID: serverMsgID,
		ChatID:      "system",
		From:        "system",
		Body:        body,
		Seq:         seq,
		Ts:          time.Now().Unix(),
	}
	s.store.SaveMessage(msg)

	// 把 pending delivery 记录写给当前在线用户（可排除触发者）
	s.MapLock.RLock()
	for name := range s.OnlineMap {
		if name == exclude {
			continue
		}
		s.store.SaveDelivery(serverMsgID, name)
	}
	s.MapLock.RUnlock()

	// 推入投递队列，让 DeliverWorker 去发送
	select {
	case s.DeliverQueue <- serverMsgID:
	default:
		// 若队列满，可记录日志或退避；这里为简化直接丢弃
	}
	return
}

// 启动
func (s *Server) Start() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	if err != nil {
		fmt.Println("启动失败:", err)
		return
	}
	fmt.Println("启动成功---", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	defer listener.Close()

	go s.DeliverWorker()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept,接受客户端的连接请求出现问题:", err)
			continue
		}
		ip := s.parseIP(conn.RemoteAddr())
		ok, reason := s.allowConnection(ip)
		if !ok {
			atomic.AddUint64(&s.rejectedConn, 1)
			_, _ = conn.Write([]byte("连接被拒绝: " + reason + "\n"))
			_ = conn.Close()
			continue
		}
		go s.Handler(conn)
	}
}
