package user

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"tet/src/dao"
	"tet/src/session"
	"tet/src/storage"
)

// Register 注册新用户并将会话上线
func Register(s *session.User, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || strings.ContainsAny(username, " \t\n\r|") {
		return errors.New("用户名不可为空或包含空格/管道符")
	}
	// 检查是否在线
	if s.Server != nil {
		online := s.Server.ListOnlineUsers()
		for _, info := range online {
			if info.Name == username {
				return errors.New("该用户名已有人在线")
			}
		}
	}
	// 检查是否存在于数据库
	if _, err := dao.GetUserByName(username); err == nil {
		return errors.New("用户已存在，请直接登录")
	}
	// 密码加密
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}
	su := &storage.User{UserName: username, PasswordHash: string(hash)}
	if err := dao.CreateUser(su); err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	// 设置会话并上线
	s.Name = username
	s.Authenticated = true
	s.Online()
	return nil
}

// Login 校验密码并上线
func Login(s *session.User, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("用户名不能为空")
	}
	su, err := dao.GetUserByName(username)
	if err != nil {
		return errors.New("用户不存在，请先注册")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(su.PasswordHash), []byte(password)); err != nil {
		return errors.New("密码错误")
	}
	// 检查是否已有该用户名在线
	if s.Server != nil {
		online := s.Server.ListOnlineUsers()
		for _, info := range online {
			if info.Name == username {
				return errors.New("该用户已在其他连接登录")
			}
		}
	}
	s.Name = username
	s.Authenticated = true
	s.Online()
	_ = dao.UpdateUserStatus(username, 1)
	return nil
}
