package server

import (
	"fmt"
	"strings"
	"time"
)

func (u *User) useGroupCreate(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gc|群ID\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	if err := u.Server.groupManager.Create(groupID, u.Name); err != nil {
		u.SendMsg("创建群失败: " + err.Error())
		return
	}
	u.SendMsg("创建群成功: " + groupID)
}

func (u *User) useGroupJoin(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gj|群ID\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	if err := u.Server.groupManager.Join(groupID, u.Name); err != nil {
		u.SendMsg("加入群失败: " + err.Error())
		return
	}
	u.SendMsg("加入群成功: " + groupID)
}

func (u *User) useGroupLeave(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gl|群ID\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	if err := u.Server.groupManager.Leave(groupID, u.Name); err != nil {
		u.SendMsg("退出群失败: " + err.Error())
		return
	}
	u.SendMsg("已退出群: " + groupID)
}

func (u *User) useGroupDelete(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gd|群ID\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	if err := u.Server.groupManager.Delete(groupID, u.Name); err != nil {
		u.SendMsg("解散群失败: " + err.Error())
		return
	}
	u.SendMsg("已解散群: " + groupID)
}

func (u *User) useGroupKick(msg string) {
	parts := strings.SplitN(msg, "|", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：gk|群ID|用户名\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	target := strings.TrimSpace(parts[2])
	if err := u.Server.groupManager.Kick(groupID, u.Name, target); err != nil {
		u.SendMsg("踢人失败: " + err.Error())
		return
	}
	u.SendMsg("踢人成功: " + target)
}

func (u *User) useGroupGrantAdmin(msg string) {
	parts := strings.SplitN(msg, "|", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：ga|群ID|用户名\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	target := strings.TrimSpace(parts[2])
	if err := u.Server.groupManager.GrantAdmin(groupID, u.Name, target); err != nil {
		u.SendMsg("设管理员失败: " + err.Error())
		return
	}
	u.SendMsg("设管理员成功: " + target)
}

func (u *User) useGroupRevokeAdmin(msg string) {
	parts := strings.SplitN(msg, "|", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：gr|群ID|用户名\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	target := strings.TrimSpace(parts[2])
	if err := u.Server.groupManager.RevokeAdmin(groupID, u.Name, target); err != nil {
		u.SendMsg("撤管理员失败: " + err.Error())
		return
	}
	u.SendMsg("撤管理员成功: " + target)
}

func (u *User) useGroupMembers(msg string) {
	parts := strings.SplitN(msg, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：gm|群ID\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	members := u.Server.groupManager.Members(groupID)
	if len(members) == 0 {
		u.SendMsg("群不存在或无成员: " + groupID)
		return
	}
	u.SendMsg("群成员(" + groupID + "): " + strings.Join(members, ", "))
}

func (u *User) useGroupTalk(msg string) {
	parts := strings.SplitN(msg, "|", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：gt|群ID|消息\n")
		return
	}
	groupID := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])

	if _, ok := u.Server.groupManager.RoleOf(groupID, u.Name); !ok {
		u.SendMsg("发送失败: 你不在该群中。")
		return
	}
	members := u.Server.groupManager.Members(groupID)
	recipients := make([]string, 0, len(members))
	for _, member := range members {
		if member == u.Name {
			continue
		}
		recipients = append(recipients, member)
	}

	clientID := fmt.Sprintf("gc-%d", time.Now().UnixNano())
	req := &Message{
		Type:        TypeSend,
		ClientMsgID: clientID,
		ChatID:      "group:" + groupID,
		From:        u.Name,
		Body:        body,
	}
	msgSaved, existing, err := u.Server.logic.ProcessSend(req, recipients)
	if err != nil {
		u.SendMsg("发送群消息失败: " + err.Error())
		return
	}
	if existing != nil {
		u.SendMsg("群消息幂等命中: " + existing.ServerMsgID)
		return
	}
	u.Server.EnqueueServerMsg(msgSaved.ServerMsgID)
	u.SendMsg("群消息已入队: " + msgSaved.ServerMsgID)
}
