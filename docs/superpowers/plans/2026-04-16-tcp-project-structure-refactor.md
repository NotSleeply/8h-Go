# 实施计划：TCP 项目结构重构

设计文档：[docs/superpowers/specs/2026-04-16-tcp-project-structure-design.md](../specs/2026-04-16-tcp-project-structure-design.md)

## 执行策略

按包批次迁移，每批次完成后执行 `go build ./...` 验证，确保每步可回滚。

---

## Step 1 — 更新 go.mod module 名

**文件：** `go.mod`

将 `module tet` 改为 `module goim`。

---

## Step 2 — 创建目录结构

```bash
mkdir -p internal/protocol
mkdir -p internal/cache
mkdir -p internal/storage
mkdir -p internal/dao
mkdir -p internal/store
mkdir -p internal/session
mkdir -p internal/server
mkdir -p internal/delivery
mkdir -p cmd/server
```

---

## Step 3 — 迁移无依赖包（protocol / cache / storage）

这三个包无业务依赖，直接复制并更新 package 声明与 import 路径。

### 3.1 internal/protocol/protocol.go
- 原文件：`src/protocol/protocol.go`
- 改动：`package protocol`（不变），import 路径无需改动（无外部依赖）

### 3.2 internal/cache/redis.go
- 原文件：`src/cache/redis.go`
- 改动：`package cache`（不变），无外部 import 需更新

### 3.3 internal/storage/db.go + model.go
- 原文件：`src/storage/db.go`, `src/storage/model.go`
- 改动：`package storage`（不变），无外部 import 需更新

**验证：** `go build ./internal/protocol/... ./internal/cache/... ./internal/storage/...`

---

## Step 4 — 迁移 dao

### 4.1 internal/dao/user.go
- 原文件：`src/dao/user_dao.go`
- import 更新：
  - `tet/src/cache` → `goim/internal/cache`
  - `tet/src/storage` → `goim/internal/storage`

### 4.2 internal/dao/message.go
- 原文件：`src/dao/message_dao.go`
- import 更新同上

### 4.3 internal/dao/recipient.go
- 原文件：`src/dao/recipient_dao.go`
- import 更新同上

**验证：** `go build ./internal/dao/...`

---

## Step 5 — 迁移 store（含 Store 接口）

### 5.1 internal/store/iface.go（新文件）
将 `src/iface/interfaces.go` 中的 `Store` 接口和 `DeliveryStatusStats` 迁入，删除 `MessageBusIface`（已删）。

```go
package store

import (
    "time"
    "goim/internal/protocol"
)

type DeliveryStatusStats struct {
    Pending   int64
    Delivered int64
    Read      int64
    Dead      int64
}

type Store interface {
    NextSeq(chatID string) (uint64, error)
    SaveMessage(msg *protocol.Message) error
    SaveMessageWithRecipients(msg *protocol.Message, recipients []string) error
    GetMessageByClientID(from, clientMsgID string) *protocol.Message
    GetMessageByServerID(serverMsgID string) *protocol.Message
    SaveDelivery(serverMsgID, to string) error
    GetRecipients(serverMsgID string) []string
    MarkDeliverySent(serverMsgID, to string, sendErr error) error
    MarkDeliveryAcked(serverMsgID, to string) error
    MarkDeliveryRead(serverMsgID, to string) error
    ScheduleRetry(serverMsgID, to string, lastErr error, maxRetries int, baseBackoff time.Duration) (bool, error)
    GetDueRetryServerMsgIDs(limit int) []string
    DeliveryStats() DeliveryStatusStats
    RecoverPendingServerMsgIDs(limit int) []string
    ListPendingServerMsgIDsByUser(toUser string, limit int) []string
    GetC2CHistory(userA, userB string, limit int) []*protocol.Message
}
```

### 5.2 其余 store 文件
迁移 `src/store/{types,messages,delivery,retry,history,helpers,convert}.go`，更新 import：
- `tet/src/cache` → `goim/internal/cache`
- `tet/src/protocol` → `goim/internal/protocol`
- `tet/src/storage` → `goim/internal/storage`
- `tet/src/iface` → `goim/internal/store`（`iface.DeliveryStatusStats` → `store.DeliveryStatusStats`）

**验证：** `go build ./internal/store/...`

---

## Step 6 — 迁移 server（含 logic + group + iface 接口）

### 6.1 internal/server/iface.go（新文件）
将 `src/iface/interfaces.go` 中的 `Sender`、`ServerAPI`、`GroupManagerAPI`、`OnlineUserInfo` 迁入。

```go
package server

import "goim/internal/protocol"

type Sender interface {
    SendJSON(m *protocol.Message)
    SendMsg(msg string)
}

type OnlineUserInfo struct {
    Name string
    Addr string
}

type GroupManagerAPI interface {
    Create(groupID, owner string) error
    Join(groupID, username string) error
    Leave(groupID, username string) error
    Delete(groupID, username string) error
    Kick(groupID, by, target string) error
    GrantAdmin(groupID, by, target string) error
    RevokeAdmin(groupID, by, target string) error
    Members(groupID string) []string
    RoleOf(groupID, username string) (string, bool)
}

type ServerAPI interface {
    RegisterOnline(name string, s Sender, addr string)
    UnregisterOnline(name string)
    EnqueuePendingForUser(username string, limit int)
    BroadcastSystemEvent(body string, exclude string) (serverMsgID string, seq uint64)
    ProcessSend(req *protocol.Message, recipients []string) (*protocol.Message, *protocol.Message, error)
    HandleDeliverAck(username, serverMsgID string)
    HandleReadAck(username, serverMsgID string)
    EnqueueServerMsg(serverMsgID string)
    GetC2CHistory(userA, userB string, limit int) []*protocol.Message
    SnapshotStatsText() string
    MarkOutbound()
    ListOnlineUsers() []OnlineUserInfo
    GroupManager() GroupManagerAPI
}
```

### 6.2 internal/server/logic.go（新文件，原 src/logic/logic.go）
- package 改为 `server`
- 类型名 `LogicService` 保持不变（私有使用）
- import 更新：
  - `tet/src/iface` → `goim/internal/store`（`iface.Store` → `store.Store`）
  - `tet/src/protocol` → `goim/internal/protocol`
  - `tet/src/utils` → `goim/internal/utils`

### 6.3 internal/server/group.go（新文件，原 src/group/group.go）
- package 改为 `server`
- 类型名 `GroupManager` 保持不变

### 6.4 其余 server 文件
迁移 `src/server/{server,bootstrap,start,accept,conn,conn_security,broadcast,messages,id,stats,api,protocol}.go`，更新 import：
- `tet/src/session` → `goim/internal/session`
- `tet/src/user` → 删除（Register/Login 已移入 session/auth.go）
- `tet/src/logic` → 删除（logic 已内联到 server 包）
- `tet/src/iface` → `goim/internal/store`（Store 接口）或 `goim/internal/server`（Sender/ServerAPI）
- `tet/src/store` → `goim/internal/store`
- `tet/src/protocol` → `goim/internal/protocol`

**server.go 字段变更：**
- `logic *logicpkg.LogicService` → `logic *LogicService`（同包，去掉包前缀）
- `groupManager *group.GroupManager` → `groupManager *GroupManager`（同包）
- `store iface.Store` → `store store.Store`

**id.go 变更：**
- 删除 `Logic()` 方法（logic 已内联，不需要对外暴露）
- `GenerateServerMsgID` 直接调用 `s.logic.GenerateServerMsgID("s")`

**api.go 变更：**
- 删除 `groupAdapter` 结构体（group 已内联，`GroupManager` 直接实现 `GroupManagerAPI`）
- `GroupManager()` 直接返回 `s.groupManager`

**验证：** `go build ./internal/server/...`

---

## Step 7 — 迁移 session（合并 user/auth）

### 7.1 internal/session/auth.go（新文件，原 src/user/user.go）
- package 改为 `session`
- 函数签名不变：`Register(s *User, ...)`, `Login(s *User, ...)`
- import 更新：
  - `tet/src/dao` → `goim/internal/dao`
  - `tet/src/session` → 删除（同包）
  - `tet/src/storage` → `goim/internal/storage`

### 7.2 其余 session 文件
迁移 `src/session/{session,commands,dispatch}.go`，更新 import：
- `tet/src/cache` → `goim/internal/cache`
- `tet/src/iface` → `goim/internal/server`（`iface.ServerAPI` → `server.ServerAPI`，`iface.Sender` → `server.Sender`，`iface.OnlineUserInfo` → `server.OnlineUserInfo`）
- `tet/src/protocol` → `goim/internal/protocol`

**session.go 字段变更：**
- `Server iface.ServerAPI` → `Server server.ServerAPI`

**conn.go（server 包）变更：**
- 删除 `userpkg "tet/src/user"` import
- `userpkg.Register(user, ...)` → `session.Register(user, ...)`
- `userpkg.Login(user, ...)` → `session.Login(user, ...)`

**验证：** `go build ./internal/session/...`

---

## Step 8 — 迁移 delivery

### 8.1 internal/delivery/worker.go
- 原文件：`src/server/deliver.go`
- package 改为 `delivery`
- `DeliverWorker` 需要访问 `Server` 的字段（`DeliverQueue`、`store`、`OnlineMap`、`MapLock` 等）
- 方案：将 `DeliverWorker` 改为接受接口参数，或将 `Server` 作为 `delivery.Worker` 的字段

**delivery.Worker 结构：**
```go
package delivery

type Worker struct {
    Queue           chan string
    Store           store.Store
    GetOnline       func(name string) (server.Sender, bool)
    MaxRetry        int
    RetryBaseDelay  time.Duration
    InFlight        map[string]struct{}
    InFlightMu      sync.Mutex
}
```

### 8.2 internal/delivery/enqueue.go
- 原文件：`src/server/enqueue.go`
- `EnqueueServerMsg`、`pushDeliverQueue`、`EnqueuePendingForUser`、`RecoverPendingDeliveries` 移入 `Worker` 的方法

### 8.3 internal/delivery/retry.go
- 原文件：`src/server/enqueue.go` 中的 `RetryWorker`

**server.go 变更：**
- 删除 `DeliverQueue`、`deliverInFlight`、`deliverInFlightMu`、`maxDeliverRetry`、`retryBaseDelay` 字段
- 新增 `worker *delivery.Worker` 字段
- `EnqueueServerMsg` 委托给 `s.worker.EnqueueServerMsg`

**验证：** `go build ./internal/delivery/... ./internal/server/...`

---

## Step 9 — 迁移 utils 并更新 cmd/server/main.go

### 9.1 internal/utils/env.go + helpers.go
- 原文件：`src/utils/{env,helpers}.go`
- 更新所有引用方的 import

### 9.2 cmd/server/main.go
- 原文件：`main.go`
- import 更新：
  - `tet/src/server` → `goim/internal/server`
  - `tet/src/storage` → `goim/internal/storage`
  - `tet/src/utils` → `goim/internal/utils`

**验证：** `go build ./...`

---

## Step 10 — 删除旧目录

```bash
rm -rf src/
rm main.go
```

**最终验证：**
```bash
go build ./...
go test ./...
```

---

## 检查点汇总

| Step | 操作 | 验证命令 |
|------|------|---------|
| 1 | 更新 go.mod | `go mod tidy` |
| 2 | 创建目录 | — |
| 3 | protocol/cache/storage | `go build ./internal/protocol/... ./internal/cache/... ./internal/storage/...` |
| 4 | dao | `go build ./internal/dao/...` |
| 5 | store + Store 接口 | `go build ./internal/store/...` |
| 6 | server + logic + group | `go build ./internal/server/...` |
| 7 | session + auth | `go build ./internal/session/...` |
| 8 | delivery | `go build ./internal/delivery/... ./internal/server/...` |
| 9 | utils + cmd/server | `go build ./...` |
| 10 | 删除 src/ | `go build ./...` + `go test ./...` |
