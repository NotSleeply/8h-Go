package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

// ManagerMessage 负责从客户端读取行并解析为协议或普通文本，交由 Server 处理。
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

// Handler 处理已接受的 net.Conn，将其包装为 User 并启动消息管理协程。
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
