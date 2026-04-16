package session

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"goim/internal/protocol"
)

func (u *User) useHelp() {
	msg := strings.Join([]string{
		"可用命令:",
		"  help                  查看帮助",
		"  who                   查看在线用户",
		"  rename|新昵称          修改昵称",
		"  to|用户名|消息         发送私聊消息",
		"  gc|群ID                创建群",
		"  gj|群ID                加入群",
		"  gl|群ID                退出群",
		"  gd|群ID                解散群(仅群主)",
		"  gk|群ID|用户名          踢人(管理员/群主)",
		"  ga|群ID|用户名          设管理员(仅群主)",
		"  gr|群ID|用户名          撤管理员(仅群主)",
		"  gm|群ID                查看群成员",
		"  gt|群ID|消息            发送群消息",
		"  history|用户名|条数     查询私聊历史（默认20）",
		"  read|server_msg_id      上报已读回执",
		"  stats                 查看服务端运行指标",
		"  exit                  退出聊天室",
	}, "\n")
	u.SendMsg(msg)
}

func (u *User) useWho() {
	if u.Server == nil {
		return
	}
	for _, info := range u.Server.ListOnlineUsers() {
		u.SendMsg("[" + info.Addr + "]" + info.Name + ":在线中...\n")
	}
}

func (u *User) useStats() {
	if u.Server != nil {
		u.SendMsg(u.Server.SnapshotStatsText())
	}
}

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
	if u.Server == nil {
		return
	}
	u.Server.UnregisterOnline(u.Name)
	old := u.Name
	u.Name = newName
	u.Server.RegisterOnline(u.Name, u, u.Addr)
	u.SendMsg("已将 " + old + " 更改为: " + newName + "\n")
}

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
	if u.Server == nil {
		return
	}
	req := &protocol.Message{
		Type:        protocol.TypeSend,
		ClientMsgID: fmt.Sprintf("c-%d", time.Now().UnixNano()),
		From:        u.Name,
		To:          toName,
		Body:        toMsg,
	}
	msgSaved, existing, err := u.Server.ProcessSend(req, []string{toName})
	if err != nil {
		u.SendMsg("消息发送失败: " + err.Error())
		return
	}
	if existing != nil {
		u.SendMsg("私聊消息幂等命中: " + existing.ServerMsgID)
		return
	}
	u.Server.EnqueueServerMsg(msgSaved.ServerMsgID)
	u.SendMsg("已将'" + toMsg + "' 消息发送 --> " + toName + "\n")
}

func (u *User) useRead(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：read|server_msg_id\n")
		return
	}
	if u.Server != nil {
		u.Server.HandleReadAck(u.Name, strings.TrimSpace(parts[1]))
	}
	u.SendMsg("已上报 read_ack: " + strings.TrimSpace(parts[1]))
}

func (u *User) useExit(msg string) {
	u.SendMsg("再见！欢迎下次再来~\n")
	u.Logout()
}

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
		u.SendMsg("暂无历史消息。\n")
		return
	}
	lines := []string{fmt.Sprintf("History with %s (count=%d)", peer, len(history))}
	for _, h := range history {
		t := time.Unix(h.Ts, 0).Format("2006-01-02 15:04:05")
		lines = append(lines, fmt.Sprintf("[%s] %s -> %s | seq=%d id=%s status=%d | %s",
			t, h.From, h.To, h.Seq, h.ServerMsgID, h.Status, h.Body))
	}
	u.SendMsg(strings.Join(lines, "\n") + "\n")
}

func (u *User) useGroupCreate(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gc|群ID\n")
		return
	}
	if err := u.Server.GroupManager().Create(strings.TrimSpace(parts[1]), u.Name); err != nil {
		u.SendMsg("创建群失败: " + err.Error() + "\n")
		return
	}
	u.SendMsg("创建群成功: " + strings.TrimSpace(parts[1]) + "\n")
}

func (u *User) useGroupJoin(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gj|群ID\n")
		return
	}
	if err := u.Server.GroupManager().Join(strings.TrimSpace(parts[1]), u.Name); err != nil {
		u.SendMsg("加入群失败: " + err.Error() + "\n")
		return
	}
	u.SendMsg("加入群成功: " + strings.TrimSpace(parts[1]) + "\n")
}

func (u *User) useGroupLeave(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gl|群ID\n")
		return
	}
	if err := u.Server.GroupManager().Leave(strings.TrimSpace(parts[1]), u.Name); err != nil {
		u.SendMsg("退出群失败: " + err.Error() + "\n")
		return
	}
	u.SendMsg("已退出群: " + strings.TrimSpace(parts[1]) + "\n")
}

func (u *User) useGroupDelete(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gd|群ID\n")
		return
	}
	if err := u.Server.GroupManager().Delete(strings.TrimSpace(parts[1]), u.Name); err != nil {
		u.SendMsg("解散群失败: " + err.Error() + "\n")
		return
	}
	u.SendMsg("已解散群: " + strings.TrimSpace(parts[1]) + "\n")
}

func (u *User) useGroupKick(msg string) {
	parts := strings.SplitN(msg, "|", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：gk|群ID|用户名\n")
		return
	}
	if err := u.Server.GroupManager().Kick(strings.TrimSpace(parts[1]), u.Name, strings.TrimSpace(parts[2])); err != nil {
		u.SendMsg("踢人失败: " + err.Error() + "\n")
		return
	}
	u.SendMsg("踢人成功: " + strings.TrimSpace(parts[2]) + "\n")
}

func (u *User) useGroupGrantAdmin(msg string) {
	parts := strings.SplitN(msg, "|", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：ga|群ID|用户名\n")
		return
	}
	if err := u.Server.GroupManager().GrantAdmin(strings.TrimSpace(parts[1]), u.Name, strings.TrimSpace(parts[2])); err != nil {
		u.SendMsg("设管理员失败: " + err.Error() + "\n")
		return
	}
	u.SendMsg("设管理员成功: " + strings.TrimSpace(parts[2]) + "\n")
}

func (u *User) useGroupRevokeAdmin(msg string) {
	parts := strings.SplitN(msg, "|", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：gr|群ID|用户名\n")
		return
	}
	if err := u.Server.GroupManager().RevokeAdmin(strings.TrimSpace(parts[1]), u.Name, strings.TrimSpace(parts[2])); err != nil {
		u.SendMsg("撤管理员失败: " + err.Error() + "\n")
		return
	}
	u.SendMsg("撤管理员成功: " + strings.TrimSpace(parts[2]) + "\n")
}

func (u *User) useGroupMembers(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gm|群ID\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	members := u.Server.GroupManager().Members(groupID)
	if len(members) == 0 {
		u.SendMsg("群不存在或无成员: " + groupID + "\n")
		return
	}
	u.SendMsg("群成员(" + groupID + "): " + strings.Join(members, ", ") + "\n")
}

func (u *User) useGroupTalk(msg string) {
	parts := strings.SplitN(msg, "|", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：gt|群ID|消息\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])
	if _, ok := u.Server.GroupManager().RoleOf(groupID, u.Name); !ok {
		u.SendMsg("发送失败: 你不在该群中。\n")
		return
	}
	members := u.Server.GroupManager().Members(groupID)
	recipients := make([]string, 0, len(members))
	for _, m := range members {
		if m != u.Name {
			recipients = append(recipients, m)
		}
	}
	req := &protocol.Message{
		Type:        protocol.TypeSend,
		ClientMsgID: fmt.Sprintf("gc-%d", time.Now().UnixNano()),
		ChatID:      "group:" + groupID,
		From:        u.Name,
		Body:        body,
	}
	msgSaved, existing, err := u.Server.ProcessSend(req, recipients)
	if err != nil {
		u.SendMsg("发送群消息失败: " + err.Error() + "\n")
		return
	}
	if existing != nil {
		u.SendMsg("群消息幂等命中: " + existing.ServerMsgID + "\n")
		return
	}
	u.Server.EnqueueServerMsg(msgSaved.ServerMsgID)
	u.SendMsg("群消息已入队: " + msgSaved.ServerMsgID + "\n")
}

func (u *User) useIllegal(msg string) {
	u.SendMsg("非法指令：(" + strings.TrimSpace(msg) + ") 命令不可识别!\n请使用 help 查看指令帮助。\n")
}
