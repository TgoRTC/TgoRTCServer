# 集群部署指南

## 架构概述

### 单机部署（推荐用于开发/测试）
```
┌─────────────────────────────────────┐
│         机器 1（单台服务器）         │
├─────────────────────────────────────┤
│  ┌──────────────────────────────┐   │
│  │   TgoRTCServer（业务服务）   │   │
│  └──────────────────────────────┘   │
│  ┌──────────────────────────────┐   │
│  │   Nginx（反向代理）           │   │
│  └──────────────────────────────┘   │
│  ┌──────────────────────────────┐   │
│  │   LiveKit（音视频服务）       │   │
│  └──────────────────────────────┘   │
│  ┌──────────────────────────────┐   │
│  │   Redis（数据存储）           │   │
│  └──────────────────────────────┘   │
└─────────────────────────────────────┘
```

### 集群部署（推荐用于生产环境）
```
┌──────────────────────────────────────────────────────────────┐
│                      前端客户端                               │
└──────────────────────────────────────────────────────────────┘
                            ↓
┌──────────────────────────────────────────────────────────────┐
│              机器 1（本服务 + Nginx）                         │
├──────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────┐                            │
│  │   TgoRTCServer（业务服务）   │                            │
│  └──────────────────────────────┘                            │
│  ┌──────────────────────────────┐                            │
│  │   Nginx（反向代理 + 负载均衡）│                            │
│  └──────────────────────────────┘                            │
│  ┌──────────────────────────────┐                            │
│  │   Certbot（HTTPS 证书）       │                            │
│  └──────────────────────────────┘                            │
└──────────────────────────────────────────────────────────────┘
                            ↓
        ┌───────────────────┼───────────────────┐
        ↓                   ↓                   ↓
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
│  机器 2（Redis）  │ │ 机器 3（LiveKit）│ │ 机器 4（LiveKit）│
├──────────────────┤ ├──────────────────┤ ├──────────────────┤
│  Redis 服务器    │ │ LiveKit 节点 1   │ │ LiveKit 节点 2   │
│  192.168.1.2     │ │ 192.168.1.3      │ │ 192.168.1.4      │
└──────────────────┘ └──────────────────┘ └──────────────────┘
        ↑                   ↑                   ↑
        └───────────────────┼───────────────────┘
                    共享数据存储
```

## 部署步骤

### 1. 单机部署

**适用场景：** 开发、测试、小规模生产

**步骤：**

```bash
# 1. 复制环境配置文件
cp .env.example .env

# 2. 编辑 .env，设置域名
# DOMAIN=livekit.example.com

# 3. 部署（包含 Nginx + LiveKit + Redis）
./部署.sh deploy

# 4. 初始化 HTTPS 证书
./部署.sh init-https

# 5. 启动业务服务
go run main.go
```

**验证：**
```bash
# 访问 Swagger UI
https://livekit.example.com/swagger/index.html

# 查看日志
./部署.sh logs nginx
./部署.sh logs livekit
./部署.sh logs redis
```

---

### 2. 集群部署

**适用场景：** 大规模生产环境，需要高可用性和可扩展性

#### 2.1 机器 2：部署 Redis（共享数据存储）

```bash
# 方式 1：使用 Docker
docker run -d \
  --name livekit-redis \
  -p 6379:6379 \
  -v redis-data:/data \
  redis:7-alpine \
  redis-server --appendonly yes

# 方式 2：使用本地 Redis
# 安装 Redis
brew install redis  # macOS
# 或
apt-get install redis-server  # Ubuntu

# 启动 Redis
redis-server --port 6379 --bind 0.0.0.0
```

#### 2.2 机器 1：部署本服务 + Nginx

```bash
# 1. 复制环境配置文件
cp .env.example .env

# 2. 编辑 .env
cat > .env << EOF
DOMAIN=livekit.example.com
LIVEKIT_NODES=192.168.1.3:7880,192.168.1.4:7880
REDIS_HOST=192.168.1.2
REDIS_PORT=6379
EOF

# 3. 部署（只包含 Nginx，不包含 LiveKit）
./部署.sh deploy

# 4. 初始化 HTTPS 证书
./部署.sh init-https

# 5. 启动业务服务
go run main.go
```

#### 2.3 机器 3, 4, ...：部署 LiveKit 节点

```bash
# 1. 复制环境配置文件
cp .env.example .env

# 2. 编辑 .env
cat > .env << EOF
DOMAIN=livekit.example.com
LIVEKIT_NODES=192.168.1.3:7880,192.168.1.4:7880
REDIS_HOST=192.168.1.2
REDIS_PORT=6379
LIVEKIT_API_KEY=your_api_key
LIVEKIT_API_SECRET=your_api_secret
EOF

# 3. 部署 LiveKit 节点
NODES=192.168.1.3,192.168.1.4 ./部署.sh deploy-livekit-only
```

---

## 关键配置说明

### LIVEKIT_NODES
- **单机模式**：留空（默认使用内置 LiveKit）
- **集群模式**：指定远程 LiveKit 节点列表
  ```bash
  LIVEKIT_NODES=192.168.1.3:7880,192.168.1.4:7880,192.168.1.5:7880
  ```

### REDIS_HOST
- **单机模式**：`redis`（Docker 容器名）
- **集群模式**：`192.168.1.2`（外部 Redis 服务器 IP）

### NODES
- 用于 `deploy-livekit-only` 命令
- 指定要部署的 LiveKit 节点列表
  ```bash
  NODES=192.168.1.3,192.168.1.4,192.168.1.5
  ```

---

## 负载均衡

Nginx 使用 `least_conn` 策略进行负载均衡：

```nginx
upstream livekit_backend {
    least_conn;
    server 192.168.1.3:7880 max_fails=3 fail_timeout=30s;
    server 192.168.1.4:7880 max_fails=3 fail_timeout=30s;
    server 192.168.1.5:7880 max_fails=3 fail_timeout=30s;
}
```

**特点：**
- 优先将请求分配给连接数最少的服务器
- 自动故障转移（3 次失败后标记为不可用，30 秒后重试）
- 支持动态添加/删除节点

---

## 故障排查

### 1. Nginx 无法连接到 LiveKit
```bash
# 检查 Nginx 配置
docker exec livekit-nginx nginx -t

# 查看 Nginx 日志
./部署.sh logs nginx

# 检查 LiveKit 节点是否在线
curl http://192.168.1.3:7880/health
```

### 2. LiveKit 节点无法连接到 Redis
```bash
# 检查 Redis 连接
redis-cli -h 192.168.1.2 -p 6379 ping

# 查看 LiveKit 日志
./部署.sh logs livekit
```

### 3. 房间数据不同步
```bash
# 确保所有 LiveKit 节点连接到同一个 Redis
# 检查 livekit.yaml 中的 redis 配置
cat livekit-deployment/config/livekit.yaml | grep -A 5 redis
```

---

## 性能优化建议

1. **Redis 优化**
   - 使用 Redis Cluster 或 Redis Sentinel 提高可用性
   - 定期备份 Redis 数据

2. **LiveKit 优化**
   - 根据负载调整 LiveKit 节点数量
   - 监控 CPU、内存、网络使用情况

3. **Nginx 优化**
   - 调整 `worker_connections` 以支持更多并发连接
   - 启用 gzip 压缩

4. **网络优化**
   - 使用专网或 VPN 连接各个节点
   - 配置防火墙规则，只允许必要的端口

---

## 常见问题

**Q: 如何添加新的 LiveKit 节点？**
A: 修改 `LIVEKIT_NODES` 环境变量，添加新节点的地址，然后重启 Nginx。

**Q: 如何升级 LiveKit 版本？**
A: 修改 `.env` 中的 `LIVEKIT_IMAGE_VERSION`，然后运行 `docker-compose up -d`。

**Q: 如何备份 Redis 数据？**
A: 使用 `./部署.sh backup` 命令备份所有数据。

**Q: 单机模式可以升级到集群模式吗？**
A: 可以。需要部署外部 Redis 和 LiveKit 节点，然后修改配置指向新的节点。

