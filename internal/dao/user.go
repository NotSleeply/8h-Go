package dao

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"goim/internal/cache"
	"goim/internal/storage"

	"gorm.io/gorm"
)

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
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		if b, err := json.Marshal(user); err == nil {
			_ = c.Set(ctx, cache.UserKey(user.UserName), b, 30*time.Minute).Err()
		}
	}
	return nil
}

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

func GetUserByName(name string) (*storage.User, error) {
	if storage.DB == nil {
		return nil, errors.New("数据库未初始化")
	}
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
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		if b, err := json.Marshal(&u); err == nil {
			_ = c.Set(ctx, cache.UserKey(name), b, 30*time.Minute).Err()
		}
	}
	return &u, nil
}

func UpdateUserStatus(userID string, status int8) error {
	if storage.DB == nil {
		return errors.New("数据库未初始化")
	}
	if userID == "" {
		return errors.New("userID 为空")
	}
	if id, err := strconv.ParseUint(userID, 10, 64); err == nil {
		res := storage.DB.Model(&storage.User{}).Where("id = ?", uint(id)).Update("status", status)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if c := cache.Client(); c != nil {
			ctx := context.Background()
			if u, err := GetUserByID(uint(id)); err == nil {
				_ = c.Del(ctx, cache.UserKey(u.UserName)).Err()
			}
		}
		return nil
	}
	res := storage.DB.Model(&storage.User{}).Where("user_name = ?", userID).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	if c := cache.Client(); c != nil {
		ctx := context.Background()
		_ = c.Del(ctx, cache.UserKey(userID)).Err()
	}
	return nil
}
