package main

import (
	"log"
	"net"
	"os"
	"strings"

	"google.golang.org/grpc"

	"tet/src/rpc"
	"tet/src/rpc/pb"
	"tet/src/server"
	"tet/src/storage"
)

func main() {
	dsn := strings.TrimSpace(os.Getenv("IM_DB_DSN"))
	if dsn == "" {
		dsn = "root:secret@tcp(mysql:3306)/goim?charset=utf8mb4&parseTime=True&loc=Local"
	}
	if err := storage.InitDB(dsn); err != nil {
		log.Fatalf("init db failed: %v", err)
	}

	addr := strings.TrimSpace(os.Getenv("IM_GRPC_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:50051"
	}

	s := server.NewServer("127.0.0.1", 8888)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen grpc failed: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterLogicServiceServer(grpcServer, rpc.NewLogicGRPCServer(s))
	log.Printf("[gRPC] listening on %s", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve grpc failed: %v", err)
	}
}
