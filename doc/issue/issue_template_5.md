## 📌 模块概述
**目标**: 基于《[一个海量在线用户即时通讯系统（IM）的完整设计Plus](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)》构建 **Gate 接入层 (Gateway/Access Layer)**，对应文章第1.1.3节的 **接入层** 设计。

> 📖 **架构参考**: 文章第1.1.3节 - "接入层主要任务是保持海量用户连接（接入）、攻击防护、将海量连接整流成少量TCP连接与逻辑层通讯"。

## ✨ 实现效果
- [ ] 统一管理所有WebSocket客户端连接（基于Issue4的Hub）
- [ ] 实现**消息路由转发**：将客户端消息转发给Logic层处理
- [ ] 实现**消息投递**：将Logic层的响应推送给目标客户端
- [ ] 支持**会话管理**：维护用户ID到Client的映射关系
- [ ] 支持**连接限流和IP黑名单**（基础攻击防护）
- [ ] 提供**统计接口**：在线用户数、消息吞吐量等监控数据

## 🏗️ 架构定位（对应文章1.1.3节）
```
┌─────────────────────────────────────┐
│        用户端 (Web Browser)          │  ← H5页面 (文章1.1.2节)
└──────────────┬──────────────────────┘
               │ WebSocket/TCP
    ┌──────────▼──────────┐
    │   Gate 接入层 (本模块) │  ← 文章1.1.3节：连接管理+攻击防护+整流
    │  ┌────────────────┐  │
    │  │ Connection Mgr  │  ← 保持海量连接
    │  │ Message Router  │  ← 整流：多→少
    │  │ Security Filter │  ← 攻击防护
    │  └───────┬────────┘  │
    └──────────┼──────────┘
               │ gRPC/本地调用
    ┌──────────▼──────────┐
    │     Logic 逻辑层      │  ← 业务逻辑处理
    └─────────────────────┘
```

## 📋 实现步骤

### Step 1: 创建 Gate 核心结构
新建文件: `gate/gate.go`

```go
package gate

import (
    "context"
    "log"
    "sync"
    "time"
)

// Gate 接入层核心（对应文章msg-gate角色）
type Gate struct {
    hub           *Hub                    // 连接管理中心（来自Issue4）
    userSessions  map[string]*UserSession  // 用户会话表（UID -> Session）
    sessionMu     sync.RWMutex
    msgHandler    LogicClient              // Logic层客户端接口（后续用gRPC实现）
    metrics       *Metrics                 // 统计指标
}

// UserSession 用户会话信息
type UserSession struct {
    UserID      string
    Client      *Client       // WebSocket连接
    LoginTime   time.Time
    LastActive  time.Time
    DeviceType  string        // 设备类型：web/mobile
    IP          string        // 客户端IP
}

// Metrics 统计指标
type Metrics struct {
    TotalConnections int64
    OnlineUsers      int64
    MessagesReceived int64
    MessagesSent     int64
    mu               sync.RWMutex
}

func NewGate(hub *Hub, logicClient LogicClient) *Gate {
    return &Gate{
        hub:          hub,
        userSessions: make(map[string]*UserSession),
        msgHandler:   logicClient,
        metrics:      &Metrics{},
    }
}
```

### Step 2: 实现用户会话管理
在 `gate/gate.go` 中添加：

```go
// OnUserLogin 用户登录成功后的回调（对应文章Auth流程步骤6-8）
func (g *Gate) OnUserLogin(userID string, client *Client, deviceType, ip string) error {
    g.sessionMu.Lock()
    defer g.sessionMu.Unlock()

    // 1. 检查是否已有其他设备登录（对应文章Kickout流程）
    if oldSession, exists := g.userSessions[userID]; exists {
        log.Printf("[Gate] Kickout user %s from old device", userID)

        // 发送踢人消息给旧设备
        kickoutMsg := map[string]interface{}{
            "msg_type": 99, // 自定义：踢人消息
            "reason":  "login_from_other_device",
        }
        oldSession.Client.send <- mustMarshal(kickoutMsg)

        // 关闭旧连接
        close(oldSession.Client.send)
        g.hub.unregister <- oldSession.Client
    }

    // 2. 创建新会话
    session := &UserSession{
        UserID:     userID,
        Client:     client,
        LoginTime:  time.Now(),
        LastActive: time.Now(),
        DeviceType: deviceType,
        IP:         ip,
    }
    g.userSessions[userID] = session

    // 3. 更新统计
    g.metrics.mu.Lock()
    g.metrics.OnlineUsers = int64(len(g.userSessions))
    g.metrics.mu.Unlock()

    // 4. 通知Logic层用户上线（用于路由表更新）
    go g.msgHandler.NotifyUserOnline(context.Background(), userID)

    log.Printf("[Gate] User %s logged in from %s (%s), total online: %d",
        userID, ip, deviceType, len(g.userSessions))

    return nil
}

// OnUserLogout 用户登出回调（对应文章Logout流程）
func (g *Gate) OnUserLogout(userID string) {
    g.sessionMu.Lock()
    defer g.sessionMu.Unlock()

    if session, exists := g.userSessions[userID]; exists {
        delete(g.userSessions, userID)
        g.hub.unregister <- session.Client

        g.metrics.mu.Lock()
        g.metrics.OnlineUsers = int64(len(g.userSessions))
        g.metrics.mu.Unlock()

        // 通知Logic层用户离线
        go g.msgHandler.NotifyUserOffline(context.Background(), userID)

        log.Printf("[Gate] User %s logged out, total online: %d",
            userID, len(g.userSessions))
    }
}

// GetUserSession 获取用户会话
func (g *Gate) GetUserSession(userID string) (*UserSession, bool) {
    g.sessionMu.RLock()
    defer g.sessionMu.RUnlock()
    session, ok := g.userSessions[userID]
    return session, ok
}
```

### Step 3: 实现消息路由器（Message Router）
新建文件: `gate/router.go`

```go
package gate

import (
    "encoding/json"
    "log"
    "your-project/protocol"
)

// MessageHandler 消息处理器接口（由Gate调用Logic层）
type MessageHandler interface {
    // OnMessage 处理客户端消息（转发给Logic层）
    OnMessage(client *Client, rawMsg []byte)

    // PushMessage 推送消息给指定用户（Logic层调用此方法）
    PushMessage(ctx context.Context, userID string, msg *protocol.Response) error

    // BroadcastMessage 广播消息给所有在线用户（群聊场景）
    BroadcastMessage(ctx context.Context, userIDs []string, msg *protocol.Response) error
}

// DefaultMessageHandler 默认消息处理器实现
type DefaultMessageHandler struct {
    gate *Gate
}

func (h *DefaultMessageHandler) OnMessage(client *Client, rawMsg []byte) {
    // 1. 解析消息
    var msg protocol.Message
    if err := json.Unmarshal(rawMsg, &msg); err != nil {
        log.Printf("[Gate] Invalid message format: %v", err)
        return
    }

    // 2. 更新活跃时间
    if session, ok := h.gate.GetUserSession(client.userID); ok {
        session.LastActive = time.Now()
    }

    // 3. 更新统计
    h.gate.metrics.mu.Lock()
    h.gate.metrics.MessagesReceived++
    h.gate.metrics.mu.Unlock()

    // 4. 根据消息类型路由到不同处理器
    switch msg.MsgType {
    case 1: // 登录认证
        h.handleAuth(client, &msg)
    case 2: // 心跳
        h.handleHeartbeat(client, &msg)
    case 3: // 单聊消息（C2C）
        h.handleC2CMessage(client, &msg)
    case 4: // 群聊消息（C2G）
        h.handleC2GMessage(client, &msg)
    case 5: // Ack确认
        h.handleAck(client, &msg)
    default:
        log.Printf("[Gate] Unknown message type: %d", msg.MsgType)
    }
}

func (h *DefaultMessageHandler) handleAuth(client *Client, msg *protocol.Message) {
    // 调用Auth服务验证Token（此处为简化版，实际应调用Logic层）
    claims, err := auth.ParseToken(msg.Token)
    if err != nil {
        sendError(client, 401, "invalid token")
        return
    }

    // 设置客户端用户信息
    client.userID = claims.UserID

    // 注册用户会话（触发Kickout检查）
    if err := h.gate.OnUserLogin(claims.UserID, client, "web", client.conn.RemoteAddr().String()); err != nil {
        sendError(client, 500, "login failed")
        return
    }

    // 返回登录成功响应
    resp := &protocol.Response{
        Code: 0,
        Msg:  "ok",
        Data: map[string]string{
            "user_id":   claims.UserID,
            "user_name": claims.UserName,
        },
    }
    client.send <- mustMarshal(resp)
}

func (h *DefaultMessageHandler) handleC2CMessage(client *Client, msg *protocol.Message) {
    // 转发给Logic层处理单聊业务
    ctx := context.Background()
    result, err := h.gate.msgHandler.HandleC2CMessage(ctx, &HandleC2CRequest{
        From:        client.userID,
        To:          msg.RecvID,
        ClientMsgID: msg.ClientMsgID,
        ContentType: msg.ContentType,
        Content:     msg.Content,
    })

    if err != nil {
        sendError(client, 500, err.Error())
        return
    }

    // 返回Ack给发送者（对应文章C2C流程步骤5）
    resp := &protocol.Response{
        Code:        0,
        Msg:         "ok",
        ServerMsgID: result.ServerMsgID,
    }
    client.send <- mustMarshal(resp)
}

// ... 其他handle方法略
```

### Step 4: 实现安全过滤器（基础攻击防护）
新建文件: `gate/security.go`

```go
package gate

import (
    "sync"
    "time"
)

// SecurityFilter 安全过滤器（对应文章攻击防护功能）
type SecurityFilter struct {
    rateLimiter    map[string]*RateBucket  // IP级别限流
    blacklistedIPs map[string]bool         // IP黑名单
    mu             sync.RWMutex
}

type RateBucket struct {
    count    int
    resetTime time.Time
}

func NewSecurityFilter() *SecurityFilter {
    return &SecurityFilter{
        rateLimiter:    make(map[string]*RateBucket),
        blacklistedIPs: make(map[string]bool),
    }
}

// CheckRateLimit 检查IP频率限制（每秒最多100个请求）
func (sf *SecurityFilter) CheckRateLimit(ip string) bool {
    sf.mu.Lock()
    defer sf.mu.Unlock()

    now := time.Now()
    bucket, exists := sf.rateLimiter[ip]
    if !exists || now.After(bucket.resetTime) {
        sf.rateLimiter[ip] = &RateBucket{
            count:     1,
            resetTime: now.Add(time.Second),
        }
        return true
    }

    if bucket.count >= 100 { // 限制：100 req/sec
        return false
    }
    bucket.count++
    return true
}

// IsBlacklisted 检查IP是否在黑名单中
func (sf *SecurityFilter) IsBlacklisted(ip string) bool {
    sf.mu.RLock()
    defer sf.mu.RUnlock()
    return sf.blacklistedIPs[ip]
}
```

### Step 5: 集成到 main.go
```go
func main() {
    // ... 初始化Storage、Auth等

    // 1. 初始化Hub（来自Issue4）
    hub := gate.NewHub()
    go hub.Run()

    // 2. 初始化Logic客户端（先用本地调用，后续Issue12改为gRPC）
    logicClient := logic.NewLocalClient()

    // 3. 初始化Gate
    securityFilter := gate.NewSecurityFilter()
    g := gate.NewGate(hub, logicClient)

    // 4. 启动WebSocket服务器
    msgHandler := &gate.DefaultMessageHandler{Gate: g}
    wsServer := gate.NewWSServer(":9000", msgHandler)
    go wsServer.Start()

    log.Println("[System] Gate layer started")
}
```

### Step 6: 编写单元测试
新建文件: `gate/gate_test.go`
新建文件: `gate/security_test.go`

测试用例:
- 测试用户登录和会话创建
- 测试踢人机制（同账号第二次登录）
- 测试用户登出和会话清理
- 测试消息路由到正确的处理器
- 测试IP频率限制生效
- 测试IP黑名单拦截

## 🎯 参考资源
- **📄 架构参考文章**: [《一个海量在线用户即时通讯系统（IM）的完整设计Plus》](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)
  - 第1.1.3节：接入层三大职责（连接管理、攻击防护、整流）
  - 第1.2.2.1节：Auth流程（Gate接收验证请求并返回结果）
  - 第1.2.2.3节：Kickout流程（Gate向旧设备发送kickout命令）
- **前置依赖**: Issue #4 (WebSocket协议) - 必须先完成Hub和Client

## 🔍 验收标准
1. ✅ 用户通过WS登录后，Gate正确创建UserSession并维护映射
2. ✅ 同一账号第二次登录时，旧设备收到踢人消息并被断开
3. ✅ 用户登出后，Session被清理且通知Logic层
4. ✅ 收到的消息根据MsgType正确路由到对应的handle方法
5. ✅ 单个IP超过100req/s后被限流器拦截
6. ✅ 黑名单IP无法建立连接
7. ✅ 在线用户数统计准确
8. ✅ 单元测试全部通过 (`go test ./gate/...`)

## ⚠️ 注意事项
- ⚠️ **并发安全**: 所有对userSessions的操作必须加锁
- ⚠️ **性能优化**: 高并发场景下，userSessions可考虑分片或使用sync.Map
- ⚠️ **扩展性**: 当前版本使用内存存储Session，生产环境建议持久化到Redis
- ⚠️ **安全性**: 应添加更完善的防护（如WAF、DDoS防护等）

## 📊 工作量评估
- 预计耗时: 3-4天
- 复杂度: ⭐⭐⭐⭐ (核心组件，涉及并发和安全)
- 依赖:
  - **必须**: Issue #4 (WebSocket协议 - Hub/Client)
  - **必须**: Issue #2 (认证系统 - JWT验证)
  - **建议**: Issue #7 (Redis - Session持久化)

---
**所属阶段**: 第2周 - 接入层核心（对应文章1.1.3节）
**优先级**: P0 (架构基础 - 所有消息的出入口)
