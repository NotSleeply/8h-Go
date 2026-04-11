# Issue 统一设计审查与修正建议

此文档为对 `doc/issue` 中所有 IM 模块 issue 的统一设计审查结果与可落地的修正建议汇总。目标是列出容易在实现/运行中造成数据丢失、重复、顺序错乱或可扩展性受限的关键点，并给出可执行的修正方案与优先级。

---

## 一、问题概要（高层）

- 消息持久化与投递队列割裂：内存队列/chan、非事务化写入会在进程崩溃或队列满时丢失消息。
- 接收者投递状态不可查询：缺少按接收者维度的 delivery 表，难以重试/补偿/统计。
- 幂等性索引不够严谨：单列 `client_msg_id` 在多客户端场景下可能冲突。
- Seq/ServerMsgID 发号本地化：单机实现对横向扩展不友好，必须抽象发号器。
- 传输协议脆弱：行分割 JSON 对粘包/半包/大消息不稳健，生产建议 length-prefix 或 protobuf。
- SQLite 可并发写限制未说明：写锁/事务/PRAGMA 未列出，易导致运行时死锁或性能瓶颈。

## 二、核心修正建议（必须 P0）

1) 引入 `message_recipient`（或 `delivery`）表

示例 SQL（可根据 GORM model 转换）:

```sql
CREATE TABLE messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  server_msg_id TEXT NOT NULL UNIQUE,
  client_msg_id TEXT,
  chat_id TEXT NOT NULL,
  from_user TEXT NOT NULL,
  chat_type INTEGER NOT NULL,
  content TEXT,
  seq INTEGER,
  status INTEGER DEFAULT 0,
  create_time DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE message_recipient (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  server_msg_id TEXT NOT NULL,
  recv_user TEXT NOT NULL,
  status INTEGER DEFAULT 0, -- 0: pending, 1: sent, 2: acked, 3: dead
  retry_count INTEGER DEFAULT 0,
  last_send_time INTEGER,
  CONSTRAINT idx_msg_recv UNIQUE(server_msg_id, recv_user)
);

CREATE INDEX idx_msg_chat_seq ON messages(chat_id, seq);
CREATE INDEX idx_recipient_recv_status ON message_recipient(recv_user, status);
```

1) 保存消息时使用事务：消息 + recipient 行一并写入

伪代码（事务）:

```go
tx := db.Begin()
tx.Create(&message)
for _, recv := range recipients {
  tx.Create(&message_recipient{ServerMsgID: message.ServerMsgID, RecvUser: recv})
}
tx.Commit()
// 成功后将 server_msg_id 推入 Deliver 队列（持久化队列或写入同一 DB 的 work_queue 表）
```

注意：不要在 commit 前将消息放入仅内存队列，否则可能出现写入 DB 失败但已入队的情况。

1) 入队持久化与恢复

- 建议把“待投递项”也写为持久化记录（例如 `message_recipient` 的 pending 行即代表待投递项）；DeliverWorker 从 DB 查询 pending 条目并投递。
- 启动时扫描 `message_recipient` 中 `status != acked` 的记录并恢复投递。

伪代码（启动恢复）:

```go
rows := db.Query("SELECT server_msg_id FROM message_recipient WHERE status=0 ORDER BY last_send_time ASC LIMIT 1000")
for each row { enqueue(row.server_msg_id) }
```

1) 幂等性与 ID 设计

- `ClientMsgID` 应作为 `(from_user, client_msg_id)` 的复合键，或由客户端保证全局唯一（UUID 等）。
- `ServerMsgID` 建议使用全局唯一的 Snowflake/UUID；如果希望使用整数 seq，请在文档中统一类型，并确保代码与 DB 类型一致。

1) 投递重试/死信/告警

- 对每条 recipient 记录维护 `retry_count`，当超过阈值（如 5 次）将其标记为 dead 并写入 `dead_letter` 或产生告警。
- 对投递失败实行指数退避：1s, 2s, 5s, 10s, ...

1) SQLite 生产注意（若继续使用 SQLite）

- 启用 WAL 模式：`PRAGMA journal_mode = WAL;`
- 设置同步级别：`PRAGMA synchronous = NORMAL;`
- 设定 busy timeout：`PRAGMA busy_timeout = 5000;`
- 限制并发写：`db.SetMaxOpenConns(1)`（写密集场景下），并用事务合并写操作以减少锁竞争。
- 若写压大，尽早迁移到 MySQL/Postgres，或将写入改为 MQ（Kafka/Redis）然后异步落盘。

1) 协议与接入建议

- 对 TCP 使用 length-prefix 帧或 Protobuf，以解决粘包/半包情况。
- 在握手阶段完成 auth（token）验证，并建立心跳（keepalive）机制以判断断线。

## 三、示例：保存并入持久化队列（事务 + 入队）

伪代码：

```go
func SaveAndEnqueue(db *gorm.DB, msg *Message, recipients []string) error {
  return db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(msg).Error; err != nil { return err }
    for _, r := range recipients {
      if err := tx.Create(&MessageRecipient{ServerMsgID: msg.ServerMsgID, RecvUser: r}).Error; err != nil { return err }
    }
    return nil
  })
  // 事务提交后，从 DB 查询刚写入的 pending 记录并交给 DeliverWorker 处理（或写入持久化队列）
}
```

## 四、优先级与逐步实施计划

- P0（必须立即实现）
  - 抽象 `Store` 接口并实现基于 DB 的持久化（messages + message_recipient）
  - 在保存时使用事务并保证写成功后才返回 send_ack
  - 在启动时恢复未 ack 的投递项，且 DeliverWorker 以 DB 为准驱动投递
  - 明确 `ServerMsgID` 与 `ClientMsgID` 的类型与唯一性约定

- P1（增强可靠性）
  - 投递重试/死信/告警
  - 支持 Kafka/Redis 作为消息缓冲层（在高并发场景）
  - 增加 admin/metrics 接口：pending count、queue depth、retry statistics

- P2（扩展能力）
  - 横向扩展（发号器 Snowflake、分布式 Redis/DB、MQ 替代 SQLite 写）
  - 性能优化（批量插入、分片、分布式路由）

## 五、下一步建议

1. 先在 `doc/issue/issue_template_1.md` 的 Storage 实现中加入 `message_recipient` 模型与事务化保存示例。
2. 将 `DeliverWorker` 从内存队列迁移为 DB 驱动：查询 pending recipient，按批次投递并更新状态。
3. 在项目中新增 `doc/issue/ISSUE_DESIGN_REVIEW.md`（本文件）并在所有 issue template 中引用（已完成）。

---

如需，我可以把 P0 的代码骨架（GORM model、事务示例、启动恢复脚手架）直接提交为 PR 到仓库。请确认是否需要我继续实现这些代码。
