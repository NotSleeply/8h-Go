## 📌 模块概述

**目标**: 基于《[一个海量在线用户即时通讯系统（IM）的完整设计Plus](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)》构建 **存储层 (Storage Layer)**，使用 SQLite 替代 MySQL 实现消息和用户数据的持久化存储。

> 📖 **架构参考**: 文章第1.1.5节 - 存储层负责缓存或存储IM系统相关数据，主要包括用户状态及路由（缓存），消息数据（采用NoSql/SQLite），文件数据。

## ✨ 实现效果

- [ ] 项目启动时自动创建 SQLite 数据库文件 `gochat.sqlite3`
- [ ] 使用 GORM ORM 框架管理数据库操作
- [ ] 实现**核心表结构**：User(用户)、Message(消息)、Room(房间)、GroupMember(群成员)
- [ ] 支持按会话分区存储（单聊/群聊分离）
- [ ] 重启服务后历史数据完整保留

## 🏗️ 架构定位

```
┌─────────────────────────────────┐
│         Logic 逻辑层             │  ← 业务逻辑处理
├─────────────────────────────────┤
│      Storage 存储层 (本模块)       │  ← 数据持久化
│  ┌──────────┬──────────┐        │
│  │  SQLite   │  Redis   │        │  ← SQLite(主) + Redis(缓存)
│  │ (消息/用户) │ (状态/路由) │        │
│  └──────────┴──────────┘        │
└─────────────────────────────────┘
```

## 📋 实现步骤

### Step 1: 安装依赖

```bash
go get gorm.io/gorm
go get gorm.io/driver/sqlite
```

### Step 2: 创建数据库初始化代码

新建文件: `storage/db.go`

```go
package storage

import (
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "log"
)

var DB *gorm.DB

func InitDB(dsn string) error {
    var err error
    DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
    if err != nil {
        return err
    }
    log.Printf("[Storage] SQLite database connected: %s", dsn)
    return AutoMigrate()
}

func AutoMigrate() error {
    return DB.AutoMigrate(
        &User{},
        &Message{},
        &Room{},
        &GroupMember{},
    )
}
```

### Step 3: 定义核心模型（对应IM系统实体）

新建文件: `storage/model.go`

```go
package storage

import "time"

// User 用户表（对应文章的用户状态管理）
type User struct {
    ID           uint      `gorm:"primarykey"`
    UserName     string    `gorm:"uniqueIndex;size:50;not null"`
    PasswordHash string    `gorm:"size:100;not null"` // bcrypt加密
    NickName     string    `gorm:"size:50"`
    AvatarURL    string    `gorm:"size:200"`
    Status       int8      `gorm:"default:0;comment:'0-离线 1-在线'"` // 对应用户在线状态
    CreateTime   time.Time `gorm:"autoCreateTime"`
    UpdateTime   time.Time `gorm:"autoUpdateTime"`
}

// Message 消息表（对应文章的消息存储，按会话分区）
type Message struct {
    ID          uint      `gorm:"primarykey"`
    ClientMsgID string    `gorm:"uniqueIndex;size:64;not null"` // 客户端消息ID（幂等性）
    ServerMsgID uint64    `gorm:"uniqueIndex;not null"`          // 服务端消息ID（全局递增）
    Seq         uint64    `gorm:"index;not null"`                // 消息序号（用于排序和同步）
    ChatType    int8      `gorm:"index;not null;comment:'1-单聊 2-群聊'"` // 会话类型
    ChatID      string    `gorm:"index;size:64;not null"`        // 会话ID
    SendID      string    `gorm:"index;size:64;not null"`        // 发送者ID
    RecvID      string    `gorm:"index;size:64;not null"`        // 接收者ID
    ContentType int8      `gorm:"not null;comment:'1-文本 2-图片...'"` // 内容类型
    Content     string    `gorm:"type:text;not null"`            // 消息内容
    Status      int8      `gorm:"default:0;comment:'0-发送中 1-成功 2-失败'"`
    CreateTime  time.Time `gorm:"autoCreateTime;index"`          // 创建时间（用于Timeline查询）
}

// Room 房间表（对应群聊功能）
type Room struct {
    ID          uint      `gorm:"primarykey"`
    RoomID      string    `gorm:"uniqueIndex;size:64;not null"`
    Name        string    `gorm:"size:100;not null"`
    OwnerID     string    `gorm:"index;size:64;not null"`
    CreateTime  time.Time `gorm:"autoCreateTime"`
}

// GroupMember 群成员表
type GroupMember struct {
    ID        uint      `gorm:"primarykey"`
    RoomID    string    `gorm:"index;size:64;not null"`
    UserID    string    `gorm:"index;size:64;not null"`
    Role      int8      `gorm:"default:0;comment:'0-普通成员 1-管理员 2-群主'"`
    JoinTime  time.Time `gorm:"autoCreateTime"`
}
```

### Step 4: 实现 DAO 层（数据访问对象）

新建文件: `storage/dao/user_dao.go`

包含方法:

- CreateUser(user *User) error
- GetUserByID(id uint) (*User, error)
- GetUserByName(name string) (*User, error)
- UpdateUserStatus(userID string, status int8) error // 更新在线状态

新建文件: `storage/dao/message_dao.go`

包含方法:

- SaveMessage(msg *Message) error                    // 保存消息（C2C/C2G通用）
- GetMessagesByChatID(chatID string, limit int, offset int) ([]*Message, error) // 按会话查询（支持分页）
- GetMessagesBySeq(chatID string, startSeq, endSeq uint64) ([]*Message, error)  // 按序号范围查询（增量同步）
- UpdateMessageStatus(serverMsgID uint64, status int8) error // 更新消息状态
- GetLatestSeq(chatID string) (uint64, error)              // 获取最新消息序号（用于离线同步）

### Step 5: 集成到 main.go

在程序启动时调用数据库初始化：

```go
func main() {
    if err := storage.InitDB("gochat.sqlite3"); err != nil {
        log.Fatal(err)
    }
    // ... 其他初始化逻辑
}
```

### Step 6: 编写单元测试

新建文件: `storage/dao/user_dao_test.go`
新建文件: `storage/dao/message_dao_test.go`

测试用例:

- 测试用户CRUD操作
- 测试消息保存和按会话查询
- 测试消息序号(Seq)自增和唯一性
- 测试按时间范围查询（模拟离线拉取）

## 🎯 参考资源

- **📄 架构参考文章**: [《一个海量在线用户即时通讯系统（IM）的完整设计Plus》](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)
  - 第1.1.5节：存储层设计
  - 第1.2节：消息存储部分（最初版本采用MySQL，我们改用SQLite）
  - 第4节：离线消息拉取方式（TimeLine模型）
- **GORM 文档**: <https://gorm.io/docs/>
- **当前项目**: src/server/store.go (现有内存存储实现)

## 🔍 验收标准

1. ✅ 执行 `go run main.go` 自动生成 `gochat.sqlite3` 数据库文件
2. ✅ 成功创建 User、Message、Room、GroupMember 四张表
3. ✅ 可以插入用户并查询（支持用户名唯一约束）
4. ✅ 可以保存消息并按会话ID查询历史记录
5. ✅ 消息序号(Seq)自动递增且全局唯一
6. ✅ 单元测试全部通过 (`go test ./storage/...`)
7. ✅ 程序重启后数据保持不变

## ⚠️ 注意事项

- ⚠️ **密码安全**: PasswordHash 必须使用 bcrypt 加密存储
- ⚠️ **幂等性设计**: ClientMsgID + ServerMsgID 双重保证消息不重复
- ⚠️ **性能优化**: Message 表的 CreateTime 和 Seq 字段必须建立索引（支持快速查询）
- ⚠️ **扩展性**: 预留 ContentType 字段，后续可支持图片、语音、视频等富媒体消息

## 📊 工作量评估

- 预计耗时: 2-3 天
- 复杂度: ⭐⭐⭐ (核心基础设施)
- 依赖: 无前置依赖

---
**所属阶段**: 第1周 - 存储层搭建（对应文章1.1.5节）
**优先级**: P0 (必须完成 - 整个IM系统的数据基础)

---

## 设计审查与必要修改（自动追加）

此 issue 模板在设计/实现层面存在若干共性风险，关键建议（快速要点）：

- 持久化与入队应为原子操作：保存 message + recipients + 标记 pending 必须在同一事务中完成。
- 新增 `message_recipient`（或 `delivery`）表：记录 `server_msg_id, to_user, status(pending/sent/acked), retry_count, last_send_time`，便于重试与恢复。
- 幂等性索引：不要只对 `ClientMsgID` 建单列 unique，建议使用复合唯一键 `(from, client_msg_id)` 或由客户端保证全局唯一。
- 统一 `ServerMsgID` 类型：明确使用 `TEXT/string` 或 `uint64`（Snowflake），并在代码/文档中一致化。
- 启动恢复：服务启动时需扫描未 ack 的 `message_recipient` 记录并将其恢复至投递队列。
- 投递策略：实现重试（指数退避）、最大重试次数与死信处理，并记录告警/监控指标。
- 协议建议：生产场景建议采用 length-prefix 帧或 Protobuf（避免仅靠行分割 JSON）。
- 连接层：必须实现 `auth`（token）与 `heartbeat`，便于断线检测和路由维护。
- SQLite 注意：启用 `PRAGMA journal_mode=WAL`、`PRAGMA synchronous=NORMAL`、设置 `busy_timeout` 和限制并发写（例如 `SetMaxOpenConns(1)`），并为写操作使用事务。

详见：[ISSUE_DESIGN_REVIEW.md](ISSUE_DESIGN_REVIEW.md) 获取完整示例 schema、初始化与恢复伪代码。
