package server

import (
	"time"
)

// BroadcastSystemEvent 在系统范围发送一条消息（可排除某个用户名），并返回 serverMsgID 与 seq
func (s *Server) BroadcastSystemEvent(body string, exclude string) (serverMsgID string, seq uint64) {
	seq, _ = s.store.NextSeq("system")
	serverMsgID = s.logic.GenerateServerMsgID("sys")

	msg := &Message{
		Type:        TypeDeliver,
		ServerMsgID: serverMsgID,
		ChatID:      "system",
		From:        "system",
		Body:        body,
		Seq:         seq,
		Ts:          time.Now().Unix(),
	}

	var recipients []string
	s.MapLock.RLock()
	for name := range s.OnlineMap {
		if name == exclude {
			continue
		}
		recipients = append(recipients, name)
	}
	s.MapLock.RUnlock()
	_ = s.store.SaveMessageWithRecipients(msg, recipients)

	s.EnqueueServerMsg(serverMsgID)
	return
}
