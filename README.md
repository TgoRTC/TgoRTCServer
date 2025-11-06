# TgoRTC Server

基于 LiveKit 的实时音视频通话服务，提供房间管理、参与者管理和通话状态查询等功能。

## 功能特性

- 🎥 **音视频通话** - 支持语音通话和视频通话
- 🏠 **房间管理** - 创建房间、邀请参与者、加入/离开房间
- 👥 **参与者管理** - 查询通话状态、管理参与者权限
- 🔔 **事件通知** - 支持业务 Webhook 回调
- 🌐 **多语言支持** - 支持中文、英文等多语言
- 📊 **Swagger 文档** - 完整的 API 文档和在线调试

## 技术栈

- **Go 1.24+** - 后端开发语言
- **Gin** - Web 框架
- **GORM** - ORM 框架
- **MySQL 8.0+** - 数据库
- **Redis 7+** - 缓存
- **LiveKit** - 实时音视频引擎

## 快速开始

### 1. 环境准备

```bash
# 克隆项目
git clone https://github.com/TgoRTC/TgoRTCServer.git
cd TgoRTCServer

# 配置环境变量
cp .env.example .env
# 编辑 .env 文件，配置数据库和 LiveKit 连接信息
```

### 2. 启动服务

```bash
# 本地开发
go run main.go

# 或使用 Docker Compose
docker-compose -f docker-compose.prod.yml up -d
```

### 3. 访问服务

- **API 服务**: http://localhost:8080
- **Swagger 文档**: http://localhost:8080/swagger/index.html
- **健康检查**: http://localhost:8080/health

## LiveKit 多服务器部署

TgoRTC Server 支持连接到 LiveKit 集群，实现高可用和负载均衡。

### 部署架构

```
┌─────────────────┐
│  TgoRTC Server  │
│   (业务层)       │
└────────┬────────┘
         │
         ├─────────────────────────────┐
         │                             │
    ┌────▼─────┐                 ┌────▼─────┐
    │ LiveKit  │                 │ LiveKit  │
    │ Server 1 │◄───────────────►│ Server 2 │
    └──────────┘                 └──────────┘
         │                             │
         └─────────────┬───────────────┘
                       │
                  ┌────▼─────┐
                  │  Redis   │
                  │ (信令同步) │
                  └──────────┘
```

### 配置说明

#### 1. 单 LiveKit 服务器

**TgoRTC Server 配置（.env）：**
```env
LIVEKIT_URL=http://livekit.example.com:7880
LIVEKIT_API_KEY=devkey
LIVEKIT_API_SECRET=secret
```

**LiveKit Server 配置（livekit.yaml）：**
```yaml
port: 7880
keys:
  devkey: secret  # 与 .env 中的 API_KEY 和 SECRET 对应
```

**说明：** LiveKit 的 API Key 和 Secret 是在 `livekit.yaml` 中自己定义的，然后在 TgoRTC Server 的 `.env` 文件中配置相同的值。

#### 2. LiveKit 集群（多服务器）

LiveKit 支持通过 Redis 实现多服务器集群：

**LiveKit Server 1 配置：**
```yaml
# livekit-server1.yaml
port: 7880
redis:
  address: redis.example.com:6379
  db: 0
```

**LiveKit Server 2 配置：**
```yaml
# livekit-server2.yaml
port: 7880
redis:
  address: redis.example.com:6379
  db: 0
```

**TgoRTC Server 配置：**
```env
# 使用负载均衡器地址或任一 LiveKit 服务器地址
LIVEKIT_URL=http://livekit-lb.example.com:7880
LIVEKIT_API_KEY=your_api_key
LIVEKIT_API_SECRET=your_api_secret
```

### 负载均衡

使用 Nginx 作为 LiveKit 集群的负载均衡器：

**Nginx 配置示例：**
```nginx
upstream livekit_cluster {
    server livekit1.example.com:7880;
    server livekit2.example.com:7880;
}

server {
    listen 80;
    server_name livekit.example.com;

    location / {
        proxy_pass http://livekit_cluster;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 集群优势

- ✅ **高可用** - 单个 LiveKit 服务器故障不影响整体服务
- ✅ **负载均衡** - 自动分配房间到不同服务器
- ✅ **水平扩展** - 根据负载动态增加服务器
- ✅ **信令同步** - 通过 Redis 实现服务器间通信

## API 接口

### 房间管理

- `POST /api/v1/rooms` - 创建房间
- `POST /api/v1/rooms/{room_id}/invite` - 邀请参与者
- `POST /api/v1/rooms/{room_id}/join` - 加入房间
- `POST /api/v1/rooms/{room_id}/leave` - 离开房间

### 参与者管理

- `POST /api/v1/participants/calling` - 查询正在通话的成员

详细 API 文档请访问 Swagger UI。

## 测试

```bash
# 运行 E2E 测试
make e2e-local

# 查看测试指南
cat scripts/E2E_TEST_GUIDE.md
```

## 部署

详细部署文档请参考：

- [Docker Compose 部署](docs/guides/DOCKER_COMPOSE_DEPLOYMENT.md)
- [集群部署指南](docs/guides/CLUSTER_DEPLOYMENT.md)
- [快速参考](docs/guides/QUICK_REFERENCE.md)

## 许可证

MIT License

## 联系方式

- GitHub: https://github.com/TgoRTC/TgoRTCServer
- Issues: https://github.com/TgoRTC/TgoRTCServer/issues

