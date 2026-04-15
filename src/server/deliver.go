package server

import (
	"errors"
	"fmt"
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
		if msg.To != "" {
			seen := false
			for _, r := range recips {
				if r == msg.To {
					seen = true
					break
				}
			}
			if !seen {
				recips = append(recips, msg.To)
			}
		}
		for _, r := range recips {
			s.MapLock.RLock()
			info, ok := s.OnlineMap[r]
			s.MapLock.RUnlock()
			if !ok {
				dead, err := s.store.ScheduleRetry(serverMsgID, r, errors.New("recipient offline"), s.maxDeliverRetry, s.retryBaseDelay)
				if err != nil {
					fmt.Println("[DeliverWorker] schedule retry failed:", err)
				}
				if dead {
					fmt.Printf("[DeliverWorker] dead-letter: msg=%s to=%s\n", serverMsgID, r)
				}
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
			if info.Sender != nil {
				fmt.Printf("[DeliverWorker] deliver msg=%s to=%s online=true\n", serverMsgID, r)
				info.Sender.SendJSON(deliver)
			}
			_ = s.store.MarkDeliverySent(serverMsgID, r, nil)
			// 标记最后发送时间（尽力记录）
			_ = time.Now()
		}
	}
}
