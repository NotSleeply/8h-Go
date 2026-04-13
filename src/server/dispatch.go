package server

import (
	"fmt"
	"strings"
	"time"
)

// DoMessage 将用户输入路由到具体命令实现（保留最小分发逻辑）
func (u *User) DoMessage(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	} else if msg == "exit" {
		u.useExit(msg)
	} else if msg == "who" {
		u.useWho()
	} else if msg == "help" {
		u.useHelp()
	} else if msg == "stats" {
		u.useStats()
	} else if strings.HasPrefix(msg, "read|") {
		u.useRead(msg)
	} else if strings.HasPrefix(msg, "history|") {
		u.useHistory(msg)
	} else if strings.HasPrefix(msg, "gc|") {
		u.useGroupCreate(msg)
	} else if strings.HasPrefix(msg, "gj|") {
		u.useGroupJoin(msg)
	} else if strings.HasPrefix(msg, "gl|") {
		u.useGroupLeave(msg)
	} else if strings.HasPrefix(msg, "gd|") {
		u.useGroupDelete(msg)
	} else if strings.HasPrefix(msg, "gk|") {
		u.useGroupKick(msg)
	} else if strings.HasPrefix(msg, "ga|") {
		u.useGroupGrantAdmin(msg)
	} else if strings.HasPrefix(msg, "gr|") {
		u.useGroupRevokeAdmin(msg)
	} else if strings.HasPrefix(msg, "gm|") {
		u.useGroupMembers(msg)
	} else if strings.HasPrefix(msg, "gt|") {
		u.useGroupTalk(msg)
	} else if strings.HasPrefix(msg, "rename|") { // rename|msg
		u.useRename(msg)
	} else if strings.HasPrefix(msg, "to|") { // to|toName|msg
		u.useChat(msg)
	} else if strings.Contains(msg, "|") {
		u.useIllegal(msg)
	} else {
		// 将普通文本消息封装为协议 send，走 ACK 流程
		clientID := fmt.Sprintf("c-%d", time.Now().UnixNano())
		req := &Message{
			Type:        TypeSend,
			ClientMsgID: clientID,
			ChatID:      "",
			From:        u.Name,
			Body:        msg,
		}
		u.Server.HandleClientSend(u, req)
	}
}
