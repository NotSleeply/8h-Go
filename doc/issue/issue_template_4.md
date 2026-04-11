## 📌 模块概述

**目标**: 基于《[一个海量在线用户即时通讯系统（IM）的完整设计Plus](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)》实现 **WebSocket 协议支持**，对应文章第1.1.2节的 **用户端API（H5页面提供WebSocket接口）**。

> 📖 **架构参考**: 文章第1.1.2节 - "对于H5页面，提供WebSocket接口"。我们专注于Web端实现，不需要iOS/Android SDK。

## ✨ 实现效果

- [ ] 服务端同时监听 WebSocket 端口（默认 :9000）
- [ ] 支持 WebSocket 握手和连接管理
- [ ] 实现 WebSocket 消息编解码（JSON格式）
- [ ] 支持心跳检测（Ping/Pong）
- [ ] 支持断线自动重连机制
- [ ] 与现有TCP协议共存（双协议接入）

## 🏗️ 架构定位

```
┌──────────────────────────────────────┐
│         用户端 (Web Browser)          │
│  ┌──────────┐  ┌──────────────────┐  │
│  │ React UI │─→│ WebSocket Client │  │
│  └──────────┘  └────────┬─────────┘  │
└────────────────────────────────┼──────┘
                                 │ WS://
                    ┌────────────▼────────────┐
                    │   Gate 接入层 (本模块)     │
                    │  ┌────────┐ ┌─────────┐  │
                    │  │WS Handler│ │TCP Handler│ │
                    │  └────────┘ └─────────┘  │
                    └────────────┬────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │      Logic 逻辑层         │
                    └──────────────────────────┘
```

## 📋 实现步骤

### Step 1: 安装依赖

```bash
go get github.com/gorilla/websocket
```

### Step 2: 创建 WebSocket Server

新建文件: `gate/ws_server.go`

```go
package gate

import (
    "github.com/gorilla/websocket"
    "log"
    "net/http"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // 开发环境允许所有来源
    },
}

// WSServer WebSocket服务器
type WSServer struct {
    server     *http.Server
    hub        *Hub       // 连接管理中心
    msgHandler MessageHandler // 消息处理器（调用Logic层）
}

func NewWSServer(addr string, handler MessageHandler) *WSServer {
    hub := NewHub()
    go hub.Run()

    mux := http.NewServeMux()
    ws := &WSServer{
        hub:        hub,
        msgHandler: handler,
    }

    // WebSocket端点
    mux.HandleFunc("/ws", ws.handleWebSocket)

    // 健康检查端点
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("OK"))
    })

    ws.server = &http.Server{
        Addr:    addr,
        Handler: mux,
    }

    return ws
}

func (ws *WSServer) Start() error {
    log.Printf("[Gate] WebSocket server listening on %s", ws.server.Addr)
    return ws.server.ListenAndServe()
}
```

### Step 3: 实现连接管理器（Hub模式）

新建文件: `gate/hub.go`

```go
package gate

import (
    "sync"
)

// Hub 连接管理中心（管理所有WebSocket客户端）
type Hub struct {
    clients    map[*Client]bool // 所有在线客户端
    register   chan *Client     // 注册通道
    unregister chan *Client     // 注销通道
    broadcast  chan []byte      // 广播通道（用于群聊）
    mu         sync.RWMutex
}

func NewHub() *Hub {
    return &Hub{
        clients:    make(map[*Client]bool),
        register:   make(chan *Client),
        unregister: make(chan *Client),
        broadcast:  make(chan []byte),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.mu.Lock()
            h.clients[client] = true
            h.mu.Unlock()
            log.Printf("[Gate] Client connected: %s, total: %d",
                client.userID, len(h.clients))

        case client := <-h.unregister:
            h.mu.Lock()
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
                log.Printf("[Gate] Client disconnected: %s, total: %d",
                    client.userID, len(h.clients))
            }
            h.mu.Unlock()

        case message := <-h.broadcast:
            h.mu.RLock()
            for client := range h.clients {
                select {
                case client.send <- message:
                default:
                    close(client.send)
                    delete(h.clients, client)
                }
            }
            h.mu.RUnlock()
        }
    }
}

// GetOnlineUsers 获取在线用户列表
func (h *Hub) GetOnlineUsers() []string {
    h.mu.RLock()
    defer h.mu.RUnlock()

    users := make([]string, 0, len(h.clients))
    for client := range h.clients {
        users = append(users, client.userID)
    }
    return users
}

// GetClientByUserID 根据UserID获取客户端连接
func (h *Hub) GetClientByUserID(userID string) *Client {
    h.mu.RLock()
    defer h.mu.RUnlock()

    for client := range h.clients {
        if client.userID == userID {
            return client
        }
    }
    return nil
}
```

### Step 4: 实现 Client 结构体

新建文件: `gate/client.go`

```go
package gate

import (
    "github.com/gorilla/websocket"
    "log"
    "time"
)

// Client 表示一个WebSocket客户端连接
type Client struct {
    hub      *Hub
    conn     *websocket.Conn
    send     chan []byte          // 发送缓冲区
    userID   string               // 用户ID（认证后设置）
    token    string               // JWT Token
    lastPing time.Time            // 最后心跳时间
}

func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()

    c.conn.SetReadLimit(512 * 1024) // 最大消息512KB
    c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        c.lastPing = time.Now()
        return nil
    })

    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err,
                websocket.CloseGoingAway,
                websocket.CloseAbnormalClosure) {
                log.Printf("[Gate] Read error: %v", err)
            }
            break
        }

        // 解析消息并转发给Logic层处理
        c.hub.msgHandler.OnMessage(c, message)
    }
}

func (c *Client) writePump() {
    ticker := time.NewTicker(30 * time.Second) // 每30秒发送心跳
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()

    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }

            // 写入消息
            if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
                return
            }

        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            // 发送Ping帧
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

### Step 5: 实现消息协议定义

新建文件: `protocol/message.go`

```go
package protocol

// Message WebSocket消息协议格式
type Message struct {
    // 基础字段
    MsgType    int    `json:"msg_type"`    // 消息类型：1-登录 2-心跳 3-单聊 4-群聊 5-Ack
    Seq        int64  `json:"seq"`         // 客户端序号
    ClientMsgID string `json:"client_msg_id"` // 客户端消息ID（幂等）

    // 认证字段（MsgType=1时使用）
    UID   string `json:"uid,omitempty"`
    Token string `json:"token,omitempty"`

    // 聊天字段（MsgType=3/4时使用）
    ChatID      string `json:"chat_id,omitempty"`      // 会话ID
    RecvID      string `json:"recv_id,omitempty"`      // 接收者ID
    ContentType int    `json:"content_type,omitempty"`  // 内容类型：1-文本
    Content     string `json:"content,omitempty"`       // 消息内容

    // Ack字段（MsgType=5时使用）
    ServerMsgID uint64 `json:"server_msg_id,omitempty"` // 服务端消息ID
}

// Response 服务端响应格式
type Response struct {
    Code    int         `json:"code"`    // 0-成功 其他-错误码
    Msg     string      `json:"msg"`     // 错误信息
    Data    interface{} `json:"data"`    // 数据载荷
    ServerMsgID uint64  `json:"server_msg_id,omitempty"` // 新消息的服务端ID
}
```

### Step 6: 集成到 main.go

```go
func main() {
    // ... 初始化Storage、Auth等

    // 启动WebSocket服务（Gate层）
    wsServer := gate.NewWSServer(":9000", messageHandler)
    go func() {
        if err := wsServer.Start(); err != nil && err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    log.Println("[System] All services started")
    select {} // 阻塞主goroutine
}
```

### Step 7: 编写单元测试

新建文件: `gate/hub_test.go`
新建文件: `gate/client_test.go`

测试用例:

- 测试客户端注册和注销
- 测试消息广播功能
- 测试在线用户查询
- 测试心跳超时断开

## 🎯 参考资源

- **📄 架构参考文章**: [《一个海量在线用户即时通讯系统（IM）的完整设计Plus》](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)
  - 第1.1.2节：用户端API（H5页面提供WebSocket接口）
  - 第1.2节：TCP接入核心流程（我们用WS替代TCP作为主要协议）
- **Gorilla WebSocket文档**: <https://pkg.go.dev/github.com/gorilla/websocket>
- **Chat Hub示例**: <https://github.com/gorilla/websocket/tree/master/examples/chat>
- **前置依赖**: Issue #2 (认证系统) - 用于验证Token

## 🔍 验收标准

1. ✅ 可以通过 `ws://localhost:9000/ws` 成功建立WebSocket连接
2. ✅ 发送登录消息（MsgType=1）后成功认证并获取用户信息
3. ✅ 心跳机制正常工作（30秒Ping/Pong）
4. ✅ 断线后可以自动重连并恢复会话
5. ✅ 可以发送文本消息并收到服务端Ack确认
6. ✅ Hub能正确管理在线用户列表
7. ✅ 单元测试全部通过 (`go test ./gate/...`)

## ⚠️ 注意事项

- ⚠️ **安全性**: 生产环境必须配置 CORS 白名单（CheckOrigin）
- ⚠️ **性能**: Hub的读写锁可能成为瓶颈，后续可考虑分片优化
- ⚠️ **兼容性**: 当前版本仅支持JSON格式，可扩展支持Protobuf二进制协议
- ⚠️ **限流**: 应添加连接数限制和IP频率限制（防DDoS）

## 📊 工作量评估

- 预计耗时: 2-3天
- 复杂度: ⭐⭐⭐ (核心通信组件)
- 依赖:
  - **必须**: Issue #2 (认证系统) - 用于Token验证
  - **建议**: Issue #3 (配置管理) - 用于读取端口等配置

---
**所属阶段**: 第2周 - 接入层协议（对应文章1.1.2节）
**优先级**: P0 (核心功能 - Web端唯一入口)

---

## 设计审查与必要修改（自动追加）

- WebSocket 与 TCP 共存时，必须有统一的协议约定（frame/消息类型/ack），避免不同接入实现导致逻辑不一致。
- 心跳/认证流程必须在握手阶段完成，避免未认证连接占用资源。
- 建议支持 length-prefix 或 protobuf 二进制帧以适配更复杂的消息类型与粘包情形。

详见：[ISSUE_DESIGN_REVIEW.md](ISSUE_DESIGN_REVIEW.md)
