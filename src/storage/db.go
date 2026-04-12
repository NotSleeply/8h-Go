package storage

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(dsn string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	log.Printf("[Storage] 已连接 SQLite 数据库: %s", dsn)
	return AutoMigrate()
}

func AutoMigrate() error {
	return DB.AutoMigrate(
		&User{},
		&Message{},
		&Room{},
		&GroupMember{},
	)
}
