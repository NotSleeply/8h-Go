# TCP 项目结构重构设计（8h-GoIM）

## 1. 背景与目标

当前项目在 `src/` 下存在包职责分散、目录层级混杂的问题。目标是：

1. 迁移到标准 Go 项目结构（`cmd/` + `internal/`）
2. 在不改变核心功能的前提下，简化重复结构
3. 保留 Redis 作为 MySQL 前缓存层
4. 去除已废弃的 MQ/分布式模块（已完成）

本次结构重构聚焦可维护性，不引入新业务能力。

---

## 2. 设计决策（已确认）

- 可以适当简化/合并重复代码
- 使用标准 Go 项目结构：`internal/` + `pkg/`（本项目不需要对外复用包，故全部进入 `internal/`）
- `session/user` 采用合并方案（A）
- `store/dao` 保持分层（B）
- `server` 与 `delivery` 分离（A）
- `logic`、`group` 合并进入 `server`
- `iface` 拆散后删除：
  - `Store` 接口迁到 `store`
  - `Sender`/`ServerAPI`/`GroupManagerAPI` 迁到 `server`
- `protocol` 继续独立（跨层共享模型）

---

## 3. 目标目录结构

```text
8h-GoIM/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── protocol/        # Message 结构体 + 消息类型常量
│   ├── cache/           # Redis 客户端封装
│   ├── storage/         # GORM DB 初始化 + ORM 模型
│   ├── dao/             # 纯数据库操作（user/message/recipient）
│   ├── store/           # 内存+持久化存储（含 Store 接口定义）
│   ├── session/         # 单连接生命周期 + 命令处理 + 注册/登录
│   ├── server/          # TCP 服务器 + group + logic + 对外接口
│   └── delivery/        # 投递 worker + enqueue + retry
└── go.mod
```

---

## 4. 包职责与文件落位

### 4.1 protocol

- 原：`src/protocol/protocol.go`
- 新：`internal/protocol/protocol.go`
- 作用：定义跨层共享消息模型（`Message`、`MsgType`）

### 4.2 cache

- 原：`src/cache/redis.go`
- 新：`internal/cache/redis.go`
- 作用：Redis client 初始化、key 生成规则、在线状态缓存辅助

### 4.3 storage

- 原：`src/storage/{db.go, model.go}`
- 新：`internal/storage/{db.go, model.go}`
- 作用：数据库连接与 ORM 模型

### 4.4 dao（保持分层）

- 原：`src/dao/*.go`
- 新：`internal/dao/`
  - `user.go`（原 `user_dao.go`）
  - `message.go`（原 `message_dao.go`）
  - `recipient.go`（原 `recipient_dao.go`）
- 约束：仅做数据访问，不承载连接态业务行为

### 4.5 store（保持分层）

- 原：`src/store/*.go`
- 新：`internal/store/`
- 新增：`iface.go`（接收从 `src/iface` 迁入的 `Store` 接口）
- 作用：内存态 + 持久化态统一存储逻辑、投递状态流转、历史查询

### 4.6 session（合并 user）

- 原：
  - `src/session/{session.go,commands.go,dispatch.go}`
  - `src/user/user.go`
- 新：`internal/session/`
  - `session.go`
  - `commands.go`
  - `dispatch.go`
  - `auth.go`（承接 `Register/Login`）
- 作用：单连接会话生命周期与命令入口；认证逻辑内聚到会话域

### 4.7 server（吸收 logic + group + iface）

- 原：`src/server/*.go` + `src/logic/logic.go` + `src/group/group.go` + `src/iface` 部分接口
- 新：`internal/server/`
  - 保留：`server.go/bootstrap.go/start.go/accept.go/conn.go/conn_security.go/broadcast.go/messages.go/id.go/stats.go/api.go`
  - 新增：`logic.go`（从 `src/logic/logic.go` 迁入）
  - 新增：`group.go`（从 `src/group/group.go` 迁入）
  - 新增：`iface.go`（`Sender/ServerAPI/GroupManagerAPI` 接口）
- 作用：TCP 服务主域，维护全局状态与核心业务协调

### 4.8 delivery（从 server 分离）

- 原：`src/server/deliver.go` + `src/server/enqueue.go` 中投递相关逻辑
- 新：`internal/delivery/`
  - `worker.go`（DeliverWorker）
  - `enqueue.go`（EnqueueServerMsg/pushDeliverQueue/EnqueuePendingForUser/RecoverPendingDeliveries）
  - `retry.go`（RetryWorker）
- 作用：消息投递流水线独立，避免 server 包继续膨胀

---

## 5. 依赖关系约束

目标依赖方向（单向）：

- `cmd/server` → `internal/server`, `internal/storage`, `internal/cache`
- `internal/server` → `internal/session`, `internal/store`, `internal/delivery`, `internal/protocol`
- `internal/session` → `internal/server`（仅依赖接口）、`internal/dao`, `internal/protocol`
- `internal/delivery` → `internal/store`, `internal/server`（仅依赖接口）
- `internal/store` → `internal/storage`, `internal/cache`, `internal/protocol`
- `internal/dao` → `internal/storage`, `internal/cache`

禁止新增反向依赖，尤其禁止 `dao` 依赖 `server/session`。

---

## 6. 重构边界与兼容策略

1. 仅做结构迁移与小规模职责内聚，不改消息协议字段
2. 不改变 Redis 缓存行为（用户缓存、在线键、待投递缓存）
3. 不改变 MySQL 表结构
4. 删除 `src/iface`、`src/logic`、`src/group` 后由新位置完全替代
5. 所有 import 路径一次性替换到 `module goim`

---

## 7. 验证策略

最小验证集：

1. `go test ./...` 通过
2. `go build ./...` 通过
3. 手动冒烟：
   - 注册/登录
   - 私聊消息发送与送达
   - `read` 回执
   - 群创建/入群/发群消息
   - `history` 查询
4. Redis 开启情况下缓存命中与在线状态续约不报错

---

## 8. 风险与应对

### 风险 1：循环依赖

- 场景：`session` 与 `server` 双向引用
- 应对：接口统一放 `internal/server/iface.go`，`session` 只持有接口类型

### 风险 2：路径替换遗漏

- 场景：迁移后部分 import 仍指向 `tet/src/...`
- 应对：按包批次迁移 + 每批次 `go build ./...` 校验

### 风险 3：职责回潮

- 场景：迁移后又将投递逻辑写回 `server`
- 应对：明确 delivery 包边界：enqueue/retry/worker 必须在 `internal/delivery`

---

## 9. 完成定义（Definition of Done）

1. 根目录不再有 `src/` 作为业务代码容器
2. 所有业务代码位于 `cmd/` 与 `internal/`
3. `logic/group/iface` 旧目录移除且功能可用
4. `go test ./...`、`go build ./...` 均通过
5. 聊天主流程（注册/登录/私聊/群聊/回执）可运行

---

## 10. 非目标（明确不做）

1. 不做协议升级（不新增消息字段）
2. 不做数据库 schema 重构
3. 不引入新的中间件或消息队列
4. 不做跨进程分布式改造
