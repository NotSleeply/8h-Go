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
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// TCP 流并重组分片消息，支持最大长度限制和基本的 JSON 协议解析。
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

		raw, tooLong, err := readRawLine(reader)
		if err != nil {
			if err == io.EOF {
				user.Logout()
				return
			}
			println("ManagerMessage:", err.Error())
			user.Logout()
			return
		}
		// 如果消息过长，通知用户并丢弃该消息
		if tooLong {
			user.SendMsg(fmt.Sprintf("消息长度超限，最多 %d 字节，本条已丢弃。\n", MaxMessageLength))
			continue
		}

		s.processRawLine(user, raw, isLive)
	}
}

// 从 reader 读取一行，重组合并分片。若超过 MaxMessageLength，会丢弃剩余片段并返回 tooLong=true。
func readRawLine(reader *bufio.Reader) ([]byte, bool, error) {
	var parts [][]byte
	total := 0
	for {
		chunk, isPrefix, err := reader.ReadLine()
		if err != nil {
			return nil, false, err
		}
		total += len(chunk)
		if total > MaxMessageLength {
			if isPrefix {
				for isPrefix {
					_, isPrefix, err = reader.ReadLine()
					if err != nil {
						return nil, false, err
					}
				}
			}
			return nil, true, nil
		}
		parts = append(parts, chunk)
		if !isPrefix {
			return bytes.Join(parts, nil), false, nil
		}
	}
}

// processRawLine 负责对已重组的原始行执行解码、解析与分发。
func (s *Server) processRawLine(user *User, raw []byte, isLive chan bool) {
	msgStr := strings.TrimSpace(decodeInputText(raw))
	if msgStr == "" {
		return
	}
	s.markInboundMessage()
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

// decodeInputText 确保将传入的行字节转换为 UTF-8 文本。首选UTF-8；如果无效，则回退到GB18030 (nc).
func decodeInputText(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if utf8.Valid(b) {
		return string(b)
	}
	if out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b); err == nil && utf8.Valid(out) {
		return string(out)
	}
	return string(b)
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
