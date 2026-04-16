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

// Sender represents an entity that can receive messages from the server.
type Sender interface {
	SendJSON(m *protocol.Message)
	SendMsg(msg string)
}

// OnlineUserInfo is a lightweight descriptor of an online user.
type OnlineUserInfo struct {
	Name string
	Addr string
}

// ServerAPI defines the subset of server operations that session users depend on.
type ServerAPI interface {
	RegisterOnline(name string, s Sender, addr string)
	UnregisterOnline(name string)
	EnqueuePendingForUser(username string, limit int)
	BroadcastSystemEvent(body string, exclude string) (serverMsgID string, seq uint64)
	ProcessSend(req *protocol.Message, recipients []string) (*protocol.Message, *protocol.Message, error)
	HandleDeliverAck(username, serverMsgID string)
	HandleReadAck(username, serverMsgID string)
	EnqueueServerMsg(serverMsgID string)
	GetC2CHistory(userA, userB string, limit int) []*protocol.Message
	SnapshotStatsText() string
	MarkOutbound()
	ListOnlineUsers() []OnlineUserInfo
	// Expose group manager operations
	GroupManager() GroupManagerAPI
}

// Group management helpers exposed to session layer.
type GroupManagerAPI interface {
	Create(groupID, owner string) error
	Join(groupID, username string) error
	Leave(groupID, username string) error
	Delete(groupID, username string) error
	Kick(groupID, by, target string) error
	GrantAdmin(groupID, by, target string) error
	RevokeAdmin(groupID, by, target string) error
	Members(groupID string) []string
	RoleOf(groupID, username string) (string, bool)
}

