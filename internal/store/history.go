package store

import (
	"sort"

	"goim/internal/protocol"
	"goim/internal/storage"
)

func (s *InMemoryStore) GetC2CHistory(userA, userB string, limit int) []*protocol.Message {
	if userA == "" || userB == "" {
		return nil
	}
	if !s.persistent {
		return s.getC2CHistoryFromMemory(userA, userB, limit)
	}
	var rows []storage.Message
	q := storage.DB.Where(
		"(send_id = ? AND recv_id = ?) OR (send_id = ? AND recv_id = ?)",
		userA, userB, userB, userA,
	).Order("seq desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil
	}
	out := make([]*protocol.Message, 0, len(rows))
	for _, row := range rows {
		msg := fromDBMessage(&row)
		toUser := userA
		if msg.From == userA {
			toUser = userB
		}
		var r storage.MessageRecipient
		if err := storage.DB.Where("server_msg_id = ? AND to_user = ?", msg.ServerMsgID, toUser).Take(&r).Error; err == nil {
			msg.Status = r.Status
		}
		out = append(out, msg)
	}
	reverseMessages(out)
	return out
}

func (s *InMemoryStore) getC2CHistoryFromMemory(userA, userB string, limit int) []*protocol.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*protocol.Message, 0, len(s.messages))
	for _, msg := range s.messages {
		if (msg.From == userA && msg.To == userB) || (msg.From == userB && msg.To == userA) {
			out = append(out, cloneMessage(msg))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}
