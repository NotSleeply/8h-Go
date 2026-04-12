package logic

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	iface "tet/src/iface"
	"tet/src/protocol"
)

// LogicService encapsulates core message business logic.
type LogicService struct {
	store     iface.Store
	idCounter uint64
}

func NewLogicService(store iface.Store) *LogicService {
	return &LogicService{store: store}
}

func (l *LogicService) GenerateServerMsgID(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" {
		p = "s"
	}
	seq := atomic.AddUint64(&l.idCounter, 1)
	return fmt.Sprintf("%s-%d-%d", p, time.Now().UnixMilli(), seq)
}

func c2cChatID(a, b string) string {
	if a <= b {
		return "c2c:" + a + ":" + b
	}
	return "c2c:" + b + ":" + a
}

// ProcessSend handles idempotency, seq/id generation and transactional persistence.
// Returns:
// - saved message when newly persisted
// - existing message when dedupe hits
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
		chatID = c2cChatID(req.From, req.To)
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
