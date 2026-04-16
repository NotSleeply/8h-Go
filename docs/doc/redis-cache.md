# Redis 缓存设计与实现（技术说明）

概述
--

本项目在消息投递与用户查询的若干热点路径引入短期 Redis 缓存，采用 cache-aside 模式。目标是降低数据库（MySQL）在短时高并发下的读压力、减少重复查询噪音，并提升端到端的投递性能，同时在 Redis 不可用时能够平滑降级到数据库以保证可用性。

主要解决的问题
--

- MySQL 热点读压：短时间内对同一数据的重复访问（如收件人列表、单用户未投递列表）会触发大量相同的 DB 查询，缓存缓解了这种瞬时峰值。
- 重复投递时的 DB 噪音：发送/重试并发场景下，短期缓存降低了重复查询造成的 I/O 与延迟。
- 提升响应速度：缓存命中时避免了 DB 访问，投递调度与排队逻辑能更快完成决策。

模块化实现概览（按职责）
--

- 缓存层（Cache）：负责初始化 Redis 客户端、提供带命名约定的 key 生成器、以及包装基本的 Get/Set/Del 操作。对外暴露简单的 helper，供上层模块直接使用。
- 存储层（Store）：在消息写入与读取路径实现 cache-aside 策略。对于收件人列表、全局/单用户的待投递/重试 ID 列表，先尝试从 Redis 读取；未命中则回退到数据库查询并将结果写回 Redis（短 TTL）。写入路径在数据库事务成功后负责更新/失效相关缓存。
- DAO 层：用户信息查询（如按用户名查用户）也采用 cache-aside，缓存用户信息以减少对用户表的频繁访问。
- 服务层（Server）：负责实时投递逻辑与短期去重（内存级 in‑flight map + 锁）。缓存不承担去重语义，去重由服务层的内存结构与队列逻辑保障。

Key 命名与 TTL（当前约定）
--

- 收件人列表（示例键样式）：`deliver:recips:<serverMsgID>`，TTL：5 秒（用于短期热点保护）。
- 单用户未投递列表：`deliver:pending:user:<username>`，TTL：2 秒（短期缓存用户待投递项）。
- 全局待投递 / 待恢复列表：`deliver:pending:due` / `deliver:pending:recover`，TTL：2 秒。
- 用户信息：`user:<username>`，TTL：30 分钟（降低用户表读取压力）。

注：上述 TTL 为经验值，建议在生产中可配置化并基于命中率调整。

读路径（典型实现思路）
--

读操作按 cache-aside 流程实现，伪代码如下：

```go
ctx := context.Background()
if client != nil {
  if v, err := client.Get(ctx, key).Result(); err == nil && v != "" {
    var out []string
    if err := json.Unmarshal([]byte(v), &out); err == nil {
      return out
    }
  }
}
// cache miss -> 查询数据库
rows := queryDB(...) // 返回 []row
out := extractIDs(rows)
if client != nil {
  if b, err := json.Marshal(out); err == nil {
    _ = client.Set(ctx, key, b, ttl).Err()
  }
}
return out
```

写路径与失效策略（典型实现思路）
--

写操作通常遵循「先写数据库事务，事务成功后更新/删除缓存」的原则：

```go
// 在 DB 事务中创建消息与收件人行
tx := db.Begin()
// insert message, insert recipients
tx.Commit()

// 事务成功后：
if client != nil {
  // 写收件人缓存（短 TTL）
  _ = client.Set(ctx, recipsKey, jsonBytes, 5*time.Second).Err()
  // 失效受影响的 per-user pending key 与全局 pending keys
  _ = client.Del(ctx, pendingUserKey(user)).Err()
  _ = client.Del(ctx, pendingDueKey).Err()
  _ = client.Del(ctx, pendingRecoverKey).Err()
}
```

要点说明
--

- 序列化：缓存值以 JSON 序列化（数组或对象），便于跨语言读取与排查。
- 失效时机：写路径在 DB 事务成功后才对缓存进行写入或 DEL 操作，尽量减少竞态窗口。
- 容错：所有 Redis 调用均以“可选”方式使用（客户端可能为 nil 或操作返回错误时直接回退到 DB），不会让缓存层影响主流程可用性。
- 去重：短期去重依赖服务层的内存 in‑flight map 与互斥保护，不依赖缓存来提供幂等性保证。

为何能解决问题（价值点）
--

- 降低数据库瞬时负载：秒级缓存能显著减少短时间内的重复查询，避免 DB 在并发高峰被击穿。
- 提升延迟表现：缓存命中避免 IO，投递路径响应更快，整体系统吞吐更稳。
- 降低重试风暴影响：发送重试并发时，缓存缓冲了大量重复读取操作，减少磁盘与网络压力。

可观测性与进一步改进建议
--

- 指标埋点：为关键缓存操作增加命中/未命中计数器、Redis 错误计数、序列化错误率等指标，用于实时调整 TTL 与排查问题。
- 参数化：把各类 TTL 与开关配置化（环境变量或配置文件），方便在不同环境下试验与回滚。
- 防穿透：对恶意或异常查询考虑引入 Bloom filter 或本地短期抑制（避免攻击导致缓存被快速击穿）。
- 本地一级缓存：针对非常高 QPS 场景可在进程内增加 LRU 作为第一缓存层，Redis 作为二级缓存。

手工验证步骤（简要）
--

1. 启动 Redis 并确保服务能连接到该实例。
2. 发送一条包含多个收件人的消息；确认写入后能在 Redis 中看到收件人缓存（JSON 列表）。
3. 发起对同一消息收件人列表的多次快速读取，观察缓存命中（减少 DB 访问）。
4. 触发消息的 ack/read 或重试调度，确认相关 pending key 被删除（代表缓存失效已生效）。

总结
--

当前实现以短期 cache-aside 为核心，侧重在秒级窗口内保护消息投递与用户查询的热点读，解决了短时间内大量重复 DB 查询带来的性能问题，同时保持了对 Redis 故障的降级能力。建议下一步先补充缓存命中率监控并把关键 TTL 提取为可配置项，再评估是否引入更复杂的防穿透措施。
