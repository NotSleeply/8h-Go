package store

import (
	"time"

	"tet/src/storage"
)

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

func (s *InMemoryStore) markRecipientRowsDeadIfMessageMissing(serverMsgID, reason string) {
	if !s.persistent || serverMsgID == "" {
		return
	}
	if reason == "" {
		reason = "message not found"
	}
	_ = storage.DB.Model(&storage.MessageRecipient{}).
		Where("server_msg_id = ? AND status = 0", serverMsgID).
		Updates(map[string]any{
			"status":        3,
			"last_error":    reason,
			"next_retry_at": nil,
		}).Error
}
