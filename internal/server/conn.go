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

	"goim/internal/session"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func (s *Server) Handler(conn net.Conn) {
	atomic.AddUint64(&s.totalConnections, 1)
	atomic.AddInt64(&s.activeConn, 1)
	defer atomic.AddInt64(&s.activeConn, -1)

	user := session.NewUser(conn, s)
	user.SendMsg("欢迎，请先注册：register|用户名|密码 或 登录：login|用户名|密码\n")
	isLive := make(chan bool, 1)

	go s.ManagerMessage(user, isLive)
	for {
		select {
		case _, ok := <-isLive:
			if !ok {
				return
			}
		case <-time.After(300 * time.Second):
			user.SendMsg("你被踢了!\n")
			user.Logout()
			return
		}
	}
}

func (s *Server) ManagerMessage(user *session.User, isLive chan bool) {
	defer func() {
		if r := recover(); r != nil {
			user.Logout()
		}
		close(isLive)
	}()

	reader := bufio.NewReader(user.Conn)
	for {
		user.Conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		raw, tooLong, err := readRawLine(reader)
		if err != nil {
			if err != io.EOF {
				println("ManagerMessage:", err.Error())
			}
			user.Logout()
			return
		}
		if tooLong {
			user.SendMsg(fmt.Sprintf("消息长度超限，最多 %d 字节，本条已丢弃。\n", MaxMessageLength))
			continue
		}
		s.processRawLine(user, raw, isLive)
	}
}

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
			for isPrefix {
				_, isPrefix, err = reader.ReadLine()
				if err != nil {
					return nil, false, err
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

func (s *Server) processRawLine(user *session.User, raw []byte, isLive chan bool) {
	msgStr := strings.TrimSpace(decodeInputText(raw))
	if msgStr == "" {
		return
	}
	s.markInboundMessage()

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

func (s *Server) handleAuthCommand(user *session.User, msgStr string, isLive chan bool) bool {
	lower := strings.ToLower(msgStr)
	if strings.HasPrefix(lower, "register|") {
		parts := strings.SplitN(msgStr, "|", 3)
		if len(parts) < 3 {
			user.SendMsg("注册格式错误，正确用法：register|用户名|密码\n")
			return true
		}
		if err := session.Register(user, strings.TrimSpace(parts[1]), parts[2]); err != nil {
			user.SendMsg("注册失败：" + err.Error() + "\n")
			return true
		}
		user.SendMsg("注册并登录成功，欢迎：" + user.Name + "\n")
		isLive <- true
		return true
	} else if strings.HasPrefix(lower, "login|") {
		parts := strings.SplitN(msgStr, "|", 3)
		if len(parts) < 3 || strings.TrimSpace(parts[2]) == "" {
			user.SendMsg("登录格式错误，正确用法：login|用户名|密码\n")
			return true
		}
		if err := session.Login(user, strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])); err != nil {
			user.SendMsg("登录失败：" + err.Error() + "\n")
			return true
		}
		user.SendMsg("登录成功，欢迎回来：" + user.Name + "\n")
		isLive <- true
		return true
	}
	user.SendMsg("请先注册或登录。注册: register|用户名|密码  登录: login|用户名|密码\n")
	return true
}

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
