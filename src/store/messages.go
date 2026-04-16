package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"tet/src/cache"
	"tet/src/protocol"
	"tet/src/storage"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
		// set recipients cache and invalidate per-user pending lists
		if c := cache.Client(); c != nil {
			ctx := context.Background()
			if b, err := json.Marshal(recipients); err == nil {
				_ = c.Set(ctx, cache.RecipientsKey(msg.ServerMsgID), b, 5*time.Second).Err()
			}
			for _, to := range recipients {
				if to == "" {
					continue
				}
				_ = c.Del(ctx, cache.PendingUserKey(to)).Err()
			}
			_ = c.Del(ctx, cache.PendingDueKey()).Err()
			_ = c.Del(ctx, cache.PendingRecoverKey()).Err()
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Historical pending records may reference old system messages that no longer exist.
			// Skip silently to avoid noisy startup logs.
			s.markRecipientRowsDeadIfMessageMissing(serverMsgID, "message not found")
			return nil
		}
		return nil
	}
	msg := fromDBMessage(&row)
	s.mu.Lock()
	s.saveMessageUnsafe(msg)
	s.mu.Unlock()
	return cloneMessage(msg)
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
