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

- **Go 1.23+** - 后端开发语言
- **Gin** - Web 框架
- **GORM** - ORM 框架
- **MySQL 8.0+** - 数据库
- **Redis 7+** - 缓存
- **LiveKit** - 实时音视频引擎

## 快速开始

### 1. 环境准备

```bash
# 克隆项目
git clone https://github.com/panyuQ/TgoRTCServer.git
cd TgoRTCServer

# 配置环境变量
cp .env.example .env
# 编辑 .env 文件，配置数据库、Redis 和 LiveKit 连接信息
```

### 2. 本地开发

```bash
# 直接运行
make run

# 或构建后运行
make build
./tgo-rtc-server
```

### 3. Docker 部署

```bash
# 构建并推送镜像
make deploy

# 启动所有服务
make up

# 查看日志
make logs

# 更新服务（拉取最新镜像并重启）
make update

# 停止服务
make stop
```

### 4. 访问服务

- **API 服务**: http://localhost:8080
- **Swagger 文档**: http://localhost:8080/swagger/index.html
- **健康检查**: http://localhost:8080/health

## 项目结构

```
TgoRTCServer/
├── main.go                 # 入口文件
├── Dockerfile              # Docker 构建文件
├── docker-compose.yml      # 服务编排配置
├── Makefile                # 构建部署命令
├── internal/               # 内部代码
│   ├── config/             # 配置管理
│   ├── database/           # 数据库连接和迁移
│   ├── handler/            # HTTP 处理器
│   ├── models/             # 数据模型
│   ├── router/             # 路由配置
│   ├── service/            # 业务逻辑
│   └── utils/              # 工具函数
├── migrations/             # 数据库迁移脚本
└── docs/                   # 文档
```

## 环境变量配置

```env
# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=tgo_rtc

# Redis 配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your_password

# LiveKit 配置
LIVEKIT_URL=http://localhost:7880
LIVEKIT_API_KEY=your_api_key
LIVEKIT_API_SECRET=your_api_secret
```

## API 接口

### 房间管理

- `POST /api/v1/rooms` - 创建房间
- `POST /api/v1/rooms/{room_id}/invite` - 邀请参与者
- `POST /api/v1/rooms/{room_id}/join` - 加入房间
- `POST /api/v1/rooms/{room_id}/leave` - 离开房间

### 参与者管理

- `POST /api/v1/participants/calling` - 查询正在通话的成员

详细 API 文档请访问 Swagger UI。

## Make 命令

```bash
make help       # 显示帮助信息
make build      # 构建本地二进制
make run        # 本地运行
make test       # 运行测试
make fmt        # 格式化代码
make deploy     # 构建并推送镜像
make up         # 启动服务
make update     # 更新服务
make stop       # 停止服务
make logs       # 查看日志
```

## 二次开发

如需修改镜像仓库地址，只需编辑 `Makefile` 中的配置：

```makefile
REGISTRY := your-registry.com
NAMESPACE := your-namespace
IMAGE_NAME := your-image-name
TAG := latest
```

## 许可证

MIT License

## 联系方式

- GitHub: https://github.com/panyuQ/TgoRTCServer
- Issues: https://github.com/panyuQ/TgoRTCServer/issues
