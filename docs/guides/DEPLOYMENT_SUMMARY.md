# Docker Compose 部署文件总结

本文档总结了为 TgoCallServer 项目创建的所有 Docker Compose 部署文件。

## 📦 创建的文件清单

### 1. 配置文件

#### `docker-compose.yml`
- **用途**：基础 Docker Compose 配置
- **支持**：单机和集群部署模式
- **包含服务**：
  - Nginx（反向代理 + 负载均衡）
  - Certbot（HTTPS 证书管理）
  - Redis（数据存储和消息总线）
  - LiveKit（音视频服务）
- **特点**：
  - 支持环境变量配置
  - 包含健康检查
  - 自动重启策略
  - 网络隔离

#### `docker-compose.prod.yml`
- **用途**：生产环境完整配置
- **包含服务**：
  - 业务服务（TgoCallServer）
  - Nginx、Certbot、Redis、LiveKit
- **特点**：
  - 包含业务服务容器
  - 完整的生产环境配置
  - 日志持久化
  - 环境变量管理

### 2. 构建文件

#### `Dockerfile`
- **用途**：构建业务服务 Docker 镜像
- **特点**：
  - 多阶段构建（减小镜像大小）
  - 自动生成 Swagger 文档
  - 非 root 用户运行
  - 健康检查配置
  - 基于 Alpine Linux（轻量级）

#### `.dockerignore`
- **用途**：优化 Docker 构建
- **排除项**：
  - Git 文件
  - IDE 配置
  - 测试文件
  - 文档
  - 部署文件

### 3. 自动化工具

#### `Makefile`
- **用途**：简化常用命令
- **主要命令**：
  - `make deploy` - 部署开发环境
  - `make deploy-prod` - 部署生产环境
  - `make init-https` - 初始化 HTTPS 证书
  - `make backup` - 备份数据
  - `make restore` - 恢复数据
  - `make verify` - 验证部署
  - `make logs` - 查看日志
  - `make clean` - 清理文件

### 4. 文档

#### `docs/guides/DOCKER_COMPOSE_DEPLOYMENT.md`
- **内容**：完整的部署指南
- **包括**：
  - 快速开始
  - 单机部署步骤
  - 集群部署步骤
  - 常用命令
  - 故障排查
  - 性能优化

#### `docs/guides/QUICK_REFERENCE.md`
- **内容**：快速参考卡片
- **包括**：
  - 一键部署命令
  - 常用命令速查表
  - Docker Compose 命令
  - 故障排查速查表
  - 配置文件位置
  - 访问地址
  - 安全建议

#### `docs/guides/DEPLOYMENT_SUMMARY.md`（本文件）
- **内容**：部署文件总结
- **包括**：
  - 文件清单
  - 使用场景
  - 快速开始
  - 常见问题

---

## 🚀 使用场景

### 场景 1：本地开发

```bash
# 1. 复制环境配置
cp .env.example .env

# 2. 启动开发环境
make deploy

# 3. 初始化 HTTPS 证书
make init-https

# 4. 启动业务服务
go run main.go
```

### 场景 2：单机生产部署

```bash
# 1. 编辑 .env
nano .env

# 2. 部署生产环境
make deploy-prod

# 3. 初始化 HTTPS 证书
make init-https
```

### 场景 3：集群生产部署

**机器 1（本服务 + Nginx）：**
```bash
# 编辑 .env
LIVEKIT_NODES=192.168.1.3:7880,192.168.1.4:7880
REDIS_HOST=192.168.1.2

# 部署
make deploy
make init-https
go run main.go
```

**机器 2（Redis）：**
```bash
docker run -d \
  --name livekit-redis \
  -p 6379:6379 \
  redis:7-alpine
```

**机器 3, 4（LiveKit 节点）：**
```bash
# 编辑 .env
REDIS_HOST=192.168.1.2

# 部署
./部署.sh deploy-livekit-only
```

---

## 📋 快速开始

### 最简单的方式

```bash
# 一键部署
make quick-start

# 启动业务服务
go run main.go

# 完成！访问 https://livekit.example.com
```

### 查看状态

```bash
# 查看容器状态
make ps

# 查看日志
make docker-logs

# 查看部署信息
make info
```

### 停止服务

```bash
# 停止所有服务
make docker-stop

# 清理所有容器和数据
make docker-clean
```

---

## 🔧 常见问题

### Q1：如何修改配置？

编辑 `.env` 文件，然后重新部署：

```bash
nano .env
make deploy
```

### Q2：如何查看日志？

```bash
# 查看所有日志
make docker-logs

# 查看特定服务日志
docker-compose -f livekit-deployment/docker-compose.yml logs -f livekit
```

### Q3：如何备份数据？

```bash
# 备份
make backup

# 恢复
make restore
```

### Q4：如何升级 LiveKit 版本？

编辑 `.env`：
```bash
LIVEKIT_IMAGE_VERSION=v1.5.0
```

然后重新部署：
```bash
make deploy
```

### Q5：如何在集群模式下添加新的 LiveKit 节点？

1. 编辑 `.env`，添加新节点到 `LIVEKIT_NODES`
2. 在新机器上部署 LiveKit：`./部署.sh deploy-livekit-only`
3. 重启 Nginx：`make restart`

---

## 📊 文件对应关系

| 文件 | 用途 | 何时使用 |
|------|------|--------|
| `docker-compose.yml` | 基础配置 | 开发/测试环境 |
| `docker-compose.prod.yml` | 生产配置 | 生产环境 |
| `Dockerfile` | 构建镜像 | 构建业务服务 |
| `.dockerignore` | 优化构建 | 自动使用 |
| `Makefile` | 便捷命令 | 日常操作 |
| `部署.sh` | 部署脚本 | 初始部署 |

---

## 🎯 下一步

1. **阅读完整指南**：[DOCKER_COMPOSE_DEPLOYMENT.md](./DOCKER_COMPOSE_DEPLOYMENT.md)
2. **查看快速参考**：[QUICK_REFERENCE.md](./QUICK_REFERENCE.md)
3. **了解集群部署**：[CLUSTER_DEPLOYMENT.md](./CLUSTER_DEPLOYMENT.md)
4. **开始部署**：`make quick-start`

---

## 📞 获取帮助

```bash
# 查看 Makefile 帮助
make help

# 查看部署脚本帮助
./部署.sh help

# 查看部署日志
tail -f livekit-deployment/deploy.log
```

---

## ✅ 检查清单

部署前请确认：

- [ ] Docker 已安装（版本 20.10+）
- [ ] Docker Compose 已安装（版本 2.0+）
- [ ] 域名已配置
- [ ] DNS 已解析
- [ ] 80 和 443 端口已开放
- [ ] `.env` 文件已配置
- [ ] 足够的磁盘空间（至少 10GB）
- [ ] 足够的内存（至少 4GB）

---

## 📚 相关资源

- [Docker 官方文档](https://docs.docker.com/)
- [Docker Compose 官方文档](https://docs.docker.com/compose/)
- [LiveKit 官方文档](https://docs.livekit.io/)
- [Nginx 官方文档](https://nginx.org/en/docs/)
- [Let's Encrypt 官方文档](https://letsencrypt.org/docs/)

