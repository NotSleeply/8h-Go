package dao

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"tet/src/cache"
	"tet/src/storage"

	"gorm.io/gorm"
)

// CreateUser 保存新用户到数据库
func CreateUser(user *storage.User) error {
	if user == nil {
		return errors.New("user 不存在")
	}
	if storage.DB == nil {
		return errors.New("数据库未初始化")
	}
	if err := storage.DB.Create(user).Error; err != nil {
		return err
	}
	// set cache
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		if b, err := json.Marshal(user); err == nil {
			_ = c.Set(ctx, cache.UserKey(user.UserName), b, 30*time.Minute).Err()
		}
	}
	return nil
}

// GetUserByID 根据数字 ID 查询用户
func GetUserByID(id uint) (*storage.User, error) {
	if storage.DB == nil {
		return nil, errors.New("数据库未初始化")
	}
	var u storage.User
	if err := storage.DB.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByName 根据用户名查询用户
func GetUserByName(name string) (*storage.User, error) {
	if storage.DB == nil {
		return nil, errors.New("数据库未初始化")
	}
	// try redis cache first
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		if s, err := c.Get(ctx, cache.UserKey(name)).Result(); err == nil && s != "" {
			var u storage.User
			if err := json.Unmarshal([]byte(s), &u); err == nil {
				return &u, nil
			}
		}
	}

	var u storage.User
	if err := storage.DB.Where("user_name = ?", name).First(&u).Error; err != nil {
		return nil, err
	}
	// set cache
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		if b, err := json.Marshal(&u); err == nil {
			_ = c.Set(ctx, cache.UserKey(name), b, 30*time.Minute).Err()
		}
	}
	return &u, nil
}

// UpdateUserStatus 更新用户在线状态。参数 userID 支持数字 ID 或用户名字符串两种形式。
func UpdateUserStatus(userID string, status int8) error {
	if storage.DB == nil {
		return errors.New("数据库未初始化")
	}
	if userID == "" {
		return errors.New("userID 为空")
	}
	// 尝试作为数字 ID 解析
	if id, err := strconv.ParseUint(userID, 10, 64); err == nil {
		res := storage.DB.Model(&storage.User{}).Where("id = ?", uint(id)).Update("status", status)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		// invalidate cache if possible
		if c := cache.Client(); c != nil {
			ctx := context.Background()
			if u, err := GetUserByID(uint(id)); err == nil {
				_ = c.Del(ctx, cache.UserKey(u.UserName)).Err()
			}
		}
		return nil
	}
	// 否则按用户名更新
	res := storage.DB.Model(&storage.User{}).Where("user_name = ?", userID).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	// invalidate cache for username
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		_ = c.Del(ctx, cache.UserKey(userID)).Err()
	}
	return nil
}
