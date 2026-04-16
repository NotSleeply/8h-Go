package dao

import (
	"errors"
	"time"

	"goim/internal/storage"

	"gorm.io/gorm"
)

func SaveMessageRecipient(recipient *storage.MessageRecipient) error {
	if recipient == nil {
		return errors.New("recipient is nil")
	}
	if storage.DB == nil {
		return errors.New("database not initialized")
	}
	return storage.DB.Create(recipient).Error
}

func MarkRecipientDelivered(serverMsgID, toUser string) error {
	if storage.DB == nil {
		return errors.New("database not initialized")
	}
	now := time.Now()
	res := storage.DB.Model(&storage.MessageRecipient{}).
		Where("server_msg_id = ? AND to_user = ?", serverMsgID, toUser).
		Updates(map[string]any{"status": 1, "acked_at": &now, "last_send_at": &now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func MarkRecipientRead(serverMsgID, toUser string) error {
	if storage.DB == nil {
		return errors.New("database not initialized")
	}
	now := time.Now()
	res := storage.DB.Model(&storage.MessageRecipient{}).
		Where("server_msg_id = ? AND to_user = ?", serverMsgID, toUser).
		Updates(map[string]any{"status": 2, "read_at": &now, "acked_at": &now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ListPendingByUser(toUser string, limit int) ([]*storage.MessageRecipient, error) {
	if storage.DB == nil {
		return nil, errors.New("database not initialized")
	}
	var out []*storage.MessageRecipient
	q := storage.DB.Where("to_user = ? AND status = 0", toUser).Order("id asc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
