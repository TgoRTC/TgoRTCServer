# TgoRTC Server 部署指南

> 基于 LiveKit 的实时音视频通话服务

---

## 🚀 一键部署（推荐）

### 快速开始

```bash
# 国内服务器（使用镜像加速）
curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy.sh | sudo bash -s -- --cn

# 海外服务器
curl -fsSL https://raw.githubusercontent.com/TgoRTC/TgoRTCServer/main/scripts/deploy.sh | sudo bash
```

一键部署会自动：
- ✅ 安装 Docker（如未安装）
- ✅ 配置镜像加速（国内）
- ✅ 生成随机密码和密钥
- ✅ 创建所有配置文件
- ✅ 启动所有服务

### 部署后操作

```bash
cd ~/tgortc

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f

# 配置防火墙
./deploy.sh firewall

# 更新服务
./deploy.sh update
```

---

## 🔧 LiveKit 集群部署

### 架构说明

```
                     ┌─────────────────────────────────────────┐
                     │           主服务器 (Master)              │
                     │  ┌─────────────────────────────────────┐│
                     │  │ TgoRTC Server + MySQL + Redis       ││
                     │  │ LiveKit Node + Nginx (负载均衡)      ││
                     │  └─────────────────────────────────────┘│
                     └────────────────┬────────────────────────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              │                       │                       │
              ▼                       ▼                       ▼
   ┌──────────────────┐   ┌──────────────────┐   ┌──────────────────┐
   │  LiveKit Node 1  │   │  LiveKit Node 2  │   │  LiveKit Node 3  │
   │  (独立服务器)     │   │  (独立服务器)     │   │  (独立服务器)     │
   └──────────────────┘   └──────────────────┘   └──────────────────┘
```

### 步骤 1: 部署主服务器

```bash
# 在主服务器上执行
curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy.sh | sudo bash -s -- --cn
```

部署完成后，记录以下信息：
- Redis 密码（在 `.env` 文件中的 `REDIS_PASSWORD`）
- LiveKit API Key（在 `.env` 文件中的 `LIVEKIT_API_KEY`）
- LiveKit API Secret（在 `.env` 文件中的 `LIVEKIT_API_SECRET`）

### 步骤 2: 开放主服务器端口

确保主服务器开放以下端口给 LiveKit 节点访问：
- **6380** (TCP): Redis
- **8080** (TCP): TgoRTC Server

### 步骤 3: 部署 LiveKit 节点

在每台额外的服务器上执行：

```bash
# 使用参数模式
curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy-livekit-node.sh | sudo bash -s -- \
    --cn \
    --master-ip <主服务器IP> \
    --redis-password <Redis密码> \
    --livekit-key <LiveKit API Key> \
    --livekit-secret <LiveKit API Secret>

# 或使用交互模式
curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy-livekit-node.sh -o deploy-livekit-node.sh
chmod +x deploy-livekit-node.sh
sudo ./deploy-livekit-node.sh
```

### 步骤 4: 配置主服务器负载均衡

在主服务器上添加新节点：

```bash
cd ~/tgortc

# 编辑 .env 文件，添加所有 LiveKit 节点
# LIVEKIT_NODES=192.168.1.101:7880,192.168.1.102:7880
nano .env

# 重新生成 Nginx 配置
./deploy.sh reload-nginx
```

### 步骤 5: 验证集群

```bash
# 在主服务器上查看状态
docker compose ps

# 测试 API
curl http://localhost:8080/health
```

---

## 📋 端口说明

### 主服务器需要开放的端口

| 端口 | 协议 | 用途 | 开放范围 |
|------|------|------|----------|
| 80 | TCP | Nginx 负载均衡（LiveKit 入口） | 公网 |
| 8080 | TCP | TgoRTC API | 公网/内网 |
| 8081 | TCP | Adminer 数据库管理 | 仅内网 |
| 3307 | TCP | MySQL | 仅内网 |
| 6380 | TCP | Redis | LiveKit 节点 |
| 7880 | TCP | LiveKit HTTP | 公网 |
| 7881 | TCP | LiveKit RTC TCP | 公网 |
| 3478 | UDP | TURN UDP | 公网 |
| 5349 | TCP | TURN TLS | 公网 |
| 50000-50100 | UDP | WebRTC 媒体 | 公网 |

### LiveKit 节点需要开放的端口

| 端口 | 协议 | 用途 |
|------|------|------|
| 7880 | TCP | LiveKit HTTP |
| 7881 | TCP | LiveKit RTC TCP |
| 3478 | UDP | TURN UDP |
| 5349 | TCP | TURN TLS |
| 50000-50100 | UDP | WebRTC 媒体 |

---

## 🔄 常用命令

```bash
cd ~/tgortc

# 查看状态
./deploy.sh status
docker compose ps

# 查看日志
docker compose logs -f
docker compose logs tgo-rtc-server -f

# 更新 TgoRTC
./deploy.sh update

# 完整更新（所有镜像）
./deploy.sh update --full

# 回滚
./deploy.sh rollback

# 重启服务
docker compose restart

# 停止服务
docker compose down

# 清理所有数据（危险）
./deploy.sh clean
```

---

## 🛠 手动部署

### 1. 构建并推送镜像（本地）

```bash
# 克隆项目
git clone https://github.com/TgoRTC/TgoRTCServer.git
cd TgoRTCServer

# 构建镜像
make deploy

# 或手动构建
docker build -t registry.cn-shanghai.aliyuncs.com/yourname/tgo-rtc-server:latest . --platform linux/amd64
docker push registry.cn-shanghai.aliyuncs.com/yourname/tgo-rtc-server:latest
```

### 2. 服务器部署

```bash
# 下载脚本
wget https://raw.githubusercontent.com/TgoRTC/TgoRTCServer/main/scripts/deploy.sh
chmod +x deploy.sh

# 自定义镜像地址
DOCKER_IMAGE=your-registry/your-image:tag ./deploy.sh
```

---

## 📝 配置说明

### .env 文件

```bash
# MySQL 配置
DB_USER=root
DB_PASSWORD=<自动生成>
DB_NAME=tgo_rtc

# Redis 配置
REDIS_PASSWORD=<自动生成>

# LiveKit 配置
LIVEKIT_API_KEY=<自动生成>
LIVEKIT_API_SECRET=<自动生成>

# LiveKit 集群节点（可选）
LIVEKIT_NODES=192.168.1.101:7880,192.168.1.102:7880

# 业务 Webhook（可选）
BUSINESS_WEBHOOK_ENDPOINTS='[{"url":"https://your-api.com/webhook","secret":"xxx"}]'
```

### livekit.yaml

```yaml
port: 7880

rtc:
  port_range_start: 50000
  port_range_end: 50100
  node_ip: <服务器公网IP>

turn:
  enabled: true
  domain: <服务器公网IP或域名>

redis:
  address: redis:6379
  password: <Redis密码>

webhook:
  api_key: <LiveKit API Key>
  urls:
    - http://tgo-rtc-server:8080/api/v1/webhooks/livekit
```

---

## ❓ 常见问题

### 1. Docker 镜像拉取失败

```bash
# 使用 --cn 参数启用国内镜像
curl -fsSL ... | sudo bash -s -- --cn
```

### 2. 端口被占用

```bash
# 检查端口占用
lsof -i :80
lsof -i :8080

# 停止占用进程或修改端口
```

### 3. LiveKit 节点无法连接 Redis

- 确保主服务器防火墙开放 6380 端口
- 确保云安全组允许 LiveKit 节点 IP 访问

### 4. 数据库连接失败

```bash
# 如果是密码不匹配（旧数据库），清理后重新部署
./deploy.sh clean
./deploy.sh
```

---

## 📚 更多文档

- [API 文档](http://your-server-ip:8080/swagger/index.html)
- [LiveKit 官方文档](https://docs.livekit.io/)

---

## 📄 许可证

MIT License
