package dao

import (
	"errors"

	"tet/src/storage"

	"gorm.io/gorm"
)

// SaveMessage 保存一条消息到数据库。
func SaveMessage(msg *storage.Message) error {
	if msg == nil {
		return errors.New("msg 不存在")
	}
	if storage.DB == nil {
		return errors.New("DB 未初始化")
	}
	return storage.DB.Create(msg).Error
}

// GetMessagesByChatID 按会话查询消息，支持分页。按 seq 降序（最近消息在前）。
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

// GetMessagesBySeq 按序号范围查询（增量同步），包含 startSeq 和 endSeq。
// 返回按 seq 升序的结果，便于按顺序重放。
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

// UpdateMessageStatus 根据 server_msg_id 更新消息状态（返回 gorm.ErrRecordNotFound 如果没有匹配行）。
func UpdateMessageStatus(serverMsgID uint64, status int8) error {
	if storage.DB == nil {
		return errors.New("database not initialized")
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

// GetLatestSeq 返回指定 chatID 的最大 seq（若无记录返回 0, nil）。
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
