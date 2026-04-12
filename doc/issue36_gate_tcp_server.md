# Issue36 交付记录: TCP Gate 接入层（连接管理 + 消息路由）

## 1. 问题背景

Issue #36 要求补齐接入层（Gate）核心能力：连接管理、消息路由、会话管理、基础攻击防护（限流/黑名单）和运行统计。  
项目已有 TCP 聊天主链路（连接、广播、私聊、ACK 与投递队列），但缺少可直接用于运维观测与基础防护的实现。

## 2. 本次目标

- 在 TCP 接入流程中增加 `IP 黑名单` 与 `连接频率限制`
- 提供可查询的服务端运行统计（在线数、吞吐、连接计数、拒绝计数、队列长度）
- 提供客户端命令可直接查询统计信息
- 保持现有 TCP 聊天协议能力不回退

## 3. 技术难点

- 连接限流需低侵入地接入 `Accept -> Handler` 流程，不能影响已有会话逻辑
- 指标统计需并发安全，避免在高并发下出现竞态
- 指令体系需兼容旧命令（`who/rename/to/exit`）并新增运维命令

## 4. 解决方案

### 4.1 接入防护

- 在 `Server` 新增:
  - `BlacklistIPs`（来源于 `IM_BLACKLIST_IPS`，逗号分隔）
  - `rateWindow`、`rateLimit`（默认 60s/30 次，可由环境变量覆盖）
  - `attempts` 连接窗口记录表（按 IP 维护滑动时间窗）
- 在 `Start()` 的 `Accept` 后、`Handler` 前执行 `allowConnection(ip)`:
  - 命中黑名单直接拒绝
  - 超过窗口阈值拒绝并关闭连接

### 4.2 统计与观测

- 使用原子计数器维护:
  - `totalConnections`, `activeConn`, `rejectedConn`
  - `inboundMessages`, `outboundMessages`
- 新增 `SnapshotStats()` 统一输出快照:
  - 启动时间、运行时长、在线用户、连接统计
  - 收发消息计数、估算吞吐（msg/s）
  - `DeliverQueue` 当前长度
- 新增命令:
  - `stats` 查看运行指标
  - `help` 查看可用命令列表

### 4.3 兼容性

- 原有 `who/rename/to/exit` 不变
- 原有 ACK 与投递队列链路不变
- 仅在入口增加拒绝分支，不影响已建立连接的消息处理

## 5. 代码变更

- `src/server/server.go`
  - 新增防护配置加载、IP 解析、连接准入判断
  - 新增运行统计结构体与快照接口
  - 在 `Accept` 和 `Handler` 路径接入统计/拒绝逻辑
  - 在读取消息处统计入站消息
- `src/server/user.go`
  - `SendMsg` 成功入队后统计出站消息
  - 新增命令分发: `help` / `stats`
- `src/server/command.go`
  - 新增 `useHelp()`
  - 新增 `useStats()`

## 6. 验证结果

- `go test ./src/server` 通过（当前目录无测试文件，但构建通过）
- `go build .` 通过（TCP 服务端可编译）

## 7. 已知限制

- 当前限流/黑名单是单机内存实现，重启后不保留
- 尚未接入分布式共享路由与集中式限流（如 Redis）
- 统计接口目前通过 TCP 命令暴露，尚未提供 HTTP/Prometheus 导出

## 8. 后续建议

- 将黑名单、限流和会话映射迁移到 Redis，支持多实例 Gate
- 增加 Prometheus 指标导出和结构化日志
- 对限流策略增加分级（按 IP、按 UID、按命令类型）
