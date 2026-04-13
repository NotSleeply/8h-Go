package server

import (
	"fmt"
	"time"

	iface "tet/src/iface"
	"tet/src/protocol"

	group "tet/src/group"
)

// RegisterOnline 将一个实现了 iface.Sender 的会话注册到在线列表。
func (s *Server) RegisterOnline(name string, sender iface.Sender, addr string) {
	if name == "" || sender == nil {
		return
	}
	s.MapLock.Lock()
	s.OnlineMap[name] = OnlineInfo{Sender: sender, Addr: addr}
	s.MapLock.Unlock()
}

// UnregisterOnline 从在线列表移除指定用户。
func (s *Server) UnregisterOnline(name string) {
	if name == "" {
		return
	}
	s.MapLock.Lock()
	delete(s.OnlineMap, name)
	s.MapLock.Unlock()
}

// ProcessSend 委托到逻辑层执行消息持久化与幂等处理。
func (s *Server) ProcessSend(req *protocol.Message, recipients []string) (*protocol.Message, *protocol.Message, error) {
	if s.logic == nil {
		return nil, nil, fmt.Errorf("logic not initialized")
	}
	return s.logic.ProcessSend(req, recipients)
}

// HandleDeliverAck 代理到逻辑层
func (s *Server) HandleDeliverAck(username, serverMsgID string) {
	if s.logic == nil {
		return
	}
	s.logic.HandleDeliverAck(username, serverMsgID)
}

// HandleReadAck 代理到逻辑层
func (s *Server) HandleReadAck(username, serverMsgID string) {
	if s.logic == nil {
		return
	}
	s.logic.HandleReadAck(username, serverMsgID)
}

// SnapshotStatsText 返回可读的运行时统计信息文本。
func (s *Server) SnapshotStatsText() string {
	st := s.SnapshotStats()
	return fmt.Sprintf(
		"Server Stats\nstart_at: %s\nuptime: %s\nmq_mode: %s\nonline_users: %d\nactive_conn: %d\ntotal_conn: %d\nrejected_conn: %d\ninbound_msgs: %d\noutbound_msgs: %d\nthroughput: %.2f msg/s\ndeliver_queue_len: %d\ndelivery_pending: %d\ndelivery_delivered: %d\ndelivery_read: %d\ndead_letter: %d",
		st.StartAt.Format(time.RFC3339),
		st.Uptime.Truncate(time.Second),
		st.MQMode,
		st.OnlineUsers,
		st.ActiveConn,
		st.TotalConnections,
		st.RejectedConn,
		st.InboundMessages,
		st.OutboundMessages,
		st.MsgPerSec,
		st.DeliverQueueLen,
		st.PendingDeliveries,
		st.DeliveredCount,
		st.ReadCount,
		st.DeadLetterCount,
	)
}

// MarkOutbound 被 session 或发送者调用以标记一条已出站消息。
func (s *Server) MarkOutbound() {
	s.markOutboundMessage()
}

// ListOnlineUsers 返回在线用户的轻量描述。
func (s *Server) ListOnlineUsers() []iface.OnlineUserInfo {
	s.MapLock.RLock()
	defer s.MapLock.RUnlock()
	out := make([]iface.OnlineUserInfo, 0, len(s.OnlineMap))
	for name, info := range s.OnlineMap {
		out = append(out, iface.OnlineUserInfo{Name: name, Addr: info.Addr})
	}
	return out
}

// GroupManager 返回底层的群管理器实现
func (s *Server) GroupManager() iface.GroupManagerAPI {
	if s.groupManager == nil {
		return nil
	}
	return &groupAdapter{gm: s.groupManager}
}

// groupAdapter adapts the concrete group.GroupManager to the iface.GroupManagerAPI
type groupAdapter struct {
	gm *group.GroupManager
}

func (a *groupAdapter) Create(groupID, owner string) error    { return a.gm.Create(groupID, owner) }
func (a *groupAdapter) Join(groupID, username string) error   { return a.gm.Join(groupID, username) }
func (a *groupAdapter) Leave(groupID, username string) error  { return a.gm.Leave(groupID, username) }
func (a *groupAdapter) Delete(groupID, username string) error { return a.gm.Delete(groupID, username) }
func (a *groupAdapter) Kick(groupID, by, target string) error { return a.gm.Kick(groupID, by, target) }
func (a *groupAdapter) GrantAdmin(groupID, by, target string) error {
	return a.gm.GrantAdmin(groupID, by, target)
}
func (a *groupAdapter) RevokeAdmin(groupID, by, target string) error {
	return a.gm.RevokeAdmin(groupID, by, target)
}
func (a *groupAdapter) Members(groupID string) []string { return a.gm.Members(groupID) }
func (a *groupAdapter) RoleOf(groupID, username string) (string, bool) {
	role, ok := a.gm.RoleOf(groupID, username)
	if !ok {
		return "", false
	}
	switch role {
	case group.GroupRoleOwner:
		return "owner", true
	case group.GroupRoleAdmin:
		return "admin", true
	default:
		return "member", true
	}
}
