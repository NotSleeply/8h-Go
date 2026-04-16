package store

import (
	"context"
	"encoding/json"
	"time"

	"tet/src/cache"
	iface "tet/src/iface"
	"tet/src/storage"

	"gorm.io/gorm/clause"
)

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
	if err := storage.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return err
	}
	// invalidate recipient and pending caches for this key/user
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		_ = c.Del(ctx, cache.RecipientsKey(serverMsgID)).Err()
		_ = c.Del(ctx, cache.PendingUserKey(to)).Err()
	}
	return nil
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

	// try redis cache first
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		if val, err := c.Get(ctx, cache.RecipientsKey(serverMsgID)).Result(); err == nil && val != "" {
			var out []string
			if err := json.Unmarshal([]byte(val), &out); err == nil {
				// populate in-memory to avoid DB hits later
				s.mu.Lock()
				for _, to := range out {
					s.saveDeliveryUnsafe(serverMsgID, to)
				}
				s.mu.Unlock()
				return out
			}
		}
	}

	var rows []storage.MessageRecipient
	if err := storage.DB.Where("server_msg_id = ? AND status = 0", serverMsgID).Find(&rows).Error; err != nil {
		return nil
	}
	for _, row := range rows {
		// populate in-memory only
		s.mu.Lock()
		s.saveDeliveryUnsafe(row.ServerMsgID, row.ToUser)
		s.mu.Unlock()
		res = append(res, row.ToUser)
	}
	// write back to redis cache
	if len(res) > 0 {
		if c := cache.Client(); c != nil {
			ctx := context.Background()
			if b, err := json.Marshal(res); err == nil {
				_ = c.Set(ctx, cache.RecipientsKey(serverMsgID), b, 5*time.Second).Err()
			}
		}
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
	if err := storage.DB.Model(&storage.MessageRecipient{}).
		Where("server_msg_id = ? AND to_user = ?", serverMsgID, to).
		Updates(map[string]any{
			"status":   clause.Expr{SQL: "CASE WHEN status < 1 THEN 1 ELSE status END"},
			"acked_at": &now,
		}).Error; err != nil {
		return err
	}
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		_ = c.Del(ctx, cache.RecipientsKey(serverMsgID)).Err()
		_ = c.Del(ctx, cache.PendingUserKey(to)).Err()
	}
	return nil
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
	if err := storage.DB.Model(&storage.MessageRecipient{}).
		Where("server_msg_id = ? AND to_user = ?", serverMsgID, to).
		Updates(map[string]any{
			"status":   2,
			"acked_at": &now,
			"read_at":  &now,
		}).Error; err != nil {
		return err
	}
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		_ = c.Del(ctx, cache.RecipientsKey(serverMsgID)).Err()
		_ = c.Del(ctx, cache.PendingUserKey(to)).Err()
	}
	return nil
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
