package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Ip   string
	Port int

	// 上线列表
	OnlineMap map[string]*User
	MapLock   sync.RWMutex // 读多写少 锁
	// 消息
	Message chan string

	// Structured delivery queue and in-memory store for ACKs
	DeliverQueue chan string
	store        *InMemoryStore
}

const MaxMessageLength = 1024 // 定义最大消息长度

// 初始化
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:           ip,
		Port:         port,
		OnlineMap:    make(map[string]*User),
		Message:      make(chan string),
		DeliverQueue: make(chan string, 1024),
		store:        NewInMemoryStore(),
	}
	return server
}

// 广播消息 格式
func (s *Server) BoradCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	s.Message <- sendMsg
}

// 消息广播分发
func (s *Server) ListenMessager() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("panic in ListenMessager:", r)
		}
	}()
	for {
		msg := <-s.Message
		s.MapLock.RLock()
		var toKick []*User
		for _, cli := range s.OnlineMap {
			select {
			case cli.C <- msg:
			case <-time.After(time.Second * 1):
				// 1秒内无法发送，判定网络拥塞，记录待踢出的用户
				toKick = append(toKick, cli)
			}
		}
		s.MapLock.RUnlock()

		// 在释放锁后统一处理需要断开的用户，避免在持锁时调用会再次获取锁的方法
		for _, u := range toKick {
			select {
			case u.C <- "系统：检测到网络拥塞，您将被断开。\n":
			default:
			}
			go u.Logout()
		}
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
	user := NewUser(conn, s)
	user.Online()
	isLive := make(chan bool)

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

// 启动
func (s *Server) Start() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	if err != nil {
		fmt.Println("启动失败:", err)
		return
	}
	fmt.Println("启动成功---", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	defer listener.Close()

	go s.ListenMessager()
	go s.DeliverWorker()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept,接受客户端的连接请求出现问题:", err)
			continue
		}
		go s.Handler(conn)
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
