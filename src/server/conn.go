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

	"tet/src/session"
	userpkg "tet/src/user"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// TCP 流并重组分片消息，支持最大长度限制和基本的 JSON 协议解析。
func (s *Server) ManagerMessage(user *session.User, isLive chan bool) {
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
func (s *Server) processRawLine(user *session.User, raw []byte, isLive chan bool) {
	msgStr := strings.TrimSpace(decodeInputText(raw))
	if msgStr == "" {
		return
	}
	s.markInboundMessage()

	// 如果用户尚未通过注册/登录，优先处理 register/login
	if !user.Authenticated {
		if s.handleAuthCommand(user, msgStr, isLive) {
			return
		}
	}

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

// 处理未认证用户的 register/login 命令，处理后返回 true（已消费）
func (s *Server) handleAuthCommand(user *session.User, msgStr string, isLive chan bool) bool {
	lower := strings.ToLower(msgStr)
	if strings.HasPrefix(lower, "register|") {
		parts := strings.SplitN(msgStr, "|", 3)
		if len(parts) < 3 {
			user.SendMsg("注册格式错误，正确用法：register|用户名|密码\n")
			return true
		}
		username := strings.TrimSpace(parts[1])
		password := parts[2]
		if username == "" || strings.ContainsAny(username, " \t\n\r|") {
			user.SendMsg("用户名不可为空或包含空格/管道符。\n")
			return true
		}
		// 检查是否已在线
		s.MapLock.RLock()
		_, occupied := s.OnlineMap[username]
		s.MapLock.RUnlock()
		if occupied {
			user.SendMsg("该用户名已有人在线，请选择其他用户名。\n")
			return true
		}
		// 检查是否存在于数据库
		if err := userpkg.Register(user, username, password); err != nil {
			user.SendMsg("注册失败：" + err.Error() + "\n")
			return true
		}
		user.SendMsg("注册并登录成功，欢迎：" + username + "\n")
		isLive <- true
		return true
	} else if strings.HasPrefix(lower, "login|") {
		parts := strings.SplitN(msgStr, "|", 3)
		if len(parts) < 3 || strings.TrimSpace(parts[2]) == "" {
			user.SendMsg("登录格式错误，正确用法：login|用户名|密码\n")
			return true
		}
		username := strings.TrimSpace(parts[1])
		password := strings.TrimSpace(parts[2])
		if username == "" {
			user.SendMsg("用户名不能为空\n")
			return true
		}
		// 检查数据库中是否存在用户
		if err := userpkg.Login(user, username, password); err != nil {
			user.SendMsg("登录失败：" + err.Error() + "\n")
			return true
		}
		user.SendMsg("登录成功，欢迎回来：" + username + "\n")
		isLive <- true
		return true
	}
	user.SendMsg("请先注册或登录。注册: register|用户名|密码  登录: login|用户名|密码\n")
	return true
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

	user := session.NewUser(conn, s)
	// 不立即注册为在线用户，要求客户端先 register| 或 login|
	user.SendMsg("欢迎，请先注册：register|用户名|密码 或 登录：login|用户名|密码\n")
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
