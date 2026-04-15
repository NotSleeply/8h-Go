package main

import (
	"log"
	"os"
	"strings"

	"tet/src/server"
	"tet/src/storage"
	"tet/src/utils"
)

func main() {
	utils.LoadEnvFilesIfUnset(".env.local", ".env")

	dsn := strings.TrimSpace(os.Getenv("IM_DB_DSN"))
	if dsn == "" {
		dsn = "root:secret@tcp(127.0.0.1:3306)/goim?charset=utf8mb4&parseTime=True&loc=Local"
	}
	if err := storage.InitDB(dsn); err != nil {
		log.Fatalf("init db failed: %v", err)
	}

	s := server.NewServer("127.0.0.1", 8888)
	s.Start()
}
