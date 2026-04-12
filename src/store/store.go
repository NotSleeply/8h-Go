package store

import (
	"errors"
	"sort"
	"sync"
	"time"

	iface "tet/src/iface"
	"tet/src/protocol"
	"tet/src/storage"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DeliveryEntry struct {
	To          string
	Acked       bool
	Read        bool
	Dead        bool
	Retry       int
	LastSent    int64
	NextRetryAt int64
	LastError   string
}

// InMemoryStore is a hybrid store:
// - fast-path cache in memory
// - optional persistence in SQLite when storage.DB is initialized
type InMemoryStore struct {
	mu         sync.Mutex
	nextSeq    map[string]uint64                       // chatID -> next seq
	messages   map[string]*protocol.Message            // serverMsgID -> Message
	byClient   map[string]map[string]*protocol.Message // from -> clientMsgID -> Message
	deliveries map[string]map[string]*DeliveryEntry    // serverMsgID -> to -> DeliveryEntry

	persistent bool
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		nextSeq:    make(map[string]uint64),
		messages:   make(map[string]*protocol.Message),
		byClient:   make(map[string]map[string]*protocol.Message),
		deliveries: make(map[string]map[string]*DeliveryEntry),
		persistent: storage.DB != nil,
	}
}

func (s *InMemoryStore) NextSeq(chatID string) (uint64, error) {
	if chatID == "" {
		chatID = "default"
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.nextSeq[chatID]; !ok && s.persistent {
		var maxSeq uint64
		if err := storage.DB.Model(&storage.Message{}).
			Where("chat_id = ?", chatID).
			Select("COALESCE(MAX(seq), 0)").
			Scan(&maxSeq).Error; err == nil {
			s.nextSeq[chatID] = maxSeq
		}
	}
	seq := s.nextSeq[chatID] + 1
	s.nextSeq[chatID] = seq
	return seq, nil
}

func (s *InMemoryStore) SaveMessage(msg *protocol.Message) error {
	return s.SaveMessageWithRecipients(msg, nil)
}

func (s *InMemoryStore) SaveMessageWithRecipients(msg *protocol.Message, recipients []string) error {
	if msg == nil || msg.ServerMsgID == "" {
		return errors.New("invalid message")
	}
	if s.persistent {
		dbMsg := toDBMessage(msg)
		if err := storage.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(dbMsg).Error; err != nil {
				return err
			}
			for _, to := range recipients {
				if to == "" {
					continue
				}
				row := &storage.MessageRecipient{
					ServerMsgID: msg.ServerMsgID,
					ToUser:      to,
					Status:      0,
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveMessageUnsafe(msg)
	for _, to := range recipients {
		s.saveDeliveryUnsafe(msg.ServerMsgID, to)
	}
	return nil
}

func (s *InMemoryStore) saveMessageUnsafe(msg *protocol.Message) {
	s.messages[msg.ServerMsgID] = cloneMessage(msg)
	if msg.From != "" && msg.ClientMsgID != "" {
		if _, ok := s.byClient[msg.From]; !ok {
			s.byClient[msg.From] = make(map[string]*protocol.Message)
		}
		s.byClient[msg.From][msg.ClientMsgID] = cloneMessage(msg)
	}
}

func (s *InMemoryStore) GetMessageByClientID(from, clientMsgID string) *protocol.Message {
	s.mu.Lock()
	if from != "" && clientMsgID != "" {
		if m, ok := s.byClient[from]; ok {
			if msg, ok2 := m[clientMsgID]; ok2 {
				out := cloneMessage(msg)
				s.mu.Unlock()
				return out
			}
		}
	}
	s.mu.Unlock()

	if !s.persistent || from == "" || clientMsgID == "" {
		return nil
	}

	var row storage.Message
	if err := storage.DB.Where("send_id = ? AND client_msg_id = ?", from, clientMsgID).Take(&row).Error; err != nil {
		return nil
	}
	msg := fromDBMessage(&row)
	s.mu.Lock()
	s.saveMessageUnsafe(msg)
	s.mu.Unlock()
	return cloneMessage(msg)
}

func (s *InMemoryStore) GetMessageByServerID(serverMsgID string) *protocol.Message {
	s.mu.Lock()
	if msg, ok := s.messages[serverMsgID]; ok {
		out := cloneMessage(msg)
		s.mu.Unlock()
		return out
	}
	s.mu.Unlock()

	if !s.persistent || serverMsgID == "" {
		return nil
	}

	var row storage.Message
	if err := storage.DB.Where("server_msg_id = ?", serverMsgID).Take(&row).Error; err != nil {
		return nil
	}
	msg := fromDBMessage(&row)
	s.mu.Lock()
	s.saveMessageUnsafe(msg)
	s.mu.Unlock()
	return cloneMessage(msg)
}

func (s *InMemoryStore) SaveDelivery(serverMsgID, to string) error {
	if serverMsgID == "" || to == "" {
		return nil
	}
	s.mu.Lock()
	s.saveDeliveryUnsafe(serverMsgID, to)
	s.mu.Unlock()

	if !s.persistent {
		return nil
	}
	row := &storage.MessageRecipient{
		ServerMsgID: serverMsgID,
		ToUser:      to,
		Status:      0,
	}
	return storage.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error
}

func (s *InMemoryStore) saveDeliveryUnsafe(serverMsgID, to string) {
	if _, ok := s.deliveries[serverMsgID]; !ok {
		s.deliveries[serverMsgID] = make(map[string]*DeliveryEntry)
	}
	if _, exists := s.deliveries[serverMsgID][to]; !exists {
		s.deliveries[serverMsgID][to] = &DeliveryEntry{To: to}
	}
}

func (s *InMemoryStore) GetRecipients(serverMsgID string) []string {
	s.mu.Lock()
	var res []string
	if m, ok := s.deliveries[serverMsgID]; ok {
		for to, entry := range m {
			if entry != nil && !entry.Acked && !entry.Read && !entry.Dead {
				res = append(res, to)
			}
		}
	}
	s.mu.Unlock()

	if len(res) > 0 || !s.persistent {
		return res
	}

	var rows []storage.MessageRecipient
	if err := storage.DB.Where("server_msg_id = ? AND status = 0", serverMsgID).Find(&rows).Error; err != nil {
		return nil
	}
	for _, row := range rows {
		_ = s.SaveDelivery(row.ServerMsgID, row.ToUser)
		res = append(res, row.ToUser)
	}
	return res
}

func (s *InMemoryStore) MarkDeliverySent(serverMsgID, to string, sendErr error) error {
	s.mu.Lock()
	if m, ok := s.deliveries[serverMsgID]; ok {
		if e, ok2 := m[to]; ok2 {
			e.Retry++
			e.LastSent = time.Now().Unix()
		}
	}
	s.mu.Unlock()

	if !s.persistent || serverMsgID == "" || to == "" {
		return nil
	}

	updates := map[string]any{
		"retry_count":  clause.Expr{SQL: "retry_count + 1"},
		"last_send_at": time.Now(),
	}
	if sendErr != nil {
		updates["last_error"] = sendErr.Error()
	} else {
		updates["last_error"] = ""
	}
	return storage.DB.Model(&storage.MessageRecipient{}).
		Where("server_msg_id = ? AND to_user = ?", serverMsgID, to).
		Updates(updates).Error
}

func (s *InMemoryStore) MarkDeliveryAcked(serverMsgID, to string) error {
	s.mu.Lock()
	if m, ok := s.deliveries[serverMsgID]; ok {
		if e, ok2 := m[to]; ok2 {
			e.Acked = true
		}
	}
	s.mu.Unlock()

	if !s.persistent {
		return nil
	}
	now := time.Now()
	return storage.DB.Model(&storage.MessageRecipient{}).
		Where("server_msg_id = ? AND to_user = ?", serverMsgID, to).
		Updates(map[string]any{
			"status":   clause.Expr{SQL: "CASE WHEN status < 1 THEN 1 ELSE status END"},
			"acked_at": &now,
		}).Error
}

func (s *InMemoryStore) MarkDeliveryRead(serverMsgID, to string) error {
	s.mu.Lock()
	if m, ok := s.deliveries[serverMsgID]; ok {
		if e, ok2 := m[to]; ok2 {
			e.Acked = true
			e.Read = true
		}
	}
	s.mu.Unlock()

	if !s.persistent {
		return nil
	}
	now := time.Now()
	return storage.DB.Model(&storage.MessageRecipient{}).
		Where("server_msg_id = ? AND to_user = ?", serverMsgID, to).
		Updates(map[string]any{
			"status":   2,
			"acked_at": &now,
			"read_at":  &now,
		}).Error
}

func (s *InMemoryStore) ScheduleRetry(serverMsgID, to string, lastErr error, maxRetries int, baseBackoff time.Duration) (bool, error) {
	if serverMsgID == "" || to == "" {
		return false, nil
	}
	if maxRetries <= 0 {
		maxRetries = 5
	}
	if baseBackoff <= 0 {
		baseBackoff = 2 * time.Second
	}

	errText := "delivery failed"
	if lastErr != nil {
		errText = lastErr.Error()
	}

	now := time.Now()
	s.mu.Lock()
	if m, ok := s.deliveries[serverMsgID]; ok {
		if e, ok2 := m[to]; ok2 && e != nil {
			e.Retry++
			e.LastError = errText
			if e.Retry >= maxRetries {
				e.Dead = true
				s.mu.Unlock()
				if !s.persistent {
					return true, nil
				}
				return true, storage.DB.Model(&storage.MessageRecipient{}).
					Where("server_msg_id = ? AND to_user = ?", serverMsgID, to).
					Updates(map[string]any{
						"status":        3,
						"retry_count":   e.Retry,
						"last_error":    errText,
						"next_retry_at": nil,
					}).Error
			}
			exp := e.Retry - 1
			if exp > 6 {
				exp = 6
			}
			delay := baseBackoff * time.Duration(1<<exp)
			e.NextRetryAt = now.Add(delay).Unix()
		}
	}
	s.mu.Unlock()

	if !s.persistent {
		return false, nil
	}

	var row storage.MessageRecipient
	if err := storage.DB.Where("server_msg_id = ? AND to_user = ?", serverMsgID, to).Take(&row).Error; err != nil {
		return false, err
	}
	if row.Status >= 1 {
		return false, nil
	}

	nextRetryCount := row.RetryCount + 1
	if nextRetryCount >= maxRetries {
		if err := storage.DB.Model(&storage.MessageRecipient{}).
			Where("id = ?", row.ID).
			Updates(map[string]any{
				"status":        3,
				"retry_count":   nextRetryCount,
				"last_error":    errText,
				"next_retry_at": nil,
			}).Error; err != nil {
			return false, err
		}
		return true, nil
	}
	exp := nextRetryCount - 1
	if exp > 6 {
		exp = 6
	}
	delay := baseBackoff * time.Duration(1<<exp)
	nextRetryAt := now.Add(delay)
	if err := storage.DB.Model(&storage.MessageRecipient{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"retry_count":   nextRetryCount,
			"last_error":    errText,
			"next_retry_at": &nextRetryAt,
		}).Error; err != nil {
		return false, err
	}
	return false, nil
}

func (s *InMemoryStore) GetDueRetryServerMsgIDs(limit int) []string {
	now := time.Now()
	if !s.persistent {
		s.mu.Lock()
		defer s.mu.Unlock()
		var out []string
		for serverMsgID, m := range s.deliveries {
			for _, e := range m {
				if e == nil || e.Acked || e.Read || e.Dead {
					continue
				}
				if e.NextRetryAt > 0 && e.NextRetryAt <= now.Unix() {
					out = append(out, serverMsgID)
					break
				}
			}
		}
		if limit > 0 && len(out) > limit {
			out = out[:limit]
		}
		return dedupeStrings(out)
	}

	var rows []storage.MessageRecipient
	q := storage.DB.Where("status = 0 AND next_retry_at IS NOT NULL AND next_retry_at <= ?", now).Order("next_retry_at asc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil
	}
	var out []string
	for _, row := range rows {
		_ = s.SaveDelivery(row.ServerMsgID, row.ToUser)
		out = append(out, row.ServerMsgID)
	}
	return dedupeStrings(out)
}

func (s *InMemoryStore) DeliveryStats() iface.DeliveryStatusStats {
	if !s.persistent {
		s.mu.Lock()
		defer s.mu.Unlock()
		var st iface.DeliveryStatusStats
		for _, m := range s.deliveries {
			for _, e := range m {
				if e == nil {
					continue
				}
				if e.Dead {
					st.Dead++
					continue
				}
				if e.Read {
					st.Read++
					continue
				}
				if e.Acked {
					st.Delivered++
					continue
				}
				st.Pending++
			}
		}
		return st
	}

	var rows []struct {
		Status int8
		Count  int64
	}
	_ = storage.DB.Model(&storage.MessageRecipient{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&rows).Error

	var st iface.DeliveryStatusStats
	for _, row := range rows {
		switch row.Status {
		case 0:
			st.Pending = row.Count
		case 1:
			st.Delivered = row.Count
		case 2:
			st.Read = row.Count
		case 3:
			st.Dead = row.Count
		}
	}
	return st
}

func (s *InMemoryStore) RecoverPendingServerMsgIDs(limit int) []string {
	if !s.persistent {
		s.mu.Lock()
		defer s.mu.Unlock()
		out := make([]string, 0, len(s.deliveries))
		for serverMsgID, m := range s.deliveries {
			for _, e := range m {
				if e != nil && !e.Acked && !e.Read {
					out = append(out, serverMsgID)
					break
				}
			}
		}
		return dedupeStrings(out)
	}

	var rows []storage.MessageRecipient
	q := storage.DB.Where("status = 0").Order("id asc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		_ = s.SaveDelivery(row.ServerMsgID, row.ToUser)
		out = append(out, row.ServerMsgID)
	}
	return dedupeStrings(out)
}

func (s *InMemoryStore) ListPendingServerMsgIDsByUser(toUser string, limit int) []string {
	if toUser == "" {
		return nil
	}
	if !s.persistent {
		s.mu.Lock()
		defer s.mu.Unlock()
		var out []string
		for serverMsgID, m := range s.deliveries {
			if e, ok := m[toUser]; ok && e != nil && !e.Acked && !e.Read {
				out = append(out, serverMsgID)
			}
		}
		if limit > 0 && len(out) > limit {
			out = out[:limit]
		}
		return out
	}

	var rows []storage.MessageRecipient
	q := storage.DB.Where("to_user = ? AND status = 0", toUser).Order("id asc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		_ = s.SaveDelivery(row.ServerMsgID, row.ToUser)
		out = append(out, row.ServerMsgID)
	}
	return dedupeStrings(out)
}

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
	sort.Slice(out, func(i, j int) bool {
		return out[i].Seq < out[j].Seq
	})
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func reverseMessages(in []*protocol.Message) {
	for i, j := 0, len(in)-1; i < j; i, j = i+1, j-1 {
		in[i], in[j] = in[j], in[i]
	}
}

func cloneMessage(m *protocol.Message) *protocol.Message {
	if m == nil {
		return nil
	}
	cp := *m
	return &cp
}

func toDBMessage(m *protocol.Message) *storage.Message {
	if m == nil {
		return nil
	}
	return &storage.Message{
		ClientMsgID: m.ClientMsgID,
		ServerMsgID: m.ServerMsgID,
		Seq:         m.Seq,
		ChatType:    1,
		ChatID:      m.ChatID,
		SendID:      m.From,
		RecvID:      m.To,
		ContentType: 1,
		Content:     m.Body,
		Status:      0,
	}
}

func fromDBMessage(m *storage.Message) *protocol.Message {
	if m == nil {
		return nil
	}
	return &protocol.Message{
		Type:        protocol.TypeDeliver,
		ClientMsgID: m.ClientMsgID,
		ServerMsgID: m.ServerMsgID,
		ChatID:      m.ChatID,
		From:        m.SendID,
		To:          m.RecvID,
		Seq:         m.Seq,
		Body:        m.Content,
		Ts:          m.CreateTime.Unix(),
	}
}
