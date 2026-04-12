package dao

import (
	"errors"
	"strconv"

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
	return storage.DB.Create(user).Error
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
	var u storage.User
	if err := storage.DB.Where("user_name = ?", name).First(&u).Error; err != nil {
		return nil, err
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
	return nil
}
