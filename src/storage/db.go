package storage

import (
	"log"
	"os"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB 初始化数据库连接，仅支持 MySQL。优先读取环境变量 IM_DB_DRIVER 与 IM_DB_DSN，
// 若未提供 DSN 则使用默认容器连接字符串。
func InitDB(dsn string) error {
	driver := strings.TrimSpace(os.Getenv("IM_DB_DRIVER"))
	if driver == "" {
		driver = "mysql"
	}

	if driver != "mysql" {
		log.Printf("[Storage] IM_DB_DRIVER=%s not supported, fallback to mysql", driver)
		driver = "mysql"
	}

	if strings.TrimSpace(dsn) == "" {
		dsn = strings.TrimSpace(os.Getenv("IM_DB_DSN"))
	}
	if dsn == "" {
		dsn = "root:secret@tcp(mysql:3306)/goim?charset=utf8mb4&parseTime=True&loc=Local"
	}

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	log.Printf("[Storage] 已连接 MySQL 数据库: %s", dsn)
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
