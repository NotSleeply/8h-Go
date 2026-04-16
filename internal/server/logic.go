package server

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"goim/internal/protocol"
	"goim/internal/store"
	"goim/internal/utils"
)

type LogicService struct {
	store     store.Store
	idCounter uint64
}

func newLogicService(s store.Store) *LogicService {
	return &LogicService{store: s}
}

func (l *LogicService) GenerateServerMsgID(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" {
		p = "s"
	}
	seq := atomic.AddUint64(&l.idCounter, 1)
	return fmt.Sprintf("%s-%d-%d", p, time.Now().UnixMilli(), seq)
}

func (l *LogicService) ProcessSend(req *protocol.Message, recipients []string) (saved *protocol.Message, existing *protocol.Message, err error) {
	if req == nil {
		return nil, nil, fmt.Errorf("request is nil")
	}
	if strings.TrimSpace(req.From) == "" {
		return nil, nil, fmt.Errorf("from is empty")
	}
	if strings.TrimSpace(req.ClientMsgID) == "" {
		req.ClientMsgID = fmt.Sprintf("c-%d", time.Now().UnixNano())
	}
	if ex := l.store.GetMessageByClientID(req.From, req.ClientMsgID); ex != nil {
		return nil, ex, nil
	}
	chatID := strings.TrimSpace(req.ChatID)
	if chatID == "" && req.To != "" {
		chatID = utils.C2CChatID(req.From, req.To)
	}
	if chatID == "" {
		chatID = "broadcast"
	}
	seq, _ := l.store.NextSeq(chatID)
	msg := &protocol.Message{
		Type:        protocol.TypeDeliver,
		ServerMsgID: l.GenerateServerMsgID("s"),
		ClientMsgID: req.ClientMsgID,
		ChatID:      chatID,
		From:        req.From,
		To:          req.To,
		Seq:         seq,
		Body:        req.Body,
		Ts:          time.Now().Unix(),
	}
	if err := l.store.SaveMessageWithRecipients(msg, recipients); err != nil {
		return nil, nil, err
	}
	return msg, nil, nil
}

func (l *LogicService) HandleDeliverAck(username, serverMsgID string) {
	if username == "" || serverMsgID == "" {
		return
	}
	_ = l.store.MarkDeliveryAcked(serverMsgID, username)
}

func (l *LogicService) HandleReadAck(username, serverMsgID string) {
	if username == "" || serverMsgID == "" {
		return
	}
	_ = l.store.MarkDeliveryRead(serverMsgID, username)
}
