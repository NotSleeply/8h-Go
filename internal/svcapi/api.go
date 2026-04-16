package svcapi

import "goim/internal/protocol"

type Sender interface {
	SendJSON(m *protocol.Message)
	SendMsg(msg string)
}

type OnlineUserInfo struct {
	Name string
	Addr string
}

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
	GroupManager() GroupManagerAPI
}
