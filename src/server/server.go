package server

import (
	"bufio"
	"bytes"
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
}

const MaxMessageLength = 1024 // 定义最大消息长度

// 初始化
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
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
	for {
		msg := <-s.Message
		s.MapLock.Lock()
		for _, cli := range s.OnlineMap {
			select {
			case cli.C <- msg:
			case <-time.After(time.Second * 1):
				// 1秒内无法发送，判定网络拥塞，断开用户
				close(cli.C)
				cli.Conn.Close()
			}
		}
		s.MapLock.Unlock()
	}
}

// 消息处理
func (s *Server) ManagerMessage(user *User, isLive chan bool) {
	defer close(isLive)

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
				msg := strings.TrimSpace(string(bytes.Join(parts, nil)))
				if msg != "" {
					user.DoMessage(msg)
					isLive <- true
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

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept,接受客户端的连接请求出现问题:", err)
			continue
		}
		go s.Handler(conn)
	}
}
