package cache

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	client *redis.Client
	once   sync.Once
)

func initClient() {
	addr := strings.TrimSpace(os.Getenv("IM_REDIS_ADDR"))
	if addr == "" {
		// Redis not configured; treat cache as disabled.
		return
	}
	c := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis connection failed (%s): %v", addr, err)
	}
	client = c
}

// Client 返回已初始化的 Redis 客户端，若不可用则返回 nil。
func Client() *redis.Client {
	once.Do(initClient)
	return client
}

// UserKey 生成按用户名缓存用户的 redis key
func UserKey(username string) string {
	return "user:byname:" + username
}

// OnlineKey 返回用于标记用户在线状态的 key
func OnlineKey(username string) string {
	return "user:online:" + username
}

// GatewayID 返回用于写入在线键的网关标识，优先使用 IM_GATEWAY_ID，回退到主机名
func GatewayID() string {
	id := strings.TrimSpace(os.Getenv("IM_GATEWAY_ID"))
	if id != "" {
		return id
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}

// OnlineTTL 返回在线键的 TTL，单位为 time.Duration，默认 60 秒
func OnlineTTL() time.Duration {
	v := strings.TrimSpace(os.Getenv("IM_USER_ONLINE_TTL_SEC"))
	if v == "" {
		return 60 * time.Second
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 60 * time.Second
}

// RecipientsKey 返回某条 serverMsgID 的接收者缓存 key
func RecipientsKey(serverMsgID string) string {
	return "deliver:recips:" + serverMsgID
}

// PendingUserKey 返回某个用户的待投递 serverMsgID 列表 key
func PendingUserKey(username string) string {
	return "deliver:pending:user:" + username
}

// PendingDueKey 返回用于缓存到期/重试候选的 key（短期）
func PendingDueKey() string {
	return "deliver:pending:due"
}

// PendingRecoverKey 返回用于缓存 RecoverPendingServerMsgIDs 的 key
func PendingRecoverKey() string {
	return "deliver:pending:recover"
}
