package server

import (
	"time"
)

// DeliverWorker 从队列中读取 serverMsgID 并尝试将消息投递给接收者。
func (s *Server) DeliverWorker() {
	for serverMsgID := range s.DeliverQueue {
		msg := s.store.GetMessageByServerID(serverMsgID)
		if msg == nil {
			continue
		}
		recips := s.store.GetRecipients(serverMsgID)
		for _, r := range recips {
			s.MapLock.RLock()
			toUser, ok := s.OnlineMap[r]
			s.MapLock.RUnlock()
			if !ok {
				// 离线 — 保留为待处理，等待稍后同步
				continue
			}
			deliver := &Message{
				Type:        TypeDeliver,
				ServerMsgID: serverMsgID,
				ChatID:      msg.ChatID,
				From:        msg.From,
				To:          r,
				Seq:         msg.Seq,
				Body:        msg.Body,
				Ts:          msg.Ts,
			}
			toUser.SendJSON(deliver)
			_ = s.store.MarkDeliverySent(serverMsgID, r, nil)
			// 标记最后发送时间（尽力记录）
			_ = time.Now()
		}
	}
}
