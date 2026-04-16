# Issue39 交付记录: S2C 消息推送（重试 + 死信 + 指标）

## 1. 问题描述

Issue #39 要求投递模块具备可恢复投递语义，核心包括：失败重试、回执状态、死信处理和观测能力。  
前置版本已具备基本推送与回执，但缺少系统化重试与死信闭环。

## 2. 技术难点

- 需要避免简单“离线即丢弃”策略，改为可控重试并持久化状态
- 要将重试调度与现有 `DeliverQueue` 协作，避免侵入过大
- 必须有可观测数据支撑（pending/delivered/read/dead）

## 3. 解决方案

### 3.1 投递状态扩展

- `MessageRecipient.Status` 扩展为:
  - `0 pending`
  - `1 delivered`
  - `2 read`
  - `3 dead-letter`

### 3.2 重试与死信策略

- 新增 `ScheduleRetry(serverMsgID, to, err, maxRetries, baseBackoff)`：
  - 按指数退避计算 `next_retry_at`
  - 递增 `retry_count`
  - 超过最大重试次数转 `dead-letter`
- 新增 `GetDueRetryServerMsgIDs(limit)`：
  - 扫描 `next_retry_at <= now` 的 pending 记录
  - 入队重试
- 新增 `RetryWorker`（每秒扫描）自动触发重试

### 3.3 Deliver 流程增强

- `DeliverWorker` 在目标用户离线时，不再仅跳过:
  - 记录失败原因
  - 调用 `ScheduleRetry`
  - 超过阈值时标记 dead-letter 并记录日志

### 3.4 指标增强

- 新增 `DeliveryStats()` 聚合:
  - `pending`, `delivered`, `read`, `dead`
- `stats` 命令新增输出:
  - `delivery_pending`
  - `delivery_delivered`
  - `delivery_read`
  - `dead_letter`

## 4. 代码变更

- `src/server/store.go`
  - 新增重试调度、到期扫描、死信标记、投递状态统计
- `src/server/server.go`
  - 新增重试配置与 `RetryWorker`
  - `SnapshotStats` 增加投递状态指标
- `src/server/deliver.go`
  - 离线投递失败时触发重试/死信
- `src/server/command.go`
  - `stats` 展示新增投递状态指标
- `src/storage/model.go`
  - `MessageRecipient.Status` 注释扩展死信状态

## 5. 配置项

- `IM_DELIVER_MAX_RETRY`（默认 `5`）
- `IM_DELIVER_RETRY_BASE_SEC`（默认 `2` 秒）

## 6. 验证结果

- `go test ./...` 通过
- `go build .` 通过
- 离线用户路径具备“重试 -> 死信”闭环能力
- 在线 `stats` 可观测当前投递积压与死信数量

## 7. 后续优化建议

- 当前重试失败仅记录日志，后续可加告警（Webhook/钉钉/邮件）
- 可引入独立死信队列（DLQ）主题，便于异步补偿
- 可增加每用户/每会话级别限流，进一步稳定峰值投递
