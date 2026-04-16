package session

import (
	"fmt"
	"strings"
	"time"

	"goim/internal/protocol"
)

func (u *User) DoMessage(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	switch {
	case msg == "exit":
		u.useExit(msg)
	case msg == "who":
		u.useWho()
	case msg == "help":
		u.useHelp()
	case msg == "stats":
		u.useStats()
	case strings.HasPrefix(msg, "read|"):
		u.useRead(msg)
	case strings.HasPrefix(msg, "history|"):
		u.useHistory(msg)
	case strings.HasPrefix(msg, "gc|"):
		u.useGroupCreate(msg)
	case strings.HasPrefix(msg, "gj|"):
		u.useGroupJoin(msg)
	case strings.HasPrefix(msg, "gl|"):
		u.useGroupLeave(msg)
	case strings.HasPrefix(msg, "gd|"):
		u.useGroupDelete(msg)
	case strings.HasPrefix(msg, "gk|"):
		u.useGroupKick(msg)
	case strings.HasPrefix(msg, "ga|"):
		u.useGroupGrantAdmin(msg)
	case strings.HasPrefix(msg, "gr|"):
		u.useGroupRevokeAdmin(msg)
	case strings.HasPrefix(msg, "gm|"):
		u.useGroupMembers(msg)
	case strings.HasPrefix(msg, "gt|"):
		u.useGroupTalk(msg)
	case strings.HasPrefix(msg, "rename|"):
		u.useRename(msg)
	case strings.HasPrefix(msg, "to|"):
		u.useChat(msg)
	case strings.Contains(msg, "|"):
		u.useIllegal(msg)
	default:
		if u.Server == nil {
			return
		}
		req := &protocol.Message{
			Type:        protocol.TypeSend,
			ClientMsgID: fmt.Sprintf("c-%d", time.Now().UnixNano()),
			From:        u.Name,
			Body:        msg,
		}
		saved, existing, err := u.Server.ProcessSend(req, nil)
		if err != nil {
			u.SendMsg("消息发送失败: " + err.Error())
			return
		}
		if existing != nil {
			u.SendJSON(&protocol.Message{Type: protocol.TypeSendAck, ClientMsgID: req.ClientMsgID, ServerMsgID: existing.ServerMsgID, Seq: existing.Seq})
			return
		}
		u.SendJSON(&protocol.Message{Type: protocol.TypeSendAck, ClientMsgID: req.ClientMsgID, ServerMsgID: saved.ServerMsgID, Seq: saved.Seq})
		u.Server.EnqueueServerMsg(saved.ServerMsgID)
	}
}
