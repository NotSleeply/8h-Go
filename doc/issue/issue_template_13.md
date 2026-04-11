## 📌 模块概述

**目标**: 使用 **React + TypeScript + Vite** 构建现代化的 **Web聊天前端界面**，提供完整的IM用户体验。

> 🎨 **技术选型**: 纯Web端（无需iOS/Android），使用现代前端技术栈：
>
> - **React 18** - 组件化UI框架
> - **TypeScript** - 类型安全
> - **Vite** - 极速构建工具
> - **WebSocket API** - 实时通信
> - **Ant Design / TailwindCSS** - UI组件库

## ✨ 实现效果

- [ ] 用户登录/注册页面
- [ ] 聊天主界面（左侧会话列表 + 右侧聊天窗口）
- [ ] 单聊/群聊切换
- [ ] 消息发送（文本、表情）
- [ ] 消息实时接收显示
- [ ] 在线状态指示器
- [ ] 响应式布局（支持桌面端和平板）

## 🏗️ 前端架构

```
src/
├── components/
│   ├── ChatWindow.tsx      # 聊天窗口主组件
│   ├── MessageList.tsx     # 消息列表
│   ├── MessageItem.tsx     # 单条消息
│   ├── InputBox.tsx        # 输入框
│   ├── SessionList.tsx     # 会话列表
│   └── Login.tsx           # 登录页
├── hooks/
│   ├── useWebSocket.ts     # WebSocket连接管理
│   └── useAuth.ts          # 认证状态管理
├── services/
│   └── api.ts              # HTTP API调用
├── types/
│   └── index.ts            # TypeScript类型定义
└── App.tsx                 # 应用入口
```

---
**所属阶段**: 第4周 - 前端UI开发
**优先级**: P1 (用户体验优化)

---

## 设计审查与必要修改（自动追加）

- 前端需与后端约定消息协议（字段/格式/心跳/ack），并与 `protocol`/proto 保持一致，避免前后端协议漂移。
- 对于显示历史消息，建议前端使用分页与增量拉取（基于 seq 或 server_msg_id），不要一次性拉取大量历史。
- 前端应实现幂等发送（重试时使用相同 client_msg_id）并处理重复消息与乱序展示。

详见：[ISSUE_DESIGN_REVIEW.md](ISSUE_DESIGN_REVIEW.md)
