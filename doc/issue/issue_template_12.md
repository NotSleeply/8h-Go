## 📌 模块概述
**目标**: 引入 **gRPC框架** 实现各微服务之间的高性能RPC通讯，替代本地函数调用，为真正的分布式架构奠定基础。

> 🔧 **技术选型**: 使用 **gRPC + Protobuf** 替代 rpcx，原因：
> - **标准化**: Protobuf是业界标准的IDL（接口定义语言）
> - **高性能**: 基于HTTP/2，支持双向流和头部压缩
> - **多语言原生支持**: Go/Java/Python/Node.js等
> - **生态完善**: gRPC Gateway可同时提供RESTful API
> - **Google背书**: 云原生CNCF毕业项目

## ✨ 实现效果
- [ ] 定义清晰的 `.proto` 接口文件（IDL）
- [ ] Logic层提供gRPC Server供Gate/API层调用
- [ ] Gate层通过gRPC Client调用Logic层
- [ ] 支持流式传输（用于实时消息推送）
- [ ] 集成服务发现（etcd或Consul）

## 🏗️ 架构定位
```
┌──────────────┐         ┌──────────────┐
│   Gate 层     │  gRPC   │   Logic 层    │
│  (Client)    │ ←────→  │  (Server)    │
│              │         │              │
│ gRPC Client  │         │ gRPC Server  │
└──────────────┘         └──────────────┘

Protobuf IDL 定义 (.proto 文件):
├── proto/im.proto          # IM核心接口定义
│   ├── service LogicService
│   │   ├── SendMessage (C2C)
│   │   ├── SendGroupMessage (C2G)
│   │   ├── Authenticate (Auth)
│   │   └── GetUserStatus
│   └── message types...
```

## 📋 实现步骤

### Step 1: 安装依赖
```bash
# 安装protoc编译器（如果未安装）
# Windows: choco install protoc
# Mac: brew install protobuf

# 安装Go的protobuf插件和gRPC库
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

go get google.golang.org/grpc
go get google.golang.org/protobuf
```

### Step 2: 定义Protobuf接口文件
新建文件: `proto/im.proto`

```protobuf
syntax = "proto3";

package im;

option go_package = "./pb";

import "google/protobuf/timestamp.proto";

// ==================== 服务定义 ====================

// LogicService IM逻辑层服务接口
service LogicService {
    // 单聊：发送消息
    rpc SendMessage(SendMessageRequest) returns (SendMessageReply);

    // 群聊：发送群消息
    rpc SendGroupMessage(SendGroupMessageRequest) returns (SendMessageReply);

    // 认证：验证Token并返回用户信息
    rpc Authenticate(AuthenticateRequest) returns (AuthenticateReply);

    // 查询：获取用户在线状态
    rpc GetUserStatus(GetUserStatusRequest) returns (GetUserStatusReply);

    // 推送：服务端主动推送消息（双向流）
    rpc MessageStream(stream ClientMessage) returns (stream ServerMessage);
}

// ==================== 消息类型 ====================

message SendMessageRequest {
    string from_user_id = 1;
    string to_user_id = 2;
    string client_msg_id = 3;
    int32 content_type = 4;  // 1-文本 2-图片 ...
    string content = 5;
}

message SendMessageReply {
    int32 code = 1;
    string message = 2;
    uint64 server_msg_id = 3;
    uint64 seq = 4;
    string chat_id = 5;
}

message SendGroupMessageRequest {
    string from_user_id = 1;
    string room_id = 2;
    string client_msg_id = 3;
    int32 content_type = 4;
    string content = 5;
}

message AuthenticateRequest {
    string token = 1;
}

message AuthenticateReply {
    int32 code = 1;
    string message = 2;
    UserInfo user_info = 3;
}

message UserInfo {
    string user_id = 1;
    string user_name = 2;
    string nick_name = 3;
    string avatar_url = 4;
}

message GetUserStatusRequest {
    string user_id = 1;
}

message GetUserStatusReply {
    bool is_online = 1;
    string gate_address = 2;  // 用户所在的Gate节点地址
    google.protobuf.Timestamp last_active_time = 3;
}

// 流式消息类型
message ClientMessage {
    int32 msg_type = 1;  // 1-心跳 2-Ack
    oneof payload {
        HeartbeatMessage heartbeat = 2;
        AckMessage ack = 3;
    }
}

message ServerMessage {
    int32 msg_type = 1;  // 1-新消息 2-通知
    oneof payload {
        PushMessage push_msg = 2;
        Notification notification = 3;
    }
}

message HeartbeatMessage {
    int64 timestamp = 1;
}

message AckMessage {
    uint64 server_msg_id = 1;
    int32 status = 2;  // 1-已送达 2-已读
}

message PushMessage {
    uint64 server_msg_id = 1;
    string chat_id = 2;
    string from_user_id = 3;
    int32 content_type = 4;
    string content = 5;
    uint64 seq = 6;
    google.protobuf.Timestamp create_time = 7;
}

message Notification {
    int32 type = 1;  // 1-踢人 2-群邀请
    string content = 2;
}
```

### Step 3: 生成Go代码
```bash
# 在项目根目录执行
protoc --go_out=. --go-grpc_out=. proto/im.proto
```

生成的文件:
- `pb/im.pb.go` - 消息类型的Go结构体
- `pb/im_grpc.pb.go` - gRPC服务端和客户端接口

### Step 4: 实现gRPC Server（Logic层）
新建文件: `logic/grpc_server.go`

```go
package logic

import (
    "context"
    "your-project/pb"
    "google/grpc"
)

// LogicGRPCServer Logic层的gRPC服务器实现
type LogicGRPCServer struct {
    pb.UnimplementedLogicServiceServer
    logicServer *LogicServer // 业务逻辑实例
}

func NewLogicGRPCServer(logicServer *LogicServer) *LogicGRPCServer {
    return &LogicGRPCServer{logicServer: logicServer}
}

// SendMessage 实现单聊接口
func (s *LogicGRPCServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageReply, error) {
    result, err := s.logicServer.HandleC2CMessage(ctx, &HandleC2CRequest{
        From:        req.FromUserId,
        To:          req.ToUserId,
        ClientMsgID: req.ClientMsgId,
        ContentType: int(req.ContentType),
        Content:     req.Content,
    })

    if err != nil {
        return &pb.SendMessageReply{
            Code:    500,
            Message: err.Error(),
        }, err
    }

    return &pb.SendMessageReply{
        Code:       0,
        Message:    "ok",
        ServerMsgId: result.ServerMsgID,
        Seq:        result.Seq,
        ChatId:     result.ChatID,
    }, nil
}

// Authenticate 实现认证接口
func (s *LogicGRPCServer) Authenticate(ctx context.Context, req *pb.AuthenticateRequest) (*pb.AuthenticateReply, error) {
    claims, err := auth.ParseToken(req.Token)
    if err != nil {
        return &pb.AuthenticateReply{
            Code:    401,
            Message: "invalid token",
        }, nil
    }

    user, _ := storage.GetUserByID(uint(claims.UserID))

    return &pb.AuthenticateReply{
        Code:    0,
        Message: "ok",
        UserInfo: &pb.UserInfo{
            UserId:   claims.UserId,
            UserName: claims.UserName,
            NickName: user.NickName,
            AvatarUrl: user.AvatarURL,
        },
    }, nil
}

// 启动gRPC服务器
func StartGRPCServer(addr string, server *LogicGRPCServer) error {
    lis, err := net.Listen("tcp", addr)
    if err != nil {
        return err
    }

    s := grpc.NewServer()
    pb.RegisterLogicServiceServer(s, server)

    log.Printf("[Logic] gRPC server listening on %s", addr)
    return s.Serve(lis)
}
```

### Step 5: 实现gRPC Client（Gate层）
新建文件: `gate/grpc_client.go`

```go
package gate

import (
    "context"
    "your-project/pb"
    "google/grpc"
    "time"
)

// LogicGRPCClient Gate层的gRPC客户端（调用Logic层）
type LogicGRPCClient struct {
    conn   *grpc.ClientConn
    client pb.LogicServiceClient
}

func NewLogicGRPCClient(addr string) (*LogicGRPCClient, error) {
    conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock())
    if err != nil {
        return nil, err
    }

    return &LogicGRPCClient{
        conn:   conn,
        client: pb.NewLogicServiceClient(conn),
    }, nil
}

// SendMessage 调用Logic层的单聊接口
func (c *LogicGRPCClient) SendMessage(ctx context.Context, req *SendMessageRequest) (*SendMessageReply, error) {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    return c.client.SendMessage(ctx, &pb.SendMessageRequest{
        FromUserId:   req.From,
        ToUserId:     req.To,
        ClientMsgId:  req.ClientMsgID,
        ContentType: int32(req.ContentType),
        Content:      req.Content,
    })
}
```

### Step 6: 编写单元测试
测试用例:
- 测试gRPC连接建立
- 测试SendMessage RPC调用成功
- 测试Authenticate Token验证
- 测试超时和错误处理

## 🎯 参考资源
- **gRPC官方文档**: https://grpc.io/docs/
- **Protobuf文档**: https://developers.google.com/protocol-buffers
- **Go gRPC示例**: https://grpc.io/docs/languages/go/quickstart/
- **前置依赖**: Issue #9 (Logic层完整实现)

## 🔍 验收标准
1. ✅ 可以通过 `protoc` 成功生成Go代码
2. ✅ Logic层gRPC Server启动监听 :50051
3. ✅ Gate层可以通过gRPC成功调用Logic层的SendMessage方法
4. ✅ gRPC调用延迟 < 10ms（本地环境）
5. ✅ 支持流式传输（MessageStream）
6. ✅ 单元测试全部通过 (`go test ./...`)

## ⚠️ 注意事项
- ⚠️ **版本兼容性**: protoc编译器和protoc-gen-go版本必须匹配
- ⚠️ **安全性**: 生产环境应启用TLS加密（grpc.WithTransportCredentials）
- ⚠️ **负载均衡**: 可结合gRPC的负载均衡策略
- ⚠️ **监控**: 应集成Prometheus监控gRPC指标（延迟/QPS/错误率）

## 📊 工作量评估
- 预计耗时: 3-4天
- 复杂度: ⭐⭐⭐⭐ (涉及协议设计和网络编程)
- 依赖:
  - **必须**: Issue #9 (Logic层必须先完成业务逻辑)
  - **建议**: Issue #11 (etcd服务发现 - 用于gRPC服务注册)

---
**所属阶段**: 第4周 - 分布式通讯架构
**优先级**: P0 (核心架构升级 - 微服务化的基础)
