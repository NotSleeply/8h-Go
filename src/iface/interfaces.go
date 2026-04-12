package iface

import (
	"time"

	"tet/src/protocol"
)

type DeliveryStatusStats struct {
	Pending   int64
	Delivered int64
	Read      int64
	Dead      int64
}

// Store defines the minimal storage operations used by server and logic layers.
type Store interface {
	NextSeq(chatID string) (uint64, error)
	SaveMessage(msg *protocol.Message) error
	SaveMessageWithRecipients(msg *protocol.Message, recipients []string) error
	GetMessageByClientID(from, clientMsgID string) *protocol.Message
	GetMessageByServerID(serverMsgID string) *protocol.Message
	SaveDelivery(serverMsgID, to string) error
	GetRecipients(serverMsgID string) []string
	MarkDeliverySent(serverMsgID, to string, sendErr error) error
	MarkDeliveryAcked(serverMsgID, to string) error
	MarkDeliveryRead(serverMsgID, to string) error
	ScheduleRetry(serverMsgID, to string, lastErr error, maxRetries int, baseBackoff time.Duration) (bool, error)
	GetDueRetryServerMsgIDs(limit int) []string
	DeliveryStats() DeliveryStatusStats
	RecoverPendingServerMsgIDs(limit int) []string
	ListPendingServerMsgIDsByUser(toUser string, limit int) []string
	GetC2CHistory(userA, userB string, limit int) []*protocol.Message
}

// MessageBusIface is an abstraction over the message bus (redis/kafka/local).
type MessageBusIface interface {
	Publish(serverMsgID string, fallback func(string))
	StartConsumers(push func(string))
	Close()
	Mode() string
}
