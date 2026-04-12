package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (u *User) useHelp() {
	msg := strings.Join([]string{
		"可用命令:",
		"  help                  查看帮助",
		"  who                   查看在线用户",
		"  rename|新昵称          修改昵称",
		"  to|用户名|消息         发送私聊消息",
		"  history|用户名|条数     查询私聊历史（默认20）",
		"  read|server_msg_id      上报已读回执",
		"  stats                 查看服务端运行指标",
		"  exit                  退出聊天室",
	}, "\n")
	u.SendMsg(msg)
}

// 查询在线用户
func (u *User) useWho() {
	u.Server.MapLock.Lock()
	for _, user := range u.Server.OnlineMap {
		msg := "[" + user.Addr + "]" + user.Name + ":" + "在线中…" + "\n"
		u.SendMsg(msg)
	}
	u.Server.MapLock.Unlock()
}

func (u *User) useStats() {
	s := u.Server.SnapshotStats()
	msg := fmt.Sprintf(
		"Server Stats\nstart_at: %s\nuptime: %s\nonline_users: %d\nactive_conn: %d\ntotal_conn: %d\nrejected_conn: %d\ninbound_msgs: %d\noutbound_msgs: %d\nthroughput: %.2f msg/s\ndeliver_queue_len: %d",
		s.StartAt.Format(time.RFC3339),
		s.Uptime.Truncate(time.Second),
		s.OnlineUsers,
		s.ActiveConn,
		s.TotalConnections,
		s.RejectedConn,
		s.InboundMessages,
		s.OutboundMessages,
		s.MsgPerSec,
		s.DeliverQueueLen,
	)
	u.SendMsg(msg)
}

// 更改姓名
func (u *User) useRename(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：rename|新昵称，例如：rename|alice\n")
		return
	}

	newName := strings.TrimSpace(parts[1])
	if len(newName) > 20 {
		u.SendMsg("昵称过长，最多 20 个字符。\n")
		return
	}
	if strings.ContainsAny(newName, " \t\n\r|") {
		u.SendMsg("昵称包含非法字符，请避免空格或 `|`。\n")
		return
	}

	u.Server.MapLock.Lock()
	if _, ok := u.Server.OnlineMap[newName]; ok {
		u.Server.MapLock.Unlock()
		u.SendMsg("该昵称已被占用，请选择其他昵称。例如：rename|bob\n")
		return
	}
	delete(u.Server.OnlineMap, u.Name)
	u.Server.OnlineMap[newName] = u
	u.Server.MapLock.Unlock()

	oldName := u.Name
	u.Name = newName
	newMsg := "已将 " + oldName + " 更改为: " + newName + "\n"
	u.SendMsg(newMsg)
}

// 私聊
func (u *User) useChat(msg string) {
	parts := strings.SplitN(msg, "|", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：to|用户名|消息，例如：to|bob|你好\n")
		return
	}

	toName := strings.TrimSpace(parts[1])
	toMsg := strings.TrimSpace(parts[2])
	if toName == u.Name {
		u.SendMsg("不能给自己发送私聊消息。\n")
		return
	}

	// 构造 send 协议并通过服务器发送（会返回 send_ack）
	clientID := fmt.Sprintf("c-%d", time.Now().UnixNano())
	req := &Message{
		Type:        TypeSend,
		ClientMsgID: clientID,
		ChatID:      "",
		From:        u.Name,
		To:          toName,
		Body:        toMsg,
	}
	u.Server.HandleClientSend(u, req)
}

// 已读回执
func (u *User) useRead(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：read|server_msg_id\n")
		return
	}
	serverMsgID := strings.TrimSpace(parts[1])
	u.Server.HandleReadAck(u, &Message{
		Type:        TypeReadAck,
		ServerMsgID: serverMsgID,
	})
	u.SendMsg("已上报 read_ack: " + serverMsgID)
}

// 查询私聊历史
func (u *User) useHistory(msg string) {
	parts := strings.Split(msg, "|")
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：history|用户名|条数(可选)\n")
		return
	}
	peer := strings.TrimSpace(parts[1])
	limit := 20
	if len(parts) >= 3 && strings.TrimSpace(parts[2]) != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	history := u.Server.GetC2CHistory(u.Name, peer, limit)
	if len(history) == 0 {
		u.SendMsg("暂无历史消息。")
		return
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("History with %s (count=%d)", peer, len(history)))
	for _, h := range history {
		t := time.Unix(h.Ts, 0).Format("2006-01-02 15:04:05")
		lines = append(lines, fmt.Sprintf("[%s] %s -> %s | seq=%d id=%s status=%d | %s",
			t, h.From, h.To, h.Seq, h.ServerMsgID, h.Status, h.Body))
	}
	u.SendMsg(strings.Join(lines, "\n"))
}

// 退出聊天室
func (u *User) useExit(msg string) {
	u.SendMsg("再见！欢迎下次再来~\n")
	u.Logout()
}

// 非法指令
func (u *User) useIllegal(msg string) {
	msg = strings.TrimSpace(msg)
	msg = "非法指令：(" + msg + ") 命令不可识别!\n" + "请使用 help 查看指令帮助。\n"
	u.SendMsg(msg)
}
