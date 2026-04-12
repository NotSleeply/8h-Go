# Issue40 交付记录: Logic 逻辑层抽取（业务核心边界）

## 1. 问题描述

Issue #40 要求明确 Gate/Logic/Storage 边界，避免网络层代码直接耦合业务决策与持久化细节。  
在改造前，`server.go` 同时承担连接处理、业务判断、ID/序号分配和存储调用，职责偏重。

## 2. 技术难点

- 不能破坏已有 TCP 接入与消息投递链路
- 需要在不引入额外服务依赖的情况下形成“逻辑层边界”
- 要兼容现有幂等、事务保存和 ACK 流程

## 3. 解决方案

### 3.1 新增 `LogicService`

新增文件: `src/server/logic.go`

`LogicService` 负责：

- 统一生成 `ServerMsgID`（`GenerateServerMsgID(prefix)`）
- `ProcessSend`:
  - 幂等判断（`from + client_msg_id`）
  - chat_id 归一化（C2C chatID）
  - seq 分配
  - 调用 `store.SaveMessageWithRecipients` 完成事务写入
- 处理 `deliver_ack` 和 `read_ack` 的状态更新

### 3.2 Server 仅负责接入编排

`src/server/server.go` 调整为：

- 连接层只做：收包、解析、调用逻辑层、回包、入投递队列
- `HandleClientSend` 改为调用 `logic.ProcessSend`
- `HandleDeliverAck`/`HandleReadAck` 改为委托 logic
- 系统消息 ID 也统一走 logic 生成器

## 4. 交付效果

- 业务逻辑与接入层解耦程度明显提升
- 逻辑层可以继续向 gRPC 服务演进（接口边界已具雏形）
- 与 Storage 的事务边界保持一致（仍通过 store 封装写入）

## 5. 代码变更

- 新增: `src/server/logic.go`
- 修改: `src/server/server.go`

## 6. 验证结果

- `go test ./...` 通过
- `go build .` 通过
- 现有 C2C、ACK、离线恢复链路保持可用

## 7. 后续建议

- 将 `LogicService` 抽为独立进程并暴露 gRPC 接口（对应后续 issue）
- 为 `LogicService` 增加单元测试，覆盖幂等与事务失败分支
