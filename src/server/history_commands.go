package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
