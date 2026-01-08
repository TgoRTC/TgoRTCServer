# TgoRTC Server 快速入门指南

> 一键部署、集群配置、二次开发完整指南

---

## 目录

- [一、一键部署 TgoRTC Server（主服务器）](#一一键部署-tgortc-server主服务器)
- [二、一键部署 LiveKit 节点（集群扩展）](#二一键部署-livekit-节点集群扩展)
- [三、二次开发与自定义镜像](#三二次开发与自定义镜像)
- [四、端口说明](#四端口说明)
- [五、常用命令](#五常用命令)

---

## 一、一键部署 TgoRTC Server（主服务器）

### 1.1 快速部署

```bash
# 国内服务器（推荐，使用镜像加速）
curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy.sh | sudo bash -s -- --cn

# 海外服务器
curl -fsSL https://raw.githubusercontent.com/TgoRTC/TgoRTCServer/main/scripts/deploy.sh | sudo bash
```

### 1.2 部署完成后

部署完成后会自动生成配置并启动服务，记录以下信息（用于后续集群配置）：

```bash
# 查看生成的配置
cat ~/tgortc/.env
```

需要记录的关键信息：
- `REDIS_PASSWORD` - Redis 密码
- `LIVEKIT_API_KEY` - LiveKit API Key
- `LIVEKIT_API_SECRET` - LiveKit API Secret

### 1.3 部署后操作

```bash
cd ~/tgortc

# 查看服务状态
sudo docker compose ps

# 查看日志
sudo docker compose logs -f

# 配置系统防火墙
sudo ./deploy.sh firewall

# 更新服务
sudo ./deploy.sh update
```

### 1.4 访问地址

| 服务 | 地址 |
|------|------|
| TgoRTC API | `http://<服务器IP>:8080` |
| API 文档 | `http://<服务器IP>:8080/swagger/index.html` |
| LiveKit | `ws://<服务器IP>:80` |
| 数据库管理 | `http://<服务器IP>:8081` |

---

## 二、一键部署 LiveKit 节点（集群扩展）

### 2.1 架构说明

```
┌─────────────────────────────────────────────────────────────┐
│                    主服务器 (Master)                         │
│  TgoRTC Server + MySQL + Redis + LiveKit + Nginx            │
└─────────────────────────────────────────────────────────────┘
                           ↑
                     Redis 同步 (6380)
                     Webhook 回调 (8080)
                           ↓
     ┌─────────────────────┼─────────────────────┐
     │                     │                     │
     ▼                     ▼                     ▼
┌─────────┐          ┌─────────┐          ┌─────────┐
│LiveKit  │          │LiveKit  │          │LiveKit  │
│Node 1   │          │Node 2   │          │Node 3   │
└─────────┘          └─────────┘          └─────────┘
```

### 2.2 部署 LiveKit 节点

在新的服务器上执行：

```bash
# 使用 IP 作为 Webhook 地址
curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy-livekit-node.sh | sudo bash -s -- \
    --cn \
    --master-ip <主服务器IP> \
    --redis-password "<Redis密码>" \
    --livekit-key "<LiveKit API Key>" \
    --livekit-secret "<LiveKit API Secret>"

# 使用域名作为 Webhook 地址
curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy-livekit-node.sh | sudo bash -s -- \
    --cn \
    --master-ip <主服务器IP> \
    --redis-password "<Redis密码>" \
    --livekit-key "<LiveKit API Key>" \
    --livekit-secret "<LiveKit API Secret>" \
    --tgortc-url "https://api.example.com"
```

**示例：**

```bash
curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy-livekit-node.sh | sudo bash -s -- \
    --cn \
    --master-ip 47.117.96.203 \
    --redis-password "TgoRedis@2025" \
    --livekit-key "prodkey" \
    --livekit-secret "Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9"
```

### 2.3 配置主服务器负载均衡

节点部署完成后，需要在主服务器上添加节点：

```bash
# 在主服务器上执行
cd ~/tgortc

# 编辑 .env 文件，添加节点 IP
nano .env

# 添加或修改这一行（多个节点用逗号分隔）：
# LIVEKIT_NODES=节点1IP:7880,节点2IP:7880

# 重新加载 Nginx 配置
sudo ./deploy.sh reload-nginx
```

### 2.4 验证集群

```bash
# 在主服务器上测试节点连通性
curl http://<节点IP>:7880

# 查看节点信息
cd ~/livekit-node
sudo ./deploy-livekit-node.sh info
```

---

## 三、二次开发与自定义镜像

### 3.1 修改镜像地址

编辑 `Makefile`，修改镜像地址：

```makefile
# 修改这一行为你的镜像地址
DOCKER_IMAGE ?= your-registry.com/your-namespace/tgortc:latest
```

**示例（阿里云）：**

```makefile
DOCKER_IMAGE ?= registry.cn-shanghai.aliyuncs.com/yourname/tgortc:latest
```

**示例（Docker Hub）：**

```makefile
DOCKER_IMAGE ?= yourusername/tgortc:latest
```

### 3.2 构建并推送镜像

```bash
# 方式一：使用 make 命令（推荐）
make deploy

# 方式二：手动执行
# 1. 构建镜像
docker build -t your-registry.com/your-namespace/tgortc:latest . --platform linux/amd64

# 2. 登录镜像仓库
docker login your-registry.com

# 3. 推送镜像
docker push your-registry.com/your-namespace/tgortc:latest
```

### 3.3 使用自定义镜像部署

**方式一：修改部署脚本后推送到你的仓库**

1. Fork 项目到你的 GitHub/Gitee
2. 修改 `scripts/deploy.sh` 中的 `DOCKER_IMAGE` 变量：

```bash
# 修改这一行
DOCKER_IMAGE="${DOCKER_IMAGE:-your-registry.com/your-namespace/tgortc:latest}"
```

3. 推送到你的仓库
4. 使用你的仓库地址部署

**方式二：部署时指定镜像地址**

```bash
# 下载脚本
curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy.sh -o deploy.sh
chmod +x deploy.sh

# 使用环境变量指定镜像
DOCKER_IMAGE=your-registry.com/your-namespace/tgortc:latest sudo -E ./deploy.sh --cn
```

**方式三：部署后修改镜像地址**

```bash
cd ~/tgortc

# 编辑 .env 文件
nano .env

# 修改 DOCKER_IMAGE 变量
# DOCKER_IMAGE=your-registry.com/your-namespace/tgortc:latest

# 重新拉取并启动
sudo docker compose pull tgo-rtc-server
sudo docker compose up -d tgo-rtc-server
```

### 3.4 完整二次开发流程

```bash
# 1. 克隆项目
git clone https://github.com/TgoRTC/TgoRTCServer.git
cd TgoRTCServer

# 2. 修改代码...

# 3. 修改 Makefile 中的镜像地址
nano Makefile
# 修改: DOCKER_IMAGE ?= your-registry.com/your-namespace/tgortc:latest

# 4. 登录镜像仓库
docker login your-registry.com

# 5. 构建并推送
make deploy

# 6. 在服务器上更新
ssh your-server
cd ~/tgortc
DOCKER_IMAGE=your-registry.com/your-namespace/tgortc:latest ./deploy.sh update
```

### 3.5 阿里云容器镜像服务配置

```bash
# 1. 创建命名空间和仓库
# 登录阿里云控制台 -> 容器镜像服务 -> 创建个人实例 -> 创建命名空间 -> 创建镜像仓库（公开）

# 2. 登录阿里云镜像仓库
docker login --username=your-aliyun-account registry.cn-shanghai.aliyuncs.com

# 3. 修改 Makefile
DOCKER_IMAGE ?= registry.cn-shanghai.aliyuncs.com/your-namespace/tgortc:latest

# 4. 构建并推送
make deploy
```

---

## 四、端口说明

### 4.1 主服务器端口

| 端口 | 协议 | 用途 | 开放范围 |
|------|------|------|----------|
| 80 | TCP | Nginx 负载均衡（LiveKit 入口） | 公网 |
| 8080 | TCP | TgoRTC API | 公网 |
| 8081 | TCP | Adminer 数据库管理 | 仅内网 |
| 3307 | TCP | MySQL | 仅内网 |
| 6380 | TCP | Redis | LiveKit 节点 |
| 7880 | TCP | LiveKit HTTP | 公网 |
| 7881 | TCP | LiveKit RTC TCP | 公网 |
| 3478 | UDP | TURN UDP | 公网 |
| 5349 | TCP | TURN TLS | 公网 |
| 50000-50100 | UDP | WebRTC 媒体 | 公网 |

### 4.2 LiveKit 节点端口

| 端口 | 协议 | 用途 |
|------|------|------|
| 7880 | TCP | LiveKit HTTP/WebSocket |
| 7881 | TCP | LiveKit RTC TCP |
| 3478 | UDP | TURN UDP |
| 5349 | TCP | TURN TLS |
| 50000-50100 | UDP | WebRTC 媒体 |

---

## 五、常用命令

### 5.1 主服务器命令

```bash
cd ~/tgortc

# 查看状态
sudo ./deploy.sh status
sudo docker compose ps

# 查看日志
sudo docker compose logs -f
sudo docker compose logs tgo-rtc-server -f

# 更新 TgoRTC（快速更新）
sudo ./deploy.sh update

# 完整更新（所有镜像）
sudo ./deploy.sh update --full

# 更新到指定版本
sudo ./deploy.sh update v1.2.0

# 回滚到上一版本
sudo ./deploy.sh rollback

# 配置防火墙
sudo ./deploy.sh firewall

# 重新加载 Nginx（添加节点后）
sudo ./deploy.sh reload-nginx

# 查看版本信息
sudo ./deploy.sh version

# 重启服务
sudo docker compose restart

# 停止服务
sudo docker compose down

# 清理所有数据（危险！）
sudo ./deploy.sh clean
```

### 5.2 LiveKit 节点命令

```bash
cd ~/livekit-node

# 查看节点信息
sudo ./deploy-livekit-node.sh info

# 查看状态
sudo ./deploy-livekit-node.sh status

# 查看日志
sudo ./deploy-livekit-node.sh logs

# 重启服务
sudo ./deploy-livekit-node.sh restart

# 更新 LiveKit
sudo ./deploy-livekit-node.sh update

# 配置防火墙
sudo ./deploy-livekit-node.sh firewall

# 停止服务
sudo ./deploy-livekit-node.sh stop
```

---

## 附录：配置文件说明

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

# Docker 镜像（二次开发时修改）
DOCKER_IMAGE=your-registry.com/your-namespace/tgortc:latest

# LiveKit 集群节点（添加节点后配置）
LIVEKIT_NODES=192.168.1.101:7880,192.168.1.102:7880

# 业务 Webhook（可选）
BUSINESS_WEBHOOK_ENDPOINTS='[{"url":"https://your-api.com/webhook","secret":"xxx"}]'
```

### livekit.yaml 关键配置

```yaml
keys:
  prodkey: <secret>

redis:
  address: <主服务器IP>:6380
  password: <Redis密码>
  db: 0

webhook:
  api_key: prodkey
  urls:
    - http://<TgoRTC地址>/api/v1/webhooks/livekit
```

---

## 常见问题

### Q1: Docker 镜像拉取失败？

```bash
# 使用 --cn 参数启用国内镜像加速
curl -fsSL ... | sudo bash -s -- --cn
```

### Q2: 端口被占用？

```bash
# 检查端口占用
lsof -i :80
lsof -i :8080

# 停止占用的进程或修改端口
```

### Q3: LiveKit 节点无法连接 Redis？

- 确保主服务器防火墙开放 6380 端口
- 确保云安全组允许节点 IP 访问

### Q4: 数据库连接失败？

```bash
# 可能是旧数据残留，清理后重新部署
sudo ./deploy.sh clean
sudo ./deploy.sh
```

### Q5: 如何查看生成的密码？

```bash
cat ~/tgortc/.env | grep PASSWORD
cat ~/tgortc/.env | grep SECRET
```

---

## 联系方式

- GitHub: https://github.com/TgoRTC/TgoRTCServer
- Gitee: https://gitee.com/No8blackball/tgo-rtcserver
