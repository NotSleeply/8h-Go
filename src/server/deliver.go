package server

import (
	"errors"
	"fmt"
	"time"
)

// DeliverWorker 从队列中读取 serverMsgID 并尝试将消息投递给接收者。
func (s *Server) DeliverWorker() {
	for serverMsgID := range s.DeliverQueue {
		// ensure in-flight marker is cleared after processing this id
		func(id string) {
			defer func() {
				s.deliverInFlightMu.Lock()
				delete(s.deliverInFlight, id)
				s.deliverInFlightMu.Unlock()
			}()
			msg := s.store.GetMessageByServerID(id)
			if msg == nil {
				return
			}
			recips := s.store.GetRecipients(id)
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
					dead, err := s.store.ScheduleRetry(id, r, errors.New("recipient offline"), s.maxDeliverRetry, s.retryBaseDelay)
					if err != nil {
						fmt.Println("[DeliverWorker] schedule retry failed:", err)
					}
					if dead {
						fmt.Printf("[DeliverWorker] dead-letter: msg=%s to=%s\n", id, r)
					}
					continue
				}
				deliver := &Message{
					Type:        TypeDeliver,
					ServerMsgID: id,
					ChatID:      msg.ChatID,
					From:        msg.From,
					To:          r,
					Seq:         msg.Seq,
					Body:        msg.Body,
					Ts:          msg.Ts,
				}
				if info.Sender != nil {
					fmt.Printf("[DeliverWorker] deliver msg=%s to=%s online=true\n", id, r)
					info.Sender.SendJSON(deliver)
				}
				_ = s.store.MarkDeliverySent(id, r, nil)
				// 标记最后发送时间（尽力记录）
				_ = time.Now()
			}
		}(serverMsgID)
	}
}
