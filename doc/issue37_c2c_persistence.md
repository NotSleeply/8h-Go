# Issue37 交付记录: 单聊 C2C（消息存储 + 幂等性 + 离线投递 + 回执）

## 1. 问题描述

Issue #37 目标是把单聊 C2C 从“仅在线内存转发”升级为“可持久化、可恢复、可回执、可查历史”的实现。  
改造前的主要问题是：消息/投递状态依赖进程内存，服务重启后离线待投递消息无法恢复，且缺少已读状态与历史查询闭环。

## 2. 技术难点

- 如何保证 `Message + MessageRecipient` 写入的一致性（事务边界）
- 如何在不破坏现有 TCP 协议链路的前提下增加离线重投递
- 如何把送达/已读状态落库并提供查询能力
- Windows 环境下 `sqlite3(cgo)` 编译不稳定，影响全量构建

## 3. 解决方案

### 3.1 存储模型升级

- `storage.Message` 的 `ServerMsgID` 统一改为 `string`
- `ClientMsgID` 幂等索引改为复合唯一键 `(send_id, client_msg_id)`
- 新增 `storage.MessageRecipient` 表，记录:
  - `server_msg_id`, `to_user`, `status(pending/delivered/read)`
  - `retry_count`, `next_retry_at`, `last_error`, `last_send_at`, `acked_at`, `read_at`

### 3.2 SQLite 驱动切换（避免 cgo）

- 从 `gorm.io/driver/sqlite` 切换为 `github.com/glebarez/sqlite`（pure-go）
- 解决当前环境下 cgo 构建失败问题，保证 `go test ./...` 可通过

### 3.3 C2C 事务写入与幂等

- `server/store.go` 升级为“内存缓存 + SQLite 持久化”的混合存储
- 新增 `SaveMessageWithRecipients(msg, recipients)`，在同一事务内写入:
  - `messages`
  - `message_recipients`
- 发送前按 `(from, client_msg_id)` 查重，命中直接返回幂等 `send_ack`

### 3.4 离线投递与恢复

- 发送私聊时，无论对端是否在线，都会持久化 pending recipient
- 用户上线时 `Online()` 自动触发 `EnqueuePendingForUser`，补投递离线消息
- 服务启动时 `RecoverPendingDeliveries()` 扫描 pending 并恢复入队

### 3.5 回执与历史查询

- 新增协议类型: `read_ack`
- 已有 `deliver_ack` 对应送达状态更新，新增 `read_ack` 对应已读状态更新
- 新增命令:
  - `read|server_msg_id` 上报已读
  - `history|用户名|条数` 查询 C2C 历史（含状态）
  - `help` 增补命令说明

## 4. 关键代码变更

- `main.go`：初始化 SQLite（`IM_DB_DSN`，默认 `gochat.sqlite3`）
- `src/storage/db.go`：驱动切换 + 自动迁移 `MessageRecipient`
- `src/storage/model.go`：消息模型与投递模型升级
- `src/server/store.go`：事务持久化、离线恢复、历史查询、回执状态更新
- `src/server/server.go`：`read_ack` 处理、C2C chat_id 规范化、pending 恢复入队
- `src/server/user.go` / `src/server/command.go`：`read/history/help` 命令与上线补投递
- `src/server/protocol.go`：新增 `TypeReadAck` 与 `Message.Status`

## 5. 验证结果

- `go test ./...` 通过
- `go build .` 通过
- 代码路径上满足:
  - 单聊发送/接收
  - 幂等发送（ClientMsgID）
  - 离线消息持久化 + 上线重投递
  - 送达/已读状态上报
  - 历史消息查询

## 6. 当前限制与后续优化

- 当前为单节点实现，分布式 Gate 需要 Redis 共享路由和分布式限流
- 重试策略目前偏轻量（上线触发重投递 + 启动恢复），可继续引入指数退避和死信队列
- 历史查询目前通过 TCP 命令返回文本，后续可补 REST/gRPC 查询接口
