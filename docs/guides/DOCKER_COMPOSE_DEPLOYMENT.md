# Docker Compose 部署指南

本指南介绍如何使用 Docker Compose 快速部署 LiveKit 服务。

## 📋 目录

- [快速开始](#快速开始)
- [单机部署](#单机部署)
- [集群部署](#集群部署)
- [常用命令](#常用命令)
- [故障排查](#故障排查)

---

## 🚀 快速开始

### 前置要求

- Docker 20.10+
- Docker Compose 2.0+
- 域名（用于 HTTPS 证书）

### 安装 Docker

**macOS:**
```bash
brew install docker docker-compose
```

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install docker.io docker-compose
sudo usermod -aG docker $USER
```

**CentOS/RHEL:**
```bash
sudo yum install docker docker-compose
sudo usermod -aG docker $USER
```

---

## 🏗️ 单机部署

### 1. 准备环境

```bash
# 克隆项目
git clone <your-repo>
cd TgoCallServer

# 复制环境配置
cp .env.example .env

# 编辑 .env 文件
nano .env
```

### 2. 修改 .env 配置

```bash
# 必需配置
DOMAIN=livekit.example.com

# Redis 配置（单机模式使用内置 Redis）
REDIS_HOST=redis
REDIS_PORT=6379

# LiveKit 节点（单机模式留空）
LIVEKIT_NODES=

# Docker 镜像版本
LIVEKIT_IMAGE_VERSION=latest
REDIS_IMAGE_VERSION=7-alpine
NGINX_IMAGE_VERSION=alpine
CERTBOT_IMAGE_VERSION=latest
```

### 3. 生成配置文件

```bash
# 运行部署脚本生成配置
./部署.sh deploy
```

### 4. 初始化 HTTPS 证书

```bash
# 申请 Let's Encrypt 证书
./部署.sh init-https
```

### 5. 启动业务服务

```bash
# 启动 Go 应用
go run main.go
```

### 6. 验证部署

```bash
# 查看容器状态
docker-compose -f livekit-deployment/docker-compose.yml ps

# 查看日志
docker-compose -f livekit-deployment/docker-compose.yml logs -f

# 测试 API
curl -X GET https://livekit.example.com/api/v1/rooms \
  -H "Authorization: Bearer $TOKEN"
```

---

## 🌐 集群部署

### 架构

```
┌─────────────────────────────────────────┐
│  机器 1（本服务 + Nginx）               │
│  - TgoCallServer（业务服务）            │
│  - Nginx（反向代理 + 负载均衡）         │
│  - Certbot（HTTPS 证书）                │
└─────────────────────────────────────────┘
              ↓
    ┌─────────┼─────────┐
    ↓         ↓         ↓
┌────────┐ ┌────────┐ ┌────────┐
│机器 2  │ │机器 3  │ │机器 4  │
│Redis   │ │LiveKit │ │LiveKit │
│        │ │节点 1  │ │节点 2  │
└────────┘ └────────┘ └────────┘
```

### 部署步骤

#### 机器 1：部署本服务 + Nginx

```bash
# 1. 编辑 .env
DOMAIN=livekit.example.com
LIVEKIT_NODES=192.168.1.3:7880,192.168.1.4:7880
REDIS_HOST=192.168.1.2
REDIS_PORT=6379

# 2. 生成配置
./部署.sh deploy

# 3. 初始化 HTTPS 证书
./部署.sh init-https

# 4. 启动业务服务
go run main.go
```

#### 机器 2：部署 Redis

```bash
# 方式 1：使用 Docker
docker run -d \
  --name livekit-redis \
  -p 6379:6379 \
  -v redis_data:/data \
  redis:7-alpine redis-server

# 方式 2：使用 Docker Compose
docker-compose up -d redis
```

#### 机器 3, 4：部署 LiveKit 节点

```bash
# 1. 编辑 .env
REDIS_HOST=192.168.1.2
REDIS_PORT=6379
LIVEKIT_NODES=

# 2. 生成配置
./部署.sh deploy-livekit-only

# 3. 查看日志
docker-compose -f livekit-deployment/docker-compose.yml logs -f livekit
```

---

## 📝 常用命令

### 启动/停止服务

```bash
# 启动所有服务
docker-compose -f livekit-deployment/docker-compose.yml up -d

# 停止所有服务
docker-compose -f livekit-deployment/docker-compose.yml down

# 重启服务
docker-compose -f livekit-deployment/docker-compose.yml restart

# 重启特定服务
docker-compose -f livekit-deployment/docker-compose.yml restart livekit
```

### 查看日志

```bash
# 查看所有日志
docker-compose -f livekit-deployment/docker-compose.yml logs -f

# 查看特定服务日志
docker-compose -f livekit-deployment/docker-compose.yml logs -f livekit
docker-compose -f livekit-deployment/docker-compose.yml logs -f redis
docker-compose -f livekit-deployment/docker-compose.yml logs -f nginx
```

### 数据备份和恢复

```bash
# 备份数据
./部署.sh backup

# 恢复数据
./部署.sh restore /path/to/backup
```

### 验证部署

```bash
# 验证所有服务
./部署.sh verify

# 检查容器状态
docker-compose -f livekit-deployment/docker-compose.yml ps

# 检查网络
docker network inspect livekit
```

---

## 🔧 故障排查

### 问题 1：Nginx 无法连接到 LiveKit

**症状：** 访问 https://livekit.example.com 返回 502 Bad Gateway

**解决方案：**
```bash
# 1. 检查 LiveKit 是否运行
docker-compose -f livekit-deployment/docker-compose.yml ps livekit

# 2. 检查 LiveKit 日志
docker-compose -f livekit-deployment/docker-compose.yml logs livekit

# 3. 检查 Nginx 配置
docker-compose -f livekit-deployment/docker-compose.yml exec nginx nginx -t

# 4. 检查网络连接
docker-compose -f livekit-deployment/docker-compose.yml exec nginx \
  curl -v http://livekit:7880/
```

### 问题 2：Redis 连接失败

**症状：** LiveKit 日志显示 "Failed to connect to Redis"

**解决方案：**
```bash
# 1. 检查 Redis 是否运行
docker-compose -f livekit-deployment/docker-compose.yml ps redis

# 2. 测试 Redis 连接
docker-compose -f livekit-deployment/docker-compose.yml exec redis \
  redis-cli ping

# 3. 检查 Redis 配置
cat livekit-deployment/config/redis.conf

# 4. 查看 Redis 日志
docker-compose -f livekit-deployment/docker-compose.yml logs redis
```

### 问题 3：HTTPS 证书申请失败

**症状：** Certbot 日志显示证书申请失败

**解决方案：**
```bash
# 1. 检查域名 DNS 解析
nslookup livekit.example.com

# 2. 检查 80 端口是否开放
curl -v http://livekit.example.com/.well-known/acme-challenge/test

# 3. 手动申请证书
docker-compose -f livekit-deployment/docker-compose.yml exec certbot \
  certbot certonly --webroot -w /var/www/certbot \
  -d livekit.example.com \
  --email admin@livekit.example.com \
  --agree-tos \
  --non-interactive
```

### 问题 4：集群模式下 LiveKit 节点无法通信

**症状：** 多个 LiveKit 节点无法同步房间信息

**解决方案：**
```bash
# 1. 检查 Redis 连接
docker-compose -f livekit-deployment/docker-compose.yml exec redis \
  redis-cli KEYS "livekit:*"

# 2. 检查 LiveKit 配置中的 Redis 地址
cat livekit-deployment/config/livekit.yaml | grep -A 3 "redis:"

# 3. 测试网络连接
docker-compose -f livekit-deployment/docker-compose.yml exec livekit \
  ping 192.168.1.2

# 4. 查看 LiveKit 日志
docker-compose -f livekit-deployment/docker-compose.yml logs livekit | grep -i redis
```

---

## 📊 性能优化

### 1. 调整 Redis 内存

编辑 `.env`：
```bash
REDIS_MAXMEMORY=4gb
REDIS_MAXMEMORY_POLICY=allkeys-lru
```

### 2. 调整 Nginx 工作进程

编辑 `livekit-deployment/config/nginx.conf`：
```nginx
worker_processes auto;  # 自动检测 CPU 核心数
worker_connections 2048;  # 增加连接数
```

### 3. 调整 LiveKit 参数

编辑 `.env`：
```bash
LIVEKIT_MAX_PARTICIPANTS=0  # 0 表示无限制
LIVEKIT_EMPTY_TIMEOUT=300   # 空房间超时时间
```

---

## 📚 相关文档

- [集群部署指南](./CLUSTER_DEPLOYMENT.md)
- [Nginx 配置指南](./NGINX_CONFIGURATION.md)
- [LiveKit 官方文档](https://docs.livekit.io/)

