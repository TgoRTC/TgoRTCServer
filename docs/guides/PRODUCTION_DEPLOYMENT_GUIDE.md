# TgoRTC Server 生产环境部署指南

本文档记录了 TgoRTC Server 从零到部署成功的完整步骤。

---

## 📋 目录

- [环境要求](#环境要求)
- [部署架构](#部署架构)
- [部署步骤](#部署步骤)
- [验证部署](#验证部署)
- [常用管理命令](#常用管理命令)
- [故障排除](#故障排除)

---

## 环境要求

### 服务器要求

- **操作系统**: Linux (CentOS/Ubuntu/Debian)
- **CPU**: 2 核心以上
- **内存**: 4GB 以上
- **磁盘**: 20GB 以上
- **网络**: 公网 IP（如果需要外网访问）

### 软件要求

- Docker 20.10+
- Docker Compose v2.0+

### 本地开发环境（用于构建镜像）

- Docker Desktop
- Go 1.24+（可选）
- Git

---

## 部署架构

```
┌─────────────────────────────────────────────────┐
│          服务器 (生产环境)                        │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌──────────────┐  ┌──────────────┐            │
│  │   MySQL 8.0  │  │   Redis 7    │            │
│  │   :3306      │  │   :6380      │            │
│  └──────────────┘  └──────────────┘            │
│                                                 │
│  ┌──────────────────────────────────┐          │
│  │      LiveKit Server              │          │
│  │      :7880-7881                  │          │
│  │      UDP :50000-50100            │          │
│  └──────────────────────────────────┘          │
│                                                 │
│  ┌──────────────────────────────────┐          │
│  │      TgoRTC API Server           │          │
│  │      :8080                       │          │
│  └──────────────────────────────────┘          │
│                                                 │
└─────────────────────────────────────────────────┘
```

---

## 部署步骤

### 步骤 1：安装 Docker

```bash
# 1.1 安装 Docker
curl -fsSL https://get.docker.com | sh

# 1.2 启动 Docker
systemctl start docker
systemctl enable docker

# 1.3 验证 Docker 安装
docker --version
# 输出示例：Docker version 24.0.7, build afdd53b
```

### 步骤 2：验证 Docker Compose

```bash
# 2.1 检查 Docker Compose 版本
docker compose version
# 输出示例：Docker Compose version v2.27.1
```

**说明：** 新版 Docker 内置了 Compose 插件，使用 `docker compose` 命令（不是 `docker-compose`）。

### 步骤 3：创建项目目录

```bash
# 3.1 创建项目目录
mkdir -p /opt/tgo-rtc
cd /opt/tgo-rtc

# 3.2 确认当前目录
pwd
# 输出：/opt/tgo-rtc
```

### 步骤 4：创建 docker-compose.yml

```bash
vim docker-compose.yml
```

**内容：**

```yaml
version: '3.8'

services:
  mysql:
    image: crpi-4ja8peh93d2yb8c8.cn-shanghai.personal.cr.aliyuncs.com/slun/mysql:amd64
    container_name: tgo-rtc-mysql
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_PASSWORD}
      MYSQL_DATABASE: ${DB_NAME}
      TZ: Asia/Shanghai
    volumes:
      - mysql_data:/var/lib/mysql
    ports:
      - "3306:3306"
    networks:
      - tgo-rtc-network
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: crpi-4ja8peh93d2yb8c8.cn-shanghai.personal.cr.aliyuncs.com/slun/redis:amd64
    container_name: tgo-rtc-redis
    restart: always
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    ports:
      - "6380:6379" # 如果是多台livekit节点，需要修改为 "0.0.0.0:6380:6379"
    networks:
      - tgo-rtc-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  livekit:
    image: crpi-4ja8peh93d2yb8c8.cn-shanghai.personal.cr.aliyuncs.com/slun/livekit:amd64
    container_name: tgo-rtc-livekit
    restart: always
    command: --config /etc/livekit.yaml
    volumes:
      - ./livekit.yaml:/etc/livekit.yaml
    ports:
      - "7880:7880"
      - "7881:7881"
      - "50000-50100:50000-50100/udp"
    networks:
      - tgo-rtc-network

  tgo-rtc-server:
    image: crpi-4ja8peh93d2yb8c8.cn-shanghai.personal.cr.aliyuncs.com/slun/tgortc:latest
    container_name: tgo-rtc-server
    restart: always
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=mysql
      - DB_PORT=3306
      - DB_USER=${DB_USER}
      - DB_PASSWORD=${DB_PASSWORD}
      - DB_NAME=${DB_NAME}
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - REDIS_DB=0
      - LIVEKIT_URL=http://livekit:7880
      - LIVEKIT_API_KEY=${LIVEKIT_API_KEY}
      - LIVEKIT_API_SECRET=${LIVEKIT_API_SECRET}
      - PORT=8080
      - BUSINESS_WEBHOOK_URL=${BUSINESS_WEBHOOK_URL}
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_healthy
      livekit:
        condition: service_started
    networks:
      - tgo-rtc-network
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  mysql_data:
  redis_data:

networks:
  tgo-rtc-network:
    driver: bridge
```

**说明：**
- Redis 端口映射为 `6380:6379`（避免与宿主机 Redis 冲突）
- 镜像地址使用阿里云容器镜像服务（国内访问更快）

### 步骤 5：创建 .env 环境变量文件

```bash
vim .env
```

**内容：**

```env
# MySQL 配置
DB_USER=root
DB_PASSWORD=TgoRTC@2025
DB_NAME=tgo_rtc

# Redis 配置
REDIS_PASSWORD=TgoRedis@2025

# LiveKit 配置
LIVEKIT_API_KEY=prodkey
LIVEKIT_API_SECRET=Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9

# 业务 Webhook（可选，如果没有可以留空）
BUSINESS_WEBHOOK_URL=
```

**安全建议：**
- 生产环境请修改为强密码
- `LIVEKIT_API_SECRET` 可以使用 `openssl rand -base64 32` 生成

### 步骤 6：创建 livekit.yaml 配置文件

```bash
vim livekit.yaml
```

**内容：**

```yaml
port: 7880
rtc:
  port_range_start: 50000
  port_range_end: 50100
  use_external_ip: true

keys:
  prodkey: Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9
```

**重要说明：**
- `prodkey` 是 API Key（对应 `.env` 中的 `LIVEKIT_API_KEY`）
- `Xj9K2mP5...` 是 Secret（对应 `.env` 中的 `LIVEKIT_API_SECRET`）
- 两者必须完全一致

### 步骤 7：检查配置文件

```bash
# 查看当前目录的文件
ls -la

# 应该看到以下文件：
# - docker-compose.yml
# - .env
# - livekit.yaml
```

---

## 构建和推送 Docker 镜像

### 步骤 8：在本地构建镜像（MacBook/开发机）

#### 8.1 准备阿里云容器镜像服务

1. 访问：https://cr.console.aliyun.com
2. 创建命名空间（例如：`slun`）
3. 创建以下镜像仓库（仓库类型选择**公开**）：
   - `mysql`
   - `redis`
   - `livekit`
   - `tgortc`（你的应用）

#### 8.2 登录阿里云镜像仓库

```bash
# 替换为你的用户名和镜像仓库地址
docker login --username=你的用户名 crpi-xxx.cn-shanghai.personal.cr.aliyuncs.com
```

#### 8.3 构建并推送应用镜像

```bash
# 进入项目目录
cd /path/to/TgoRTCServer

# 使用 buildx 构建 AMD64 架构镜像并推送
docker buildx build --platform linux/amd64 \
  -t crpi-xxx.cn-shanghai.personal.cr.aliyuncs.com/你的命名空间/tgortc:latest \
  --push \
  .
```

**重要提示：**
- 如果你的开发机是 Apple Silicon (M 系列芯片)，必须使用 `--platform linux/amd64`
- 服务器通常是 AMD64 架构

#### 8.4 推送基础镜像（MySQL、Redis、LiveKit）

```bash
# 创建并使用 buildx builder
docker buildx create --name multiarch --use
docker buildx inspect --bootstrap

# 推送 MySQL
docker buildx build --platform linux/amd64 \
  -t crpi-xxx.cn-shanghai.personal.cr.aliyuncs.com/你的命名空间/mysql:amd64 \
  --push \
  - <<'EOF'
FROM mysql:8.0
EOF

# 推送 Redis
docker buildx build --platform linux/amd64 \
  -t crpi-xxx.cn-shanghai.personal.cr.aliyuncs.com/你的命名空间/redis:amd64 \
  --push \
  - <<'EOF'
FROM redis:7-alpine
EOF

# 推送 LiveKit
docker buildx build --platform linux/amd64 \
  -t crpi-xxx.cn-shanghai.personal.cr.aliyuncs.com/你的命名空间/livekit:amd64 \
  --push \
  - <<'EOF'
FROM livekit/livekit-server:latest
EOF
```

---

## 启动服务

### 步骤 9：修改 docker-compose.yml 中的镜像地址

将 `docker-compose.yml` 中的镜像地址替换为你的阿里云镜像地址。

### 步骤 10：登录阿里云镜像仓库（服务器上）

```bash
docker login --username=你的用户名 crpi-xxx.cn-shanghai.personal.cr.aliyuncs.com
```

### 步骤 11：拉取镜像

```bash
docker compose pull
```

### 步骤 12：启动所有服务

```bash
docker compose up -d
```

### 步骤 13：查看容器状态

```bash
docker compose ps
```

**预期输出：**

```
NAME              IMAGE                                    COMMAND                   STATUS
tgo-rtc-mysql     .../mysql:amd64                         "docker-entrypoint.s…"   Up (healthy)
tgo-rtc-redis     .../redis:amd64                         "docker-entrypoint.s…"   Up (healthy)
tgo-rtc-livekit   .../livekit:amd64                       "/livekit-server --c…"   Up
tgo-rtc-server    .../tgortc:latest                       "./tgo-rtc-server"        Up (healthy)
```

---

## 验证部署

### 1. 健康检查

```bash
curl http://localhost:8080/health
# 预期输出：{"status":"ok"}
```

### 2. 访问 Swagger 文档

浏览器访问：`http://服务器IP:8080/swagger/index.html`

### 3. 测试 API 接口

```bash
curl -X POST http://localhost:8080/api/v1/rooms \
  -H 'Content-Type: application/json' \
  -d '{
    "source_channel_id": "test_channel_001",
    "creator": "test_user_001",
    "rtc_type": 1,
    "uids": ["test_user_002"]
  }'
```

### 4. 查看日志

```bash
# 查看所有服务日志
docker compose logs -f

# 查看特定服务日志
docker compose logs -f tgo-rtc-server
```

---

## 常用管理命令

### 查看容器状态

```bash
docker compose ps
```

### 重启服务

```bash
# 重启所有服务
docker compose restart

# 重启特定服务
docker compose restart tgo-rtc-server
```

### 停止服务

```bash
docker compose down
```

### 更新服务

```bash
# 拉取最新镜像
docker compose pull

# 重新启动
docker compose up -d
```

### 查看日志

```bash
# 实时查看日志
docker compose logs -f

# 查看最近 100 行日志
docker compose logs --tail 100
```

### 进入容器

```bash
docker exec -it tgo-rtc-server sh
```

---

## 故障排除

### 问题 1：端口被占用

**错误信息：**
```
Error: Bind for 0.0.0.0:8080 failed: port is already allocated
```

**解决方案：**
```bash
# 查看占用端口的进程
lsof -i:8080

# 停止占用端口的进程
kill <PID>

# 或修改 docker-compose.yml 中的端口映射
```

### 问题 2：容器启动失败

**解决方案：**
```bash
# 查看容器日志
docker logs tgo-rtc-server

# 查看详细错误信息
docker compose logs tgo-rtc-server
```

### 问题 3：数据库连接失败

**检查：**
1. MySQL 容器是否健康：`docker compose ps`
2. 环境变量是否正确：`cat .env`
3. 查看应用日志：`docker logs tgo-rtc-server`

### 问题 4：Swagger 文档为空

**原因：** Dockerfile 中运行了 `swag init` 覆盖了手动维护的文档

**解决方案：**
1. 确保 `.dockerignore` 中没有忽略 `docs/` 目录
2. 确保 Dockerfile 不运行 `swag init`
3. 重新构建镜像

---

## 配置信息汇总

### 访问地址

- **Swagger 文档**: http://服务器IP:8080/swagger/index.html
- **API 文档 JSON**: http://服务器IP:8080/api/docs/swagger.json
- **健康检查**: http://服务器IP:8080/health
- **API 基础路径**: http://服务器IP:8080/api/v1

### 端口映射

| 服务 | 容器端口 | 宿主机端口 | 说明 |
|------|---------|-----------|------|
| MySQL | 3306 | 3306 | 数据库 |
| Redis | 6379 | 6380 | 缓存（避免冲突改为 6380） |
| LiveKit | 7880-7881 | 7880-7881 | 信令端口 |
| LiveKit | 50000-50100/udp | 50000-50100/udp | RTC 媒体端口 |
| TgoRTC API | 8080 | 8080 | API 服务 |

### 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| DB_PASSWORD | TgoRTC@2024 | MySQL 密码 |
| REDIS_PASSWORD | TgoRedis@2024 | Redis 密码 |
| LIVEKIT_API_KEY | prodkey | LiveKit API Key |
| LIVEKIT_API_SECRET | Xj9K2mP5... | LiveKit Secret |

---

## 下一步优化

### 1. 配置 Nginx 反向代理

参考：`docs/guides/SERVER_DEPLOYMENT.md`

### 2. 配置 HTTPS

使用 Let's Encrypt 免费证书

### 3. 配置防火墙

```bash
# 开放必要端口
firewall-cmd --permanent --add-port=8080/tcp
firewall-cmd --permanent --add-port=7880-7881/tcp
firewall-cmd --permanent --add-port=50000-50100/udp
firewall-cmd --reload
```

### 4. 数据备份

```bash
# 备份 MySQL 数据
docker exec tgo-rtc-mysql mysqldump -uroot -p${DB_PASSWORD} tgo_rtc > backup.sql

# 备份 Redis 数据
docker exec tgo-rtc-redis redis-cli -a ${REDIS_PASSWORD} SAVE
```

---

## 关键注意事项

### ⚠️ 架构兼容性

**问题：** Apple Silicon (M 系列芯片) 是 ARM64 架构，服务器通常是 AMD64 架构

**解决方案：**
- 必须使用 `docker buildx build --platform linux/amd64` 构建镜像
- 不能使用 `docker pull --platform linux/amd64`（会拉取 ARM64 版本）
- 推荐使用 `docker buildx` 交叉编译

### ⚠️ Swagger 文档

**问题：** 项目使用手动维护的 swagger.yaml/swagger.json，不是通过注解自动生成

**注意事项：**
- ❌ 不要运行 `swag init`（会覆盖手动维护的文档）
- ✅ 确保 `.dockerignore` 中没有忽略 `docs/` 目录
- ✅ Dockerfile 中不要运行 `swag init`

### ⚠️ LiveKit API Key 和 Secret

**重要概念：**
- **API Key**: 任意字符串，你自己定义（如 `prodkey`、`myapp`）
- **Secret**: 强密码，使用 `openssl rand -base64 32` 生成

**配置示例：**

```yaml
# livekit.yaml
keys:
  prodkey: Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9
  # ↑       ↑
  # API Key  Secret
```

```env
# .env
LIVEKIT_API_KEY=prodkey
LIVEKIT_API_SECRET=Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9
```

**两者必须完全一致！**

### ⚠️ Redis 端口冲突

如果服务器上已有 Redis 运行在 6379 端口：

**方案 1：** 停止现有 Redis
```bash
systemctl stop redis
systemctl disable redis
```

**方案 2：** 修改 Docker 中的 Redis 端口
```yaml
redis:
  ports:
    - "6380:6379"  # 宿主机使用 6380
```

---

## 总结

本文档记录了 TgoRTC Server 的完整部署流程，包括：

✅ Docker 环境准备
✅ 配置文件创建
✅ 镜像构建和推送（AMD64 架构）
✅ 服务启动和验证
✅ 常见问题解决
✅ 关键注意事项

### 部署检查清单

- [ ] Docker 和 Docker Compose 已安装
- [ ] 阿里云镜像仓库已创建
- [ ] 镜像已构建并推送（AMD64 架构）
- [ ] docker-compose.yml 已创建
- [ ] .env 环境变量已配置
- [ ] livekit.yaml 已创建
- [ ] 所有容器状态为 healthy
- [ ] 健康检查返回 `{"status":"ok"}`
- [ ] Swagger 文档可访问
- [ ] API 接口测试通过

### 相关文档

- [服务器部署指南](./SERVER_DEPLOYMENT.md) - 详细的部署说明
- [LiveKit 配置指南](./LIVEKIT_CONFIG.md) - LiveKit API Key 和 Secret 配置
- [E2E 测试指南](../scripts/E2E_TEST_GUIDE.md) - 端到端测试

如有问题，请参考项目文档或提交 Issue。

---

**文档版本**: 1.0
**更新日期**: 2025-11-05
**作者**: TgoRTC Team

