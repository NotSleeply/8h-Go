package storage

import (
	"log"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB 初始化数据库连接。支持通过环境变量 IM_DB_DRIVER 指定驱动（mysql 或 sqlite），
// 如果未指定则会基于传入的 DSN 做自动检测。
func InitDB(dsn string) error {
	driver := strings.TrimSpace(os.Getenv("IM_DB_DRIVER"))
	if driver == "" {
		// 简单检测 DSN 中是否像 MySQL 的形式
		if strings.Contains(dsn, "@tcp(") || strings.Contains(dsn, "mysql") || strings.Contains(dsn, ":3306") {
			driver = "mysql"
		} else {
			driver = "sqlite"
		}
	}

	var err error
	switch driver {
	case "mysql":
		DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			return err
		}
		log.Printf("[Storage] 已连接 MySQL 数据库: %s", dsn)
	default:
		DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		if err != nil {
			return err
		}
		log.Printf("[Storage] 已连接 SQLite 数据库: %s", dsn)
	}

	return AutoMigrate()
}

func AutoMigrate() error {
	return DB.AutoMigrate(
		&User{},
		&Message{},
		&MessageRecipient{},
		&Room{},
		&GroupMember{},
	)
}
