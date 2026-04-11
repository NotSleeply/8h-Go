## 📌 模块概述
**目标**: 引入 **Redis + Kafka 双引擎** 构建现代化的IM消息中间件层，对应文章存储层的缓存和消息队列需求。

> 🚀 **现代化升级**: 除了文章提到的Redis缓存外，我们还引入 **Apache Kafka** 作为分布式消息队列，实现：
> - **削峰填谷**: 高并发场景下的流量控制
> - **异步解耦**: Logic/Gate/Task层完全解耦
> - **消息持久化**: 确保消息不丢失
> - **离线消息队列**: 用户离线时消息暂存Kafka

## ✨ 实现效果
- [ ] **Redis集群**:
  - [ ] 用户在线状态存储（Key: `user:online:{uid}`）
  - [ ] 用户路由信息（Key: `user:route:{uid}` → Gate节点地址）
  - [ ] 会话Session管理（替代内存存储）
  - [ ] 消息序号生成器（INCR原子操作）
  - [ ] 频率限制计数器（防刷机制）
- [ ] **Kafka集群**:
  - [ ] 消息投递Topic (`im-message-deliver`) - 异步推送消息
  - [ ] 离线消息Topic (`im-offline-message`) - 存储离线消息
  - [ ] 消息确认Topic (`im-message-ack`) - 处理客户端Ack
  - [ ] 事件流Topic (`im-events`) - 用户上下线、群组变更等事件

## 🏗️ 架构定位
```
┌─────────────────────────────────────────────┐
│              Gate 接入层                      │
└──────────────┬──────────────────────────────┘
               │
    ┌──────────▼──────────┐
    │     Kafka Producer   │ ← 生产者：Gate发送消息到Kafka
    │  ┌────────────────┐  │
    │  │ im-message-     │  │
    │  │ deliver Topic   │  │
    │  └───────┬────────┘  │
    └──────────┼──────────┘
               │
    ┌──────────▼──────────┐
    │   Kafka Consumer    │ ← 消费者：Logic/Task处理消息
    │  (Logic/Task Layer)  │
    └──────────┬──────────┘
               │
    ┌──────────▼──────────┐
    │      Redis Cluster   │ ← 缓存层：状态/路由/限流
    │  ┌───────┬────────┐  │
    │  │用户状态 │ 路由表  │  │
    │  │Session │ 序号器  │  │
    │  └───────┴────────┘  │
    └─────────────────────┘
```

## 📋 实现步骤

### Step 1: 安装依赖
```bash
# Redis客户端
go get github.com/redis/go-redis/v9

# Kafka客户端（使用sarama库）
go get github.com/IBM/sarama
```

### Step 2: 实现 Redis 服务封装
新建文件: `cache/redis.go`

```go
package cache

import (
    "context"
    "github.com/redis/go-redis/v9"
    "time"
)

var RDB *redis.Client

func InitRedis(addr, password string, db int) error {
    RDB = redis.NewClient(&redis.Options{
        Addr:     addr,
        Password: password,
        DB:       db,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := RDB.Ping(ctx).Err(); err != nil {
        return err
    }

    log.Printf("[Cache] Redis connected: %s", addr)
    return nil
}

// UserService 用户相关的Redis操作
type UserService struct{}

// SetUserOnline 设置用户在线状态和路由信息
func (s *UserService) SetUserOnline(ctx context.Context, userID, gateAddr string) error {
    pipe := RDB.Pipeline()
    pipe.Set(ctx, fmt.Sprintf("user:online:%s", userID), gateAddr, 24*time.Hour)
    pipe.Set(ctx, fmt.Sprintf("user:route:%s", userID), gateAddr, 24*time.Hour)
    _, err = pipe.Exec(ctx)
    return err
}

// GetUserRoute 获取用户所在Gate节点地址（用于消息路由）
func (s *UserService) GetUserRoute(ctx context.Context, userID string) (string, error) {
    return RDB.Get(ctx, fmt.Sprintf("user:route:%s", userID)).Result()
}

// IsUserOnline 检查用户是否在线
func (s *UserService) IsUserOnline(ctx context.Context, userID string) (bool, error) {
    result, err := RDB.Exists(ctx, fmt.Sprintf("user:online:%s", userID)).Result()
    return result > 0, err
}

// GenerateSeq 生成全局递增的消息序号（原子操作）
func (s *UserService) GenerateSeq(ctx context.Context, chatID string) (int64, error) {
    return RDB.Incr(ctx, fmt.Sprintf("msg:seq:%s", chatID)).Result()
}

// RateLimitCheck 频率限制检查（滑动窗口算法）
func (s *UserService) RateLimitCheck(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
    current, err := RDB.Incr(ctx, key).Result()
    if err != nil {
        return false, err
    }
    if current == 1 {
        RDB.Expire(ctx, key, window)
    }
    return current <= limit, nil
}
```

### Step 3: 实现 Kafka 生产者和消费者
新建文件: `messaging/kafka_producer.go`

```go
package messaging

import (
    "github.com/IBM/sarama"
    "log"
    "encoding/json"
)

// KafkaProducer Kafka生产者封装
type KafkaProducer struct {
    producer sarama.SyncProducer
}

func NewKafkaProducer(brokers []string) (*KafkaProducer, error) {
    config := sarama.NewConfig()
    config.Producer.RequiredAcks = sarama.WaitForAll // 等待所有副本确认
    config.Producer.Retry.Max = 5                    // 最大重试5次
    config.Producer.Return.Successes = true

    producer, err := sarama.NewSyncProducer(brokers, config)
    if err != nil {
        return nil, err
    }

    log.Printf("[Messaging] Kafka producer connected to %v", brokers)
    return &KafkaProducer{producer: producer}, nil
}

// SendMessage 发送消息到指定Topic
func (p *KafkaProducer) SendMessage(topic string, key string, value interface{}) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }

    msg := &sarama.ProducerMessage{
        Topic: topic,
        Key:   sarama.StringEncoder(key),
        Value: sarama.ByteEncoder(data),
    }

    _, offset, err := p.producer.SendMessage(msg)
    if err != nil {
        return err
    }

    log.Printf("[Messaging] Message sent to %s [%d] offset=%d", topic, key, offset)
    return nil
}

// DeliverMessage 投递消息给接收者（写入im-message-deliver Topic）
func (p *KafkaProducer) DeliverMessage(msg *DeliverMessage) error {
    return p.SendMessage("im-message-deliver", msg.RecvID, msg)
}
```

新建文件: `messaging/kafka_consumer.go`

```go
package messaging

import (
    "github.com/IBM/sarama"
    "log"
    "context"
)

// KafkaConsumer Kafka消费者封装
type KafkaConsumer struct {
    consumerGroup sarama.ConsumerGroup
    handler       MessageHandler
}

type MessageHandler interface {
    HandleDeliverMessage(ctx context.Context, msg *DeliverMessage) error
    HandleOfflineMessage(ctx context.Context, msg *OfflineMessage) error
    HandleMessageAck(ctx context.Context, ack *MessageAck) error
}

func NewKafkaConsumer(brokers []string, groupID string, handler MessageHandler) (*KafkaConsumer, error) {
    config := sarama.NewConfig()
    config.Version = sarama.V2_8_0_0
    config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
    config.Consumer.Offsets.Initial = sarama.OffsetNewest

    group, err := sarama.NewConsumerGroup(brokers, groupID, config)
    if err != nil {
        return nil, err
    }

    log.Printf("[Messaging] Kafka consumer group '%s' connected", groupID)
    return &KafkaConsumer{
        consumerGroup: group,
        handler:       handler,
    }, nil
}

// StartConsuming 启动消费循环
func (c *KafkaConsumer) StartConsuming(ctx context.Context) {
    topics := []string{
        "im-message-deliver",
        "im-offline-message",
        "im-message-ack",
    }

    for {
        select {
        case <-ctx.Done():
            return
        default:
            if err := c.consumerGroup.Consume(ctx, topics, c); err != nil {
                log.Printf("[Messaging] Consumer error: %v", err)
            }
        }
    }
}

// Setup 消费组初始化
func (c *KafkaConsumer) Setup(sarama.ConsumerGroupSession) error { return nil }

// Cleanup 消费组清理
func (c *KafkaConsumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim 消息处理主逻辑
func (c *KafkaConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    for msg := range claim.Messages() {
        switch msg.Topic {
        case "im-message-deliver":
            var deliverMsg DeliverMessage
            json.Unmarshal(msg.Value, &deliverMsg)
            c.handler.HandleDeliverMessage(session.Context(), &deliverMsg)

        case "im-offline-message":
            var offlineMsg OfflineMessage
            json.Unmarshal(msg.Value, &offlineMsg)
            c.handler.HandleOfflineMessage(session.Context(), &offlineMsg)

        case "im-message-ack":
            var ack MessageAck
            json.Unmarshal(msg.Value, &ack)
            c.handler.HandleMessageAck(session.Context(), &ack)
        }
        session.MarkMessage(msg, "")
    }
    return nil
}
```

### Step 4: 定义消息结构体
新建文件: `messaging/types.go`

```go
package messaging

// DeliverMessage 消息投递结构（Logic → Gate）
type DeliverMessage struct {
    ServerMsgID uint64                 `json:"server_msg_id"`
    ChatID      string                 `json:"chat_id"`
    From        string                 `json:"from"`
    To          string                 `json:"to"`
    ContentType int                    `json:"content_type"`
    Content     string                 `json:"content"`
    Seq         uint64                 `json:"seq"`
    Timestamp   int64                  `json:"timestamp"`
}

// OfflineMessage 离线消息结构
type OfflineMessage struct {
    RecvID      string `json:"recv_id"`
    ServerMsgID uint64 `json:"server_msg_id"`
    Content     []byte `json:"content"`
    ExpireAt    int64  `json:"expire_at"` // 过期时间戳
}

// MessageAck 消息确认结构
type MessageAck struct {
    UserID      string `json:"user_id"`
    ServerMsgID uint64 `json:"server_msg_id"`
    Status      int    `json:"status"` // 1-已送达 2-已读
}
```

### Step 5: 在 main.go 中初始化
```go
func main() {
    // 初始化Redis
    cache.InitRedis("localhost:6379", "", 0)

    // 初始化Kafka生产者（供Logic层使用）
    kafkaProducer, _ := messaging.NewKafkaProducer([]string{"localhost:9092"})

    // 初始化Kafka消费者（Task层）
    kafkaConsumer, _ := messaging.NewKafkaConsumer(
        []string{"localhost:9092"},
        "im-task-group",
        taskHandler, // Task层的消息处理器
    )
    go kafkaConsumer.StartConsuming(context.Background())

    log.Println("[System] Redis + Kafka initialized")
}
```

### Step 6: 编写单元测试 + Docker Compose配置
新建文件: `docker/kafka-redis.yml`（用于本地开发）

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data

  zookeeper:
    image: confluentinc/cp-zookeeper:7.3.0
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181

  kafka:
    image: confluentinc/cp-kafka:7.3.0
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    depends_on:
      - zookeeper

volumes:
  redis-data:
```

## 🎯 参考资源
- **📄 架构参考文章**: [《一个海量在线用户即时通讯系统（IM）的完整设计Plus》](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)
  - 第1.1.5节：存储层（缓存+消息数据）
  - 第4节：TimeLine模型和离线消息拉取
- **Redis文档**: https://redis.io/docs/
- **Kafka文档**: https://kafka.apache.org/documentation/
- **Sarama库**: https://github.com/IBM/sarama

## 🔍 验收标准
1. ✅ 可以成功连接Redis并执行SET/GET操作
2. ✅ 用户上线后可以在Redis中查询到路由信息
3. ✅ 可以通过Kafka成功发送和消费消息
4. ✅ 消息投递延迟 < 100ms（本地环境）
5. ✅ Kafka消费者能正确处理不同Topic的消息
6. ✅ 单元测试全部通过 (`go test ./cache/... ./messaging/...`)

## ⚠️ 注意事项
- ⚠️ **可靠性**: Kafka应配置 `RequiredAcks=All` 确保消息不丢失
- ⚠️ **性能**: Redis连接池大小应根据并发量调整
- ⚠️ **监控**: 应接入Prometheus监控Redis/Kafka指标
- ⚠️ **容灾**: 生产环境建议部署3节点Kafka集群 + Redis Sentinel

## 📊 工作量评估
- 预计耗时: 4-5天
- 复杂度: ⭐⭐⭐⭐⭐ (核心基础设施 - 影响全系统性能)
- 依赖: 无强依赖（但建议在Issue #6之后做）

---
**所属阶段**: 第3周 - 现代化技术栈（Redis + Kafka双引擎）
**优先级**: P0 (关键基础设施 - 性能和可靠性的基石)
