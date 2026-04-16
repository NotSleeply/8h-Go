# Issue42 交付记录: 群聊 C2G（群管理 + 广播 + 权限）

## 1. 问题描述

Issue #42 要求实现群聊能力，包括群组管理、群消息广播和成员权限管理。  
当前项目已具备 C2C 和投递链路，本次重点是把群场景接到现有 `message_recipient` fan-out 持久化路径上。

## 2. 技术难点

- 需要在不破坏现有 C2C 协议的情况下扩展 C2G 指令体系
- 群消息必须拆分为多接收者投递项（fan-out）并复用现有重试/回执机制
- 权限模型要最小可用且行为明确（群主/管理员/成员）

## 3. 解决方案

### 3.1 群管理核心

新增 `src/server/group.go`：

- `GroupManager` 内存管理群数据
- 支持:
  - 创建群 `Create`
  - 解散群 `Delete`（仅群主）
  - 加入/退出 `Join/Leave`
  - 踢人 `Kick`（管理员/群主）
  - 设/撤管理员 `GrantAdmin/RevokeAdmin`（仅群主）
  - 查询成员 `Members`
  - 查询角色 `RoleOf`

### 3.2 群消息广播（fan-out）

- 新增命令 `gt|群ID|消息`
- 发送时校验发言者是否在群中
- 群成员（排除发送者）拆分为 recipients
- 调用 `logic.ProcessSend(req, recipients)` 落库并写入 `message_recipient`
- 复用既有投递、重试、回执、死信链路

### 3.3 命令扩展

新增命令：

- `gc|群ID` 创建群
- `gj|群ID` 加入群
- `gl|群ID` 退出群
- `gd|群ID` 解散群
- `gk|群ID|用户名` 踢人
- `ga|群ID|用户名` 设管理员
- `gr|群ID|用户名` 撤管理员
- `gm|群ID` 查看群成员
- `gt|群ID|消息` 群发

## 4. 代码变更

- 新增: `src/server/group.go`
- 修改: `src/server/server.go`（挂载 `groupManager`）
- 修改: `src/server/user.go`（路由群命令）
- 修改: `src/server/command.go`（群命令实现 + help）

## 5. 验证结果

- `go test ./...` 通过
- `go build .` 通过
- 群管理与群广播命令可执行
- 群消息通过 `message_recipient` fan-out 进入既有 S2C 投递链路

## 6. 当前限制与后续优化

- 当前群数据在内存中，重启不保留；后续可落库到 `Room/GroupMember`
- 权限体系为简化版，后续可补审计日志与操作时间线
- 大群 fan-out 可进一步引入批量分片与并发 worker 调优
