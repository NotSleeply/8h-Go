package dao

import (
	"errors"

	"goim/internal/storage"

	"gorm.io/gorm"
)

func SaveMessage(msg *storage.Message) error {
	if msg == nil {
		return errors.New("msg 不存在")
	}
	if storage.DB == nil {
		return errors.New("DB 未初始化")
	}
	return storage.DB.Create(msg).Error
}

func GetMessagesByChatID(chatID string, limit int, offset int) ([]*storage.Message, error) {
	if storage.DB == nil {
		return nil, errors.New("DB 未初始化")
	}
	var msgs []*storage.Message
	q := storage.DB.Where("chat_id = ?", chatID).Order("seq desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

func GetMessagesBySeq(chatID string, startSeq, endSeq uint64) ([]*storage.Message, error) {
	if storage.DB == nil {
		return nil, errors.New("database not initialized")
	}
	var msgs []*storage.Message
	if startSeq > endSeq {
		return nil, nil
	}
	if err := storage.DB.Where("chat_id = ? AND seq >= ? AND seq <= ?", chatID, startSeq, endSeq).
		Order("seq asc").Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

func UpdateMessageStatus(serverMsgID string, status int8) error {
	if storage.DB == nil {
		return errors.New("database not initialized")
	}
	if serverMsgID == "" {
		return errors.New("serverMsgID is empty")
	}
	res := storage.DB.Model(&storage.Message{}).Where("server_msg_id = ?", serverMsgID).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func GetLatestSeq(chatID string) (uint64, error) {
	if storage.DB == nil {
		return 0, errors.New("database not initialized")
	}
	var m storage.Message
	if err := storage.DB.Where("chat_id = ?", chatID).Order("seq desc").Limit(1).Take(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return m.Seq, nil
}
