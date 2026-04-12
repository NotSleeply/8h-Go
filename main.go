package main

import (
	"log"
	"os"
	"strings"

	"tet/src/server"
	"tet/src/storage"
)

func main() {
	dsn := strings.TrimSpace(os.Getenv("IM_DB_DSN"))
	if dsn == "" {
		dsn = "gochat.sqlite3"
	}
	if err := storage.InitDB(dsn); err != nil {
		log.Fatalf("init sqlite failed: %v", err)
	}

	s := server.NewServer("127.0.0.1", 8888)
	s.Start()
}
