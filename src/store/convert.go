package store

import (
	"tet/src/protocol"
	"tet/src/storage"
)

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
