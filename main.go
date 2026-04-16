package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"goim/internal/delivery"
	"goim/internal/server"
	"goim/internal/storage"
	"goim/internal/svcapi"
	"goim/internal/utils"
)

func main() {
	utils.LoadEnvFilesIfUnset(".env.local")

	dsn := strings.TrimSpace(os.Getenv("IM_DB_DSN"))
	if dsn == "" {
		dsn = "root:secret@tcp(127.0.0.1:3306)/goim?charset=utf8mb4&parseTime=True&loc=Local"
	}
	if err := storage.InitDB(dsn); err != nil {
		log.Fatalf("init db failed: %v", err)
	}

	maxRetry := 5
	if v := os.Getenv("IM_MAX_RETRY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRetry = n
		}
	}
	retryDelay := 2 * time.Second
	if v := os.Getenv("IM_RETRY_DELAY_SEC"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			retryDelay = time.Duration(sec) * time.Second
		}
	}

	s := server.NewServer("127.0.0.1", 8888)

	worker := delivery.NewWorker(s.Store(), maxRetry, retryDelay, func() map[string]svcapi.Sender {
		m := make(map[string]svcapi.Sender)
		s.MapLock.RLock()
		for name, info := range s.OnlineMap {
			m[name] = info.Sender
		}
		s.MapLock.RUnlock()
		return m
	})
	s.SetWorker(worker)

	s.Start()
}
