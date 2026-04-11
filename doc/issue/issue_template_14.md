## 📌 模块概述

**目标**: 使用 **Docker + Docker Compose** 容器化部署整个IM系统，实现一键启动所有服务。

## ✨ 实现效果

- [ ] 所有服务可容器化运行（Gate/Logic/API/Task）
- [ ] 一键 `docker-compose up` 启动全部服务
- [ ] 包含Redis/Kafka/etcd等依赖服务
- [ ] 支持开发环境和生产环境配置
- [ ] 日志统一收集

## 🐳 Docker Compose 服务清单

```yaml
services:
  gate:       # WebSocket接入层
  logic:      # 业务逻辑层 (gRPC Server)
  api:        # RESTful API层 (:8080)
  task:       # 异步任务处理层 (Kafka Consumer)

  # 基础设施
  redis:      # 缓存和状态存储
  kafka:      # 消息队列
  zookeeper:  # Kafka依赖
  etcd:       # 服务发现

  # 可选
  prometheus: # 监控
  grafana:    # 可视化
```

---
**所属阶段**: 第4周 - 部署与运维
**优先级**: P0 (交付必备)

---

## 设计审查与必要修改（自动追加）

- Docker Compose 的配置仅适合开发/测试环境，生产应建议使用 k8s 或受管服务并考虑状态ful组件（Redis/Kafka/DB）的高可用部署。
- 在文档中加入演练步骤：如何恢复持久化 DB、如何回放死信队列、如何平滑滚动重启 Gate/Logic 服务并保证投递不丢失。
- 增加监控与告警清单（队列积压、未 ack 比例、消费者滞后、磁盘/连接不足等指标）。

详见：[ISSUE_DESIGN_REVIEW.md](ISSUE_DESIGN_REVIEW.md)
