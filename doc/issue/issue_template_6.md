## 📌 模块概述
**目标**: 基于《[一个海量在线用户即时通讯系统（IM）的完整设计Plus](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)》实现 **单对单聊天功能 (Client-to-Client, C2C)**，对应文章第1.2.2.6节的 **C2C完整流程**。

> 📖 **架构参考**: 文章第1.2.2.6节 - "1.App1向gate1发送信息 → 2.Gate1将信息投递给logic → 3.Logic收到信息后存储 → ... → 9.App2向gate2发送ack"。

## ✨ 实现效果
- [ ] 用户A可以通过WebSocket发送消息给用户B
- [ ] 消息经过 Gate → Logic → Storage 完整链路处理
- [ ] 消息持久化到SQLite数据库（支持历史记录查询）
- [ ] 消息序号(Seq)全局自增且唯一
- [ ] 支持幂等性（通过ClientMsgID防重复）
- [ ] 发送者收到ServerMsgID确认（Ack机制）
- [ ] 接收者实时收到消息推送

## 🏗️ 架构定位（对应文章1.2.2.6节 C2C流程）
```
App1(用户A) ──→ Gate1 ──→ Logic ──→ SQLite(存储)
   │              │         │
   │ ① 发送消息    │ ② 转发    │ ③ 存储消息
   │              │         │
   │ ← ⑤ Ack     │ ← ④ 返回  │
   │ (ServerMsgID)│         │
   │                       │
   │              ┌────────┘
   │              │ ⑥ 查询用户B状态(Redis)
   │              │
   │              ↓ ⑦ 推送给Gate2
   │                              │
   │                         Gate2 ──→ App2(用户B)
   │                          │ ⑧ 推送消息    │
   │                          │           │ ⑨ 收到消息
   │                          │           │
   │                          │ ← ⑩ Ack  │
```

## 📋 实现步骤

### Step 1: 定义C2C消息服务接口
新建文件: `logic/c2c_service.go`

```go
package logic

import (
    "context"
    "your-project/storage"
)

// C2CService 单聊服务（对应文章msg-logic角色）
type C2CService struct {
    messageDAO *storage.MessageDAO // 数据访问对象
    userDAO    *storage.UserDAO
}

// SendC2CMessage 发送单聊消息（对应文章C2C流程步骤3-5）
func (s *C2CService) SendC2CMessage(ctx context.Context, req *SendC2CRequest) (*SendC2CReply, error) {
    // 1. 幂等性检查：根据ClientMsgID判断是否重复发送
    existingMsg, _ := s.messageDAO.GetByClientMsgID(req.ClientMsgID)
    if existingMsg != nil {
        // 消息已存在，直接返回之前的ServerMsgID（幂等性保证）
        return &SendC2CReply{
            ServerMsgID: existingMsg.ServerMsgID,
            Seq:        existingMsg.Seq,
        }, nil
    }

    // 2. 生成全局唯一的ServerMsgID和Seq（原子操作）
    serverMsgID, seq := generateMessageID() // 使用Snowflake算法或Redis INCR

    // 3. 构造消息对象并保存到SQLite
    msg := &storage.Message{
        ClientMsgID: req.ClientMsgID,
        ServerMsgID: serverMsgID,
        Seq:         seq,
        ChatType:    1, // 1-单聊
        ChatID:      generateChatID(req.From, req.To), // 会话ID
        SendID:      req.From,
        RecvID:      req.To,
        ContentType: req.ContentType,
        Content:     req.Content,
        Status:      0, // 0-发送中
    }

    if err := s.messageDAO.Save(msg); err != nil {
        return nil, err
    }

    // 4. 更新消息状态为成功
    s.messageDAO.UpdateStatus(serverMsgID, 1) // 1-成功

    // 5. 返回结果给Gate层（用于返回Ack给发送者）
    return &SendC2CReply{
        ServerMsgID: serverMsgID,
        Seq:        seq,
        ChatID:     msg.ChatID,
    }, nil
}

// GetC2CHistory 获取单聊历史消息（支持分页和时间线查询）
func (s *C2CService) GetC2CHistory(ctx context.Context, chatID string, limit, offset int) ([]*storage.Message, int64, error) {
    messages, total, err := s.messageDAO.GetByChatID(chatID, limit, offset)
    return messages, total, err
}
```

### Step 2: 实现消息ID生成器（全局唯一）
新建文件: `logic/id_generator.go`

```go
package logic

import (
    "sync/atomic"
    "time"
)

// MessageIDGenerator 消息ID生成器（基于Snowflake算法简化版）
type MessageIDGenerator struct {
    nodeID     int64
    sequence   uint64
    lastTime   int64
    mu         sync.Mutex
}

var globalIDGen *MessageIDGenerator

func InitIDGenerator(nodeID int64) {
    globalIDGen = &MessageIDGenerator{nodeID: nodeID}
}

// Generate 生成全局唯一的ServerMsgID和Seq
func (g *MessageIDGenerator) Generate() (serverMsgID uint64, seq uint64) {
    g.mu.Lock()
    defer g.mu.Unlock()

    now := time.Now().UnixMilli()
    if now == g.lastTime {
        g.sequence++
    } else {
        g.sequence = 0
        g.lastTime = now
    }

    // ServerMsgID: 时间戳(42bit) + 节点ID(10bit) + 序列号(12bit)
    serverMsgID = uint64((now << 22) | (g.nodeID << 12) | (g.sequence & 0xFFF))

    // Seq: 简化为全局递增计数器（生产环境建议使用Redis INCR）
    seq = atomic.AddUint64(&globalSeqCounter, 1)

    return
}
```

### Step 3: 在Logic层集成C2C服务
新建文件: `logic/service.go`（扩展）

```go
// LogicServer 逻辑层服务器（对应文章msg-logic角色）
type LogicServer struct {
    c2cService   *C2CService
    c2gService   *C2GService       // Issue11: 群聊服务
    authService  *auth.AuthService
    gateClient   GateClient        // 用于推送消息给Gate层
}

// HandleC2CMessage 处理单聊消息（被Gate层调用）
func (l *LogicServer) HandleC2CMessage(ctx context.Context, req *HandleC2CRequest) (*HandleC2CReply, error) {
    // 1. 调用C2C服务保存消息到SQLite
    result, err := l.c2cService.SendC2CMessage(ctx, &SendC2CRequest{
        From:        req.From,
        To:          req.To,
        ClientMsgID: req.ClientMsgID,
        ContentType: req.ContentType,
        Content:     req.Content,
    })
    if err != nil {
        return nil, err
    }

    // 2. 【关键】异步推送给接收者（对应文章C2C流程步骤7-8）
    go func() {
        pushCtx := context.Background()

        // 构造推送消息
        pushMsg := &protocol.Response{
            Code:        0,
            Msg:         "new_message",
            ServerMsgID: result.ServerMsgID,
            Data: map[string]interface{}{
                "chat_id":      result.ChatID,
                "from":         req.From,
                "content_type": req.ContentType,
                "content":      req.Content,
                "seq":          result.Seq,
            },
        }

        // 通过Gate客户端推送给接收者的Gate节点
        if err := l.gateClient.PushMessage(pushCtx, req.To, pushMsg); err != nil {
            log.Printf("[Logic] Failed to push message to user %s: %v", req.To, err)
            // TODO: 如果用户不在线，可以存入离线消息队列（Issue7: Redis/Kafka）
        }
    }()

    // 3. 返回结果给Gate（用于返回Ack给发送者）
    return &HandleC2CReply{
        ServerMsgID: result.ServerMsgID,
        Seq:        result.Seq,
        ChatID:     result.ChatID,
    }, nil
}
```

### Step 4: 编写单元测试
新建文件: `logic/c2c_service_test.go`

测试用例:
- 测试正常发送消息并返回ServerMsgID
- 测试相同ClientMsgID的幂等性（不重复插入）
- 测试消息Seq单调递增
- 测试查询历史消息（按会话ID分页）
- 测试消息内容完整性（发送方和接收方看到的内容一致）

## 🎯 参考资源
- **📄 架构参考文章**: [《一个海量在线用户即时通讯系统（IM）的完整设计Plus》](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)
  - 第1.2.2.6节：单对单聊天(C2C)完整流程图（11个步骤）
  - 第4节：离线消息拉取方式（TimeLine模型）
- **Snowflake算法**: https://github.com/bwmarrin/snowflake
- **前置依赖**: Issue #1 (SQLite存储) + Issue #5 (Gate接入层)

## 🔍 验收标准
1. ✅ 用户A发送消息后立即收到ServerMsgID和Seq（<100ms延迟）
2. ✅ 用户B在3秒内收到消息推送（在线场景）
3. ✅ 相同ClientMsgID不会产生重复消息（幂等性验证）
4. ✅ ServerMsgID全局唯一且按时间有序
5. ✅ 消息成功保存到SQLite数据库
6. ✅ 可以通过ChatID查询到完整的历史消息列表
7. ✅ 单元测试全部通过 (`go test ./logic/...`)

## ⚠️ 注意事项
- ⚠️ **性能**: 消息写入SQLite应使用批量插入或事务优化
- ⚠️ **可靠性**: 应考虑消息投递失败的重试机制（后续Issue7用Kafka增强）
- ⚠️ **顺序性**: 同一会话内的消息必须严格按Seq排序
- ⚠️ **安全性**: 应检查发送者和接收者是否为好友关系（可选）

## 📊 工作量评估
- 预计耗时: 2-3天
- 复杂度: ⭐⭐⭐⭐ (核心业务逻辑)
- 依赖:
  - **必须**: Issue #1 (SQLite存储)
  - **必须**: Issue #5 (Gate接入层 - 提供调用入口)
  - **建议**: Issue #7 (Redis - 用户在线状态查询)

---
**所属阶段**: 第2周 - 核心业务逻辑（对应文章1.2.2.6节）
**优先级**: P0 (核心功能 - IM系统的基本能力)
