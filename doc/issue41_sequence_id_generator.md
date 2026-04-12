# Issue41 交付记录: 消息序号 / ID 生成器

## 1. 问题描述

Issue #41 要求提供统一的消息 ID 与序号生成能力：

- 全局唯一 `ServerMsgID`
- 按 `chat_id` 递增的 `Seq`
- 对外暴露生成接口，便于同步/重放链路复用

## 2. 技术难点

- 需要保证在并发场景下 `ServerMsgID` 不冲突
- `Seq` 既要按会话递增，又要兼容重启恢复和持久化场景
- 不能破坏已有消息发送与 ACK 主流程

## 3. 解决方案

### 3.1 统一 ID 生成逻辑

- 继续使用 `LogicService.GenerateServerMsgID(prefix)` 作为核心生成器
- 生成格式：`<prefix>-<unix_milli>-<atomic_counter>`
  - 示例：`s-1712937600123-42`
  - 特性：同进程内高并发无重复，可读性好

### 3.2 会话序号生成

- 复用 `store.NextSeq(chatID)`：
  - 持久化开启时，首次会从 DB 读取该会话最大 seq 作为基线
  - 后续内存递增，保证同进程内单调递增

### 3.3 对外接口暴露

在 `Server` 层新增显式接口：

- `GenerateServerMsgID() string`
- `NextSeq(chatID string) uint64`

这样业务模块或后续 API/gRPC 层可以直接调用，不必耦合内部逻辑实现。

## 4. 代码变更

- 修改: `src/server/server.go`
  - 新增 `GenerateServerMsgID()`
  - 新增 `NextSeq(chatID string)`

## 5. 并发与一致性说明

- `ServerMsgID`：依赖 `atomic counter + 时间戳`，满足进程内并发唯一
- `Seq`：由 `InMemoryStore` 互斥锁保护递增；持久化模式下启动可从 DB 最大值恢复
- 事务边界：消息保存仍走 `SaveMessageWithRecipients` 事务，不影响原一致性设计

## 6. 验证结果

- `go test ./...` 通过
- `go build .` 通过
- 现有消息发送流程正常，新增接口可直接复用
