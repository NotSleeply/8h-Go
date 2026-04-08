# 在轻量 Go 聊天服务中实现消息 ACK：一次可落地的实践记录

最近我在一个教学性质的 Go TCP 聊天服务上做了一个小但关键的改造：加入消息 ACK（确认）机制。本文以技术博客的形式写下设计动机、实现要点、遇到的坑与下一步思路，供想把“玩具聊天室”往“可用消息通道”升级的人参考。

---

## 为什么要加 ACK？

原始实现是把任意文本通过一个广播通道推给所有在线用户，代码结构简单，适合教学。但这带来三个现实问题：

- 不重（去重）：客户端重复发送会产生重复消息；
- 不乱序（有序）：并发发送时无法在会话内保证顺序；
- 不丢（可靠）：进程崩溃、网络抖动或对端掉线都会导致消息丢失或无法确认。

目标并不要求做到分布式一致性或全局严格顺序，而是先实现在线场景下的“不重、不乱序、可确认”。把这些打通后，再逐步引入持久化与离线补偿。

---

## 设计原则（约束）

- 最小入侵：尽量复用现有按行读取的框架（每行承载一个 JSON）；
- 明确边界：协议层（消息字段与类型）要清晰，业务层只按类型处理；
- 渐进式演进：先做内存 Store 验证逻辑，再替换为持久化实现。

因此我采用了单行 JSON 协议，核心消息类型为 `send` -> `send_ack` -> `deliver` -> `deliver_ack`，并在服务端引入一个轻量 Store（当前为内存实现）和一个投递队列。

---

## 协议与核心字段（简述）

- `type`：`send|send_ack|deliver|deliver_ack|sync`，明确消息语义；
- `client_msg_id`：客户端侧唯一 ID，做幂等去重键；
- `server_msg_id`：服务器生成的唯一 ID，用于投递与确认；
- `chat_id`：会话维度的序号空间（按 chat 分配 `seq`）；
- `seq`：服务端在 `chat_id` 范围内分配的单调序号，用以会话内有序保证。

示例：

```json
{"type":"send","client_msg_id":"c123","chat_id":"room1","from":"alice","body":"hello"}
```

收到后服务端会回复 `send_ack`（包含 `server_msg_id` 与 `seq`），随后把 `deliver` 发给接收者，接收者再发回 `deliver_ack`。

---

## 关键组件与实现要点（逻辑级别）

1) Store（当前是 `InMemoryStore`）

- 功能：`NextSeq(chatID)`、保存 message、按 `(from, client_msg_id)` 查重、为每条消息记录待投递接收者并标记 ack 状态。
- 目的：保证幂等、分配序号、维护 pending delivery 状态。

1) DeliverQueue + DeliverWorker

- DeliverQueue 存放 `server_msg_id`。投递 worker 取出 ID，从 Store 拉到 message 与 pending recipient 列表，向在线接收方发送 `deliver`。
- 如果接收方在线但短期写阻塞，worker 记录该状态并重试；如果接收方不在线，则保留 pending，等客户端重连或后续补偿。

1) 服务端消息入口

- 客户端每行 JSON 被解析后分发：`send` 走 `HandleClientSend`（幂等检查 -> NextSeq -> SaveMessage -> SaveDelivery -> reply send_ack -> enqueue deliver`）；
- `deliver_ack` 则调用 `MarkDeliveryAcked` 标记该接收者的 delivery 完成。

伪码（HandleClientSend）：

```text
if store.GetMessageByClientID(from, client_msg_id) != nil:
  reply existing send_ack
else:
  seq = store.NextSeq(chat_id)
  server_msg_id = genID()
  store.SaveMessage(server_msg)
  for r in recipients: store.SaveDelivery(server_msg_id, r)
  reply send_ack(server_msg_id, seq)
  enqueue DeliverQueue <- server_msg_id
```

伪码（DeliverWorker）：

```text
for server_msg_id in DeliverQueue:
  msg = store.GetMessage(server_msg_id)
  recips = store.GetRecipients(server_msg_id)
  for r in recips:
    if r online: send deliver to r
    else: keep pending
```

---

## 实际问题与权衡（笔记）

- 幂等性来源：使用 `(from, client_msg_id)` 做幂等键，务必让客户端生成有足够熵的 ID（如 UUID 或时间戳+随机）。
- 写阻塞与背压：原实现中直接在广播协程写 `Conn` 会导致全局阻塞。解决办法是每个用户单独的写 goroutine + channel，投递 worker 向该 channel 写入 JSON 串并设置写超时。若 channel 或写超时失败，应将该用户标记为“拥塞”并最终下线或暂缓重试。
- 去重与顺序：分配 seq 在服务端完成，能保证同一 chat 的单调性；但若要跨多个接收者保证“严格同步可见的顺序”，则需更复杂的序列化策略（通常不是简单聊天室需要的）。

---

## 如何测试（快速上手）

1. 本地启动服务：

```bash
go run main.go
```

1. 用两个终端或脚本模拟客户端发送/接收 JSON，验证 `send` -> `send_ack` -> `deliver` -> `deliver_ack` 的完整链路；也可写一个小脚本自动发送 `deliver_ack` 以便验证 Store 中 pending 状态变更。

我通常会做的两项自动化测试：

- 并发发送的去重测试：同一 `client_msg_id` 重发多次，确认服务端只保存一份消息并返回相同 `send_ack`；
- 重连补偿测试：发送消息给离线用户，用户上线后执行 `sync`（拉取未 ack 的 seq）并确认全部消息补发。

---

## 下一步建议（工程化方向）

1. 持久化 Store：把 `InMemoryStore` 换成 SQLite（或 BoltDB），在服务启动时恢复 pending delivery，并保证 seq 分配的原子性（DB 事务或自增列）；
2. 重试机制：为 DeliverWorker 加入指数退避、最大重试次数、失败告警与持久化标记；
3. 监控与指标：记录未 ack 数量、平均投递延迟、重试次数分布等，以便观察系统健康；
4. 协议兼容：保持现有文本命令向后兼容（例如：普通文本依然包装成 `send`），并为更复杂消息类型（图片/文件）扩展 `body` 或增加 `attachment` 字段。

---

## 结语

这一改造是在原有教学示例上做的小步前进：通过一套轻量的协议与内存 Store，我们把聊天室从“只会广播字符串”进化成“能确认消息已送达并支持去重、有序”的消息通道。最关键的收获是：设计需把边界划清楚——协议、存储、投递各司其职——这样后续从内存换到持久化时，改动点很有限。

如果你愿意，我可以把这篇博客再细化成一篇带时序图的长文，或者直接把 `InMemoryStore` 替换为 SQLite 实现并把测试脚本提交为示例。
