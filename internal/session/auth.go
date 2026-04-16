package session

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"goim/internal/dao"
	"goim/internal/storage"
)

func Register(s *User, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || strings.ContainsAny(username, " \t\n\r|") {
		return errors.New("用户名不可为空或包含空格/管道符")
	}
	if s.Server != nil {
		for _, info := range s.Server.ListOnlineUsers() {
			if info.Name == username {
				return errors.New("该用户名已有人在线")
			}
		}
	}
	if _, err := dao.GetUserByName(username); err == nil {
		return errors.New("用户已存在，请直接登录")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}
	if err := dao.CreateUser(&storage.User{UserName: username, PasswordHash: string(hash)}); err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	s.Name = username
	s.Authenticated = true
	s.Online()
	return nil
}

func Login(s *User, username, password string) error {
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
	if s.Server != nil {
		for _, info := range s.Server.ListOnlineUsers() {
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
