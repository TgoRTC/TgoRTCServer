# TgoCallServer - 音视频服务接口业务服务

一个完整的基于 LiveKit 的音视频服务接口业务服务项目，提供房间管理、参与者管理等核心功能。

## ✨ 项目特点

- 🏗️ **分层架构**: 清晰的代码组织，易于维护和扩展
- 🎯 **完整功能**: 涵盖房间和参与者管理的所有核心功能
- 📚 **详细文档**: 完善的 API 文档和开发指南
- 🔐 **安全认证**: 集成 LiveKit Token 认证
- 💾 **数据持久化**: MySQL 数据库 + Redis 缓存
- 🚀 **快速部署**: 支持单机和分布式部署

## 🚀 快速开始

### 前置要求

- Go 1.21+
- MySQL 5.7+
- Redis 5.0+
- LiveKit 服务器

### 5 分钟快速启动

```bash
# 1. 配置环境变量
cp .env.example .env
nano .env

# 2. 创建数据库
mysql -u root -p -e "CREATE DATABASE tgo_call CHARACTER SET utf8mb4;"

# 3. 安装依赖
go mod download

# 4. 启动服务
go run main.go

# 5. 测试 API
curl http://localhost:8080/health
```

详见 [快速开始.md](快速开始.md)

## 📚 文档导航

| 文档 | 说明 |
|------|------|
| [快速开始.md](快速开始.md) | 🚀 5 分钟快速启动指南 |
| [API_文档.md](API_文档.md) | 📖 完整的 API 接口文档 |
| [开发指南.md](开发指南.md) | 🛠️ 本地开发环境和开发指南 |
| [项目结构说明.md](项目结构说明.md) | 🏗️ 项目架构和模块说明 |
| [数据库字段说明.md](数据库字段说明.md) | 🗄️ 数据库表结构说明 |
| [项目创建总结.md](项目创建总结.md) | 📋 项目创建总结 |
| [项目验证清单.md](项目验证清单.md) | ✅ 项目验证清单 |
| [部署架构指南.md](部署架构指南.md) | 🌐 部署架构说明 |
| [部署常见问题.md](部署常见问题.md) | ❓ 常见问题解答 |

## 🎯 核心功能

### 房间管理
- ✅ 创建房间
- ✅ 获取房间信息
- ✅ 列出房间列表
- ✅ 更新房间状态
- ✅ 结束房间

### 参与者管理
- ✅ 加入房间
- ✅ 离开房间
- ✅ 获取参与者列表
- ✅ 邀请参与者
- ✅ 更新参与者状态

## 🌐 API 端点

### 房间相关
```
POST   /api/rooms                      # 创建房间
GET    /api/rooms                      # 列出房间列表
GET    /api/rooms/:room_name           # 获取房间信息
PUT    /api/rooms/:room_name/status    # 更新房间状态
POST   /api/rooms/:room_name/end       # 结束房间
```

### 参与者相关
```
POST   /api/participants/join                              # 加入房间
POST   /api/participants/leave                             # 离开房间
GET    /api/rooms/:room_name/participants                  # 获取参与者列表
POST   /api/rooms/:room_name/invite                        # 邀请参与者
PUT    /api/rooms/:room_name/participants/:uid/status      # 更新参与者状态
```

### 其他
```
GET    /health                         # 健康检查
```

## 📊 项目结构

```
tgo-call-server/
├── main.go                          # 主程序入口
├── go.mod                           # Go 模块定义
├── .env.example                     # 环境变量示例
├── internal/
│   ├── config/                      # 配置管理
│   ├── database/                    # 数据库和 Redis
│   ├── models/                      # 数据模型
│   ├── service/                     # 业务逻辑
│   ├── handler/                     # API 处理器
│   ├── livekit/                     # LiveKit 集成
│   └── router/                      # 路由配置
└── 文档文件/
    ├── API_文档.md
    ├── 开发指南.md
    ├── 项目结构说明.md
    └── ...
```

## 🛠️ 技术栈

| 技术 | 用途 |
|------|------|
| Go 1.21 | 编程语言 |
| Gin | Web 框架 |
| GORM | ORM 框架 |
| MySQL | 关系数据库 |
| Redis | 缓存和消息队列 |
| LiveKit | 音视频服务 |
| JWT | 身份认证 |

## 📝 API 示例

### 创建房间

```bash
curl -X POST http://localhost:8080/api/rooms \
  -H "Content-Type: application/json" \
  -d '{
    "source_channel_id": "channel_123",
    "source_channel_type": 0,
    "creator": "user_001",
    "room_name": "meeting_001",
    "call_type": 1,
    "invite_on": 1
  }'
```

**响应**:
```json
{
  "code": 0,
  "msg": "房间创建成功",
  "data": {
    "id": 1,
    "room_name": "meeting_001",
    "token": "eyJhbGc...",
    "livekit_url": "http://localhost:7880",
    "status": 0,
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

### 加入房间

```bash
curl -X POST http://localhost:8080/api/participants/join \
  -H "Content-Type: application/json" \
  -d '{
    "room_name": "meeting_001",
    "uid": "user_002"
  }'
```

**响应**:
```json
{
  "code": 0,
  "msg": "加入房间成功",
  "data": {
    "id": 1,
    "room_name": "meeting_001",
    "uid": "user_002",
    "token": "eyJhbGc...",
    "status": 1
  }
}
```

## 🗄️ 数据库表

### livekit_room（房间表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | 主键 |
| source_channel_id | varchar(100) | 所属频道 ID |
| source_channel_type | smallint | 频道类型 |
| creator | varchar(40) | 房间发起者 |
| room_name | varchar(40) | 房间名称（唯一） |
| call_type | smallint | 呼叫类型（0=语音，1=视频） |
| invite_on | smallint | 是否开启邀请（0=否，1=是） |
| status | smallint | 房间状态（0=未开始，1=进行中，2=已结束，3=已取消） |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |

### livekit_participant（参与者表）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | 主键 |
| room_name | varchar(40) | 房间名称 |
| uid | varchar(40) | 用户 ID |
| status | smallint | 参与者状态（0-6，见文档） |
| join_time | bigint | 加入时间（Unix 时间戳） |
| leave_time | bigint | 离开时间（Unix 时间戳） |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |

## 🔧 配置说明

编辑 `.env` 文件配置以下项：

```bash
# 服务配置
PORT=8080
ENV=development
LOG_LEVEL=info

# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=tgo_call

# Redis 配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# LiveKit 配置
LIVEKIT_URL=http://localhost:7880
LIVEKIT_API_KEY=your_api_key
LIVEKIT_API_SECRET=your_api_secret

# Webhook 配置
WEBHOOK_ENABLED=false
WEBHOOK_SECRET=your_webhook_secret
```

## 🚀 部署

### Docker 部署

```bash
# 构建镜像
docker build -t tgo-call-server:latest .

# 运行容器
docker run -p 8080:8080 --env-file .env tgo-call-server:latest
```

### 生产部署

详见 [部署架构指南.md](部署架构指南.md)

## 📖 开发指南

### 添加新的 API 接口

1. 定义数据模型 (`internal/models/`)
2. 实现业务逻辑 (`internal/service/`)
3. 创建 HTTP 处理器 (`internal/handler/`)
4. 配置路由 (`internal/router/`)

详见 [开发指南.md](开发指南.md)

## ❓ 常见问题

### 数据库连接失败

检查 MySQL 是否运行，以及 `.env` 中的数据库配置是否正确。

### Redis 连接失败

检查 Redis 是否运行，以及 `.env` 中的 Redis 配置是否正确。

### LiveKit Token 生成失败

检查 `.env` 中的 `LIVEKIT_API_KEY` 和 `LIVEKIT_API_SECRET` 是否正确。

详见 [部署常见问题.md](部署常见问题.md)

## 📞 获取帮助

- 查看 [快速开始.md](快速开始.md)
- 查看 [API_文档.md](API_文档.md)
- 查看 [开发指南.md](开发指南.md)
- 查看 [部署常见问题.md](部署常见问题.md)

## 📄 许可证

MIT License

## 👥 贡献

欢迎提交 Issue 和 Pull Request！

---

**项目状态**: ✅ 完成  
**最后更新**: 2024-01-15  
**Go 版本**: 1.21+

祝你使用愉快！🎉

