package store

import (
	"context"
	"encoding/json"
	"time"

	"tet/src/cache"
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
				if err := storage.DB.Model(&storage.MessageRecipient{}).
					Where("server_msg_id = ? AND to_user = ?", serverMsgID, to).
					Updates(map[string]any{
						"status":        3,
						"retry_count":   e.Retry,
						"last_error":    errText,
						"next_retry_at": nil,
					}).Error; err != nil {
					return true, err
				}
				// invalidate caches
				if c := cache.Client(); c != nil {
					ctx := context.Background()
					_ = c.Del(ctx, cache.RecipientsKey(serverMsgID)).Err()
					_ = c.Del(ctx, cache.PendingUserKey(to)).Err()
					_ = c.Del(ctx, cache.PendingDueKey()).Err()
					_ = c.Del(ctx, cache.PendingRecoverKey()).Err()
				}
				return true, nil
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
		// invalidate caches
		if c := cache.Client(); c != nil {
			ctx := context.Background()
			_ = c.Del(ctx, cache.RecipientsKey(serverMsgID)).Err()
			_ = c.Del(ctx, cache.PendingUserKey(to)).Err()
			_ = c.Del(ctx, cache.PendingDueKey()).Err()
			_ = c.Del(ctx, cache.PendingRecoverKey()).Err()
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
	// invalidate due/recover caches so next collector will refresh
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		_ = c.Del(ctx, cache.PendingDueKey()).Err()
		_ = c.Del(ctx, cache.PendingRecoverKey()).Err()
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
	// try redis cache for due list
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		if v, err := c.Get(ctx, cache.PendingDueKey()).Result(); err == nil && v != "" {
			var out []string
			if err := json.Unmarshal([]byte(v), &out); err == nil {
				return dedupeStrings(out)
			}
		}
	}

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
	// cache short-term
	if len(out) > 0 {
		if c := cache.Client(); c != nil {
			ctx := context.Background()
			if b, err := json.Marshal(out); err == nil {
				_ = c.Set(ctx, cache.PendingDueKey(), b, 2*time.Second).Err()
			}
		}
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
	// try redis cache for recover list
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		if v, err := c.Get(ctx, cache.PendingRecoverKey()).Result(); err == nil && v != "" {
			var out []string
			if err := json.Unmarshal([]byte(v), &out); err == nil {
				return dedupeStrings(out)
			}
		}
	}

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
	if len(out) > 0 {
		if c := cache.Client(); c != nil {
			ctx := context.Background()
			if b, err := json.Marshal(out); err == nil {
				_ = c.Set(ctx, cache.PendingRecoverKey(), b, 2*time.Second).Err()
			}
		}
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

	// try per-user redis cache
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		if v, err := c.Get(ctx, cache.PendingUserKey(toUser)).Result(); err == nil && v != "" {
			var out []string
			if err := json.Unmarshal([]byte(v), &out); err == nil {
				if limit > 0 && len(out) > limit {
					return out[:limit]
				}
				return out
			}
		}
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
	if len(out) > 0 {
		if c := cache.Client(); c != nil {
			ctx := context.Background()
			if b, err := json.Marshal(out); err == nil {
				_ = c.Set(ctx, cache.PendingUserKey(toUser), b, 2*time.Second).Err()
			}
		}
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
