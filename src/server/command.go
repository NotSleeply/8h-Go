package server

import (
	"strings"
)

// 查询在线用户
func (u *User) useWho() {
	u.Server.MapLock.Lock()
	for _, user := range u.Server.OnlineMap {
		msg := "[" + user.Addr + "]" + user.Name + ":" + "在线中…" + "\n"
		u.SendMsg(msg)
	}
	u.Server.MapLock.Unlock()
}

// 更改姓名
func (u *User) useRename(msg string) {
	parts := strings.Split(msg, "|")
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		u.SendMsg("命令格式错误，正确用法：rename|新昵称，例如：rename|alice\n")
		return
	}

	newName := strings.TrimSpace(parts[1])
	if _, ok := u.Server.OnlineMap[newName]; ok {
		u.SendMsg("该昵称已被占用，请选择其他昵称。例如：rename|bob\n")
		return
	}

	oldName := u.Name
	u.Server.MapLock.Lock()
	delete(u.Server.OnlineMap, u.Name)
	u.Server.OnlineMap[newName] = u
	u.Server.MapLock.Unlock()

	u.Name = newName
	newMsg := "已将 " + oldName + " 更改为: " + newName + "\n"
	u.SendMsg(newMsg)
}

// 私聊
func (u *User) useChat(msg string) {
	parts := strings.Split(msg, "|")
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		u.SendMsg("命令格式错误，正确用法：to|用户名|消息，例如：to|bob|你好\n")
		return
	}

	toName := strings.TrimSpace(parts[1])
	toMsg := strings.TrimSpace(parts[2])
	toUser, ok := u.Server.OnlineMap[toName]
	if !ok {
		u.SendMsg("发送对象不存在，请检查用户名后重试。使用 who 查看在线用户。\n")
		return
	}

	sendText := u.Name + " --> " + toMsg + "\n"
	toUser.SendMsg(sendText)
}

// 退出聊天室
func (u *User) useExit(msg string) {
	u.SendMsg("再见！欢迎下次再来~\n")
	u.Logout()
}
