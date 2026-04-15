# 8h-Go 聊天服务端

本项目是一个简单的基于 TCP 的聊天室服务端示例，来自「[8小时转go](https://www.bilibili.com/video/BV1gf4y1r79E)」学习实践。项目实现了用户上线/下线、广播消息、私聊和改名功能，适合用来学习 Go 并发、网络编程和通道设计。

## 功能

- TCP 服务端监听 `127.0.0.1:8888`
- 用户连接后自动上线
- 支持广播消息给所有在线用户
- 支持超时踢人机制

## 快速启动

```bash
go run main.go
```

Windows（推荐一键）：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start.ps1
```

## Docker 一键启动

```bash
docker compose up -d
```

或使用脚本：

```bash
sh scripts/start.sh
```

停止：

```bash
sh scripts/stop.sh
```

Windows 停止：

```powershell
.\scripts\stop.ps1
```

## 常见问题

### `init db failed: dial tcp: lookup mysql: no such host`

这是因为 `mysql` 这个主机名只在 Docker Compose 内部网络可解析。  
如果你在 Windows 主机直接运行 `go run .`，应使用主机端口映射地址（例如 `127.0.0.1:3306`），或者使用仓库中的 `.env.local` 配置。

项目启动时会按以下优先级读取配置：
1. 已存在的系统环境变量（不覆盖）
2. `.env.local`
3. 代码默认值

## 测试连接

在另一个终端使用 `nc` 连接：

```bash
nc 127.0.0.1 8888
```

## 致敬

- [学习笔记](https://juejin.cn/post/7617013574440501289)
- [LockGit/gochat](https://github.com/LockGit/gochat)
- [8小时转go](https://www.bilibili.com/video/BV1gf4y1r79E)
- [一个海量在线用户即时通讯系统（IM）的完整设计Plus](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)
