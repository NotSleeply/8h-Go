# Issue38 交付记录: Redis + Kafka 双引擎消息中间件

## 1. 问题描述

Issue #38 目标是将消息投递队列从单一进程内通道升级为可扩展的消息中间件模式，支持 Redis/Kafka，降低单点内存队列风险，并为后续多实例部署做准备。

## 2. 技术难点

- 需要在不破坏现有 TCP 投递逻辑的前提下平滑接入 MQ
- Redis/Kafka 在本地开发环境可能未启动，必须可自动降级
- 需要保持 at-least-once 投递语义，避免因中间件异常导致消息丢失

## 3. 解决方案

### 3.1 新增 MessageBus 抽象

新增 `src/server/mq.go`，实现统一发布/消费接口，支持 4 种模式：

- `local`：仅使用本地 `DeliverQueue`
- `redis`：使用 Redis List (`LPUSH` + `BRPOP`)
- `kafka`：使用 Kafka topic（writer + group reader）
- `dual`：同时发布到 Redis 和 Kafka（任一成功即视为发布成功）

通过环境变量控制：

- `IM_MQ_MODE` (`local|redis|kafka|dual`)
- `IM_REDIS_ADDR`、`IM_REDIS_LIST_KEY`
- `IM_KAFKA_BROKERS`、`IM_KAFKA_TOPIC`、`IM_KAFKA_GROUP_ID`

### 3.2 与投递链路集成

- `Server` 新增 `bus *MessageBus`
- 原 `EnqueueServerMsg` 改为优先 `bus.Publish`，失败自动 fallback 到本地队列
- 启动时 `Start()` 自动启动 bus 消费协程，并将消费到的 `server_msg_id` 推入 `DeliverQueue`
- 退出时自动关闭 bus 资源

### 3.3 可观测性

- `stats` 指标新增 `mq_mode`
- 可在线确认当前实例中间件工作模式

## 4. 代码变更

- 新增: `src/server/mq.go`
- 修改: `src/server/server.go`（接入 bus 发布/消费）
- 修改: `src/server/command.go`（stats 增加 mq_mode）
- 依赖更新: `go.mod`, `go.sum`
  - `github.com/redis/go-redis/v9`
  - `github.com/segmentio/kafka-go`

## 5. 验证结果

- `go test ./...` 通过
- `go build .` 通过
- 默认不配置环境变量时使用 `local` 模式，不影响现有功能
- 配置 Redis/Kafka 后可将消息 ID 通过对应中间件投递并消费回本地投递队列

## 6. 当前限制与后续优化

- 当前 dual 模式存在“重复消费”可能，但现有投递状态去重可容忍（at-least-once）
- 未实现统一死信队列（DLQ）和重试策略分级，可在后续版本补齐
- 生产环境建议增加 MQ 链路监控与告警（consumer lag、publish fail rate）
