# Issue43 交付记录: gRPC 通讯框架（Protobuf + Logic RPC）

## 1. 问题描述

Issue #43 要求确定并落地服务间通信框架。  
本次采用 **gRPC + Protobuf**，为 Gate/Logic 的内部调用建立清晰协议层。

## 2. 技术难点

- 需要兼容当前代码结构，避免大规模破坏既有 TCP 路径
- 需要把 proto 生成流程标准化，避免手工维护 pb 文件
- 需要在逻辑层接口化与现有业务流程之间保持一致

## 3. 解决方案

### 3.1 Proto 定义与版本位置

- 新增 `proto/im.proto`
- package: `im.v1`
- go_package: `tet/src/rpc/pb;pb`
- 目前暴露 RPC:
  - `SendMessage`
  - `AckDelivery`
  - `AckRead`

### 3.2 代码生成流程

- 新增 `scripts/gen_proto.ps1`
- 使用 `protoc + protoc-gen-go + protoc-gen-go-grpc`
- 生成产物固定到 `src/rpc/pb/*.pb.go`
- 支持开发机重复执行，避免路径漂移

### 3.3 gRPC 服务实现

- 新增 `src/rpc/logic_server.go`
  - `SendMessage` 调用现有 `LogicService.ProcessSend`
  - `AckDelivery` / `AckRead` 调用逻辑层回执接口
- 新增 `cmd/grpcserver/main.go`
  - 独立启动 gRPC 服务
  - 默认地址 `127.0.0.1:50051`（`IM_GRPC_ADDR` 可配置）

### 3.4 逻辑层对接

- `server.Server` 暴露 `Logic()` 访问器，供 RPC 层调用
- 维持既有消息持久化、投递、回执语义一致

## 4. 代码变更

- 新增: `proto/im.proto`
- 新增: `scripts/gen_proto.ps1`
- 新增: `src/rpc/pb/im.pb.go`
- 新增: `src/rpc/pb/im_grpc.pb.go`
- 新增: `src/rpc/logic_server.go`
- 新增: `cmd/grpcserver/main.go`
- 修改: `src/server/server.go`
- 依赖更新: `google.golang.org/grpc`, `google.golang.org/protobuf`

## 5. 验证结果

- `go test ./...` 通过
- `go build ./...` 通过
- `cmd/grpcserver` 可正常编译并提供 RPC 服务入口

## 6. 后续建议

- 增加 gRPC 客户端示例与集成测试（Gate -> Logic）
- 增加流式接口（双向流）支持高并发推送场景
- 将 proto 生成纳入 CI（防止 pb 文件与 proto 定义漂移）
