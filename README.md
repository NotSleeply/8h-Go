# 8h-GoIM

一个基于 Go 的即时通讯（IM）后端学习项目，包含：

- TCP 长连接聊天室
- 消息持久化（MySQL + GORM）
- 离线投递与重试
- 群聊与历史消息
- 可选 Redis/Kafka 消息总线
- gRPC 逻辑服务接口

## 使用方法

### 1. 前置依赖

- Go `1.25+`
- Docker Desktop（用于 MySQL/Redis/Kafka/Zookeeper）

### 2. 启动依赖服务（Docker）

本项目的 `docker-compose.yml` 已写死所需端口和配置，不依赖 `.env`。

```bash
docker compose up -d
docker compose ps
```

默认映射端口：

- MySQL: `3306`
- Redis: `6379`
- Kafka: `9092`
- Zookeeper: `2181`

停止依赖服务：

```bash
docker compose down
```

也可使用脚本：

- Linux/macOS: `sh scripts/start.sh` / `sh scripts/stop.sh`
- Windows PowerShell: `.\scripts\start.ps1` / `.\scripts\stop.ps1`

### 3. 启动 TCP IM 服务

```bash
go run .
```

默认监听：`127.0.0.1:8888`

### 4. 启动 gRPC 逻辑服务（可选）

```bash
go run ./cmd/grpcserver
```

默认监听：`127.0.0.1:50051`

### 5. 连接测试

可使用 `nc` 或 `telnet` 连接 TCP 服务：

```bash
nc 127.0.0.1 8888
```

输入 `help` 查看可用命令。

## 常用命令

客户端文本命令（TCP）：

- `help` 查看帮助
- `who` 查看在线用户
- `rename|新昵称` 修改昵称
- `to|用户名|消息` 私聊
- `gc|群ID` 创建群
- `gj|群ID` 加入群
- `gl|群ID` 退出群
- `gd|群ID` 解散群（群主）
- `gk|群ID|用户名` 踢人
- `ga|群ID|用户名` 设管理员
- `gr|群ID|用户名` 撤管理员
- `gm|群ID` 查看群成员
- `gt|群ID|消息` 群聊
- `history|用户名|条数` 查看私聊历史
- `read|server_msg_id` 上报已读
- `stats` 查看运行指标
- `exit` 退出

## 技术介绍

### 技术栈

- 语言：Go
- 网络：TCP 长连接、gRPC
- 存储：MySQL + GORM
- 消息队列：本地队列 / Redis / Kafka（可配置）
- 协议：行分隔 JSON 协议 + Protobuf(gRPC)

### 协议模型（TCP JSON）

核心消息结构位于 `src/protocol/protocol.go`，关键字段：

- `type`：`send/send_ack/deliver/deliver_ack/read_ack/sync`
- `client_msg_id`：客户端去重 ID
- `server_msg_id`：服务端全局消息 ID
- `chat_id/from/to/seq/body/ts/status`

### gRPC 接口

定义在 `proto/im.proto`：

- `SendMessage`
- `AckDelivery`
- `AckRead`

生成脚本：`scripts/gen_proto.ps1`

## 结构设计

### 总体架构

1. 客户端通过 TCP 连接接入 `Server`
2. 消息进入 `logic` 层做幂等、分配序号、生成服务端消息 ID
3. `store` 层写入持久化（MySQL）并维护投递状态
4. `deliver worker` 消费投递队列，向在线用户推送 `deliver`
5. 客户端回 `deliver_ack/read_ack`，服务端更新投递状态
6. 离线用户由重试与补投递流程处理

### 模块分层

- `main.go`：TCP 服务入口
- `cmd/grpcserver/main.go`：gRPC 服务入口
- `src/server`：连接管理、命令分发、投递/重试、统计
- `src/session`：会话层抽象（基于 `iface`）
- `src/logic`：核心业务逻辑（幂等、Seq、消息入库）
- `src/store`：存储实现（内存 + MySQL 混合）
- `src/storage`：GORM 模型与 DB 初始化
- `src/mq`：Redis/Kafka/Local 消息总线
- `src/group`：群组管理
- `src/rpc` + `src/rpc/pb`：gRPC 服务与代码生成产物
- `src/protocol`：协议结构定义
- `src/iface`：接口契约
- `src/utils`：工具函数（如 `.env.local` 加载）

### 数据模型（MySQL）

`src/storage/model.go` 中定义：

- `users`：用户信息
- `messages`：消息主表
- `message_recipients`：每个接收者的投递状态
- `rooms`：群信息
- `group_members`：群成员关系

启动时 `AutoMigrate` 自动建表/迁移。

## 关键配置

应用优先读取系统环境变量，其次读取 `.env.local`（若存在），最后使用代码默认值。

常用配置：

- `IM_DB_DSN`：MySQL DSN（默认 `127.0.0.1:3306`）
- `IM_MQ_MODE`：`local` / `redis` / `kafka` / `dual`
- `IM_REDIS_ADDR`：默认 `127.0.0.1:6379`
- `IM_KAFKA_BROKERS`：如 `127.0.0.1:9092`
- `IM_KAFKA_TOPIC`：默认 `im-deliver`
- `IM_KAFKA_GROUP_ID`：默认 `im-gate`
- `IM_CONN_RATE_WINDOW_SEC` / `IM_CONN_RATE_LIMIT`：连接限流
- `IM_DELIVER_MAX_RETRY` / `IM_DELIVER_RETRY_BASE_SEC`：投递重试策略
- `IM_GRPC_ADDR`：gRPC 监听地址（默认 `127.0.0.1:50051`）

可参考 `/.env.local` 示例值（本地开发）。

## 常见问题

### 1) `lookup mysql: no such host`

你在主机上运行 `go run .` 时，MySQL 应用 `127.0.0.1:3306`，而不是容器内 DNS 名 `mysql:3306`。

### 2) 启动时报外键/唯一索引迁移错误

历史数据库结构或旧迁移残留可能导致 GORM 迁移冲突。建议先确认当前表结构与模型一致，再决定是否清理旧表或手动修正索引。

### 3) Kafka/Redis 不可用时消息是否丢失

当 MQ 不可用时，`mq` 层会回退到本地发布逻辑，避免主流程完全中断。

## 开发与验证

```bash
go test ./...
docker compose config
```

## 致敬

- [8小时转go](https://www.bilibili.com/video/BV1gf4y1r79E)
- [LockGit/gochat](https://github.com/LockGit/gochat)
- [一个海量在线用户即时通讯系统（IM）的完整设计Plus](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)
