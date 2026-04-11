## 📌 模块概述
**目标**: 基于《[一个海量在线用户即时通讯系统（IM）的完整设计Plus](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)》实现 **群聊功能 (Client-to-Group, C2G)**，对应文章第1.1.4节提到的 **"群聊(c2g)"** 功能。

> 📖 **架构参考**: 文章第1.1.4节 - 逻辑层核心功能包括 "单聊(c2c)、... 群聊(c2g) ..."。

## ✨ 实现效果
- [ ] 支持创建群组（设置群名、群主）
- [ ] 用户可以加入/退出群组
- [ ] 群消息广播给所有在线成员
- [ ] 群成员管理（管理员/普通成员/群主）
- [ ] 群消息持久化到SQLite（按ChatID分区）
- [ ] 离线用户上线后可拉取群消息历史

## 🏗️ 架构定位
```
发送者 ──→ Gate ──→ Logic(C2G Service) ──→ SQLite(群消息)
                      │
                      ├── 遍历群成员列表
                      ├── 查询每个成员的在线状态(Redis)
                      │
                      ↓ 通过Kafka异步推送
                 Kafka(im-message-deliver)
                      │
                      ↓ Task消费者处理
                 Gate ──→ 各在线成员客户端
```

## 📋 实现步骤

### Step 1: 定义群组相关数据模型
在 `storage/model.go` 中添加（已在Issue1定义Room和GroupMember表）

### Step 2: 实现C2G服务
新建文件: `logic/c2g_service.go`

```go
package logic

// C2GService 群聊服务
type C2GService struct {
    roomDAO    *storage.RoomDAO
    memberDAO  *storage.GroupMemberDAO
    msgDAO     *storage.MessageDAO
    kafkaProd  *messaging.KafkaProducer
    redisSvc   *cache.UserService
}

// CreateGroup 创建群组
func (s *C2GService) CreateGroup(ctx context.Context, name, ownerID string) (*storage.Room, error)

// JoinGroup 加入群组
func (s *C2GService) JoinGroup(ctx context.Context, roomID, userID string) error

// LeaveGroup 离开群组
func (s *C2GService) LeaveGroup(ctx context.Context, roomID, userID string) error

// SendGroupMessage 发送群消息（对应文章C2G流程）
func (s *C2GService) SendGroupMessage(ctx context.Context, req *SendGroupMsgRequest) (*SendGroupMsgReply, error) {
    // 1. 检查发送者是否为群成员
    // 2. 生成ServerMsgID和Seq
    // 3. 保存消息到SQLite（ChatType=2表示群聊）
    // 4. 获取群成员列表
    // 5. 遍历成员，通过Kafka异步推送给每个在线成员
    // 6. 返回Ack给发送者
}

// GetGroupMembers 获取群成员列表
func (s *C2GService) GetGroupMembers(ctx context.Context, roomID string) ([]*storage.GroupMember, error)

// GetGroupHistory 获取群聊历史消息
func (s *C2GService) GetGroupHistory(ctx context.Context, roomID string, limit, offset int) ([]*storage.Message, int64, error)
```

### Step 3: 在Logic层集成C2G服务
扩展 `logic/service.go`，添加HandleC2GMessage方法

---
**所属阶段**: 第3周 - 群聊功能模块
**优先级**: P0 (核心功能)
