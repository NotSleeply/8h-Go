## 📌 模块概述

**目标**: 基于《[一个海量在线用户即时通讯系统（IM）的完整设计Plus](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)** 抽取完整的 **Logic 逻辑层**，对应文章第1.1.4节的 **逻辑层** 设计。

> 📖 **架构参考**: 文章第1.1.4节 - "逻辑层负责IM系统各项功能的核心逻辑实现。包括单聊(c2c)、上报(c2s)、推送(s2c)、群聊(c2g)、离线消息、登录授权、组织机构树等等内容"。

## ✨ 实现效果

- [ ] Logic层作为独立进程运行（可水平扩展）
- [ ] 统一的业务逻辑入口（C2C/C2G/S2C/Auth）
- [ ] 提供gRPC接口供Gate/API层调用（后续Issue12完善）
- [ ] 集成Redis/Kafka进行高性能处理

---
**所属阶段**: 第3周 - 逻辑层完整实现
**优先级**: P0 (核心架构组件)

---

## 设计审查与必要修改（自动追加）

- Logic 层必须定义清晰的契约（接口/错误码/超时策略），并在 proto 或 API 文档中标准化。
- 对于可能的高并发写操作（如保存大量 message/recipient），Logic 应使用批量写入与事务，以减少 DB 锁竞争。
- 在逻辑层加入可观察性（trace/metrics/log）以便定位未 ack 的消息与投递瓶颈。

详见：[ISSUE_DESIGN_REVIEW.md](ISSUE_DESIGN_REVIEW.md)
