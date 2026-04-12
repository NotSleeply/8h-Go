# Issue45 交付记录: Docker 部署（一键启动全服务）

## 1. 问题描述

Issue #45 要求提供一键启动的容器化部署方案，覆盖应用与核心中间件，支持本地开发/联调。

## 2. 技术难点

- 需要兼容当前 Go 工程结构与已有环境变量
- 既要满足快速启动，也要保留数据卷避免重启丢数据
- 需要同时容纳 Redis + Kafka（含依赖 Zookeeper）

## 3. 解决方案

### 3.1 docker-compose 编排

新增 `docker-compose.yml`，包含：

- `app`（Go 应用）
- `redis`（持久化 AOF + 数据卷）
- `zookeeper`
- `kafka`

并配置：

- 端口映射
- 依赖关系
- 卷持久化
- 健康检查（Redis）

### 3.2 环境变量模板

新增 `.env.example`，统一管理：

- 应用端口与数据库路径
- MQ 模式与 Redis/Kafka 参数
- 连接限流/重试策略
- 中间件端口

### 3.3 启停脚本

新增：

- `scripts/start.sh`：自动生成 `.env`（若不存在）并 `docker compose up -d --build`
- `scripts/stop.sh`：`docker compose down`

### 3.4 文档更新

- `README.md` 新增 Docker 启动/停止说明

## 4. 代码变更

- 新增: `docker-compose.yml`
- 新增: `.env.example`
- 新增: `scripts/start.sh`
- 新增: `scripts/stop.sh`
- 修改: `README.md`

## 5. 验证结果

- `go test ./...` 通过（代码层验证）
- Compose 文件可用于本地容器编排启动（需本机 Docker 环境）

## 6. 当前限制与后续优化

- 当前 app 容器采用 `go run` 开发模式，生产建议改为多阶段构建镜像
- Kafka 采用单节点配置，生产建议升级为多副本集群
- 可补 `docker-compose.override.yml` 支持开发调试配置
