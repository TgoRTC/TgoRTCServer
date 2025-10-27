# 音视频服务 API 文档

## 项目概述

这是一个基于 LiveKit 的音视频服务接口业务服务，提供房间管理、参与者管理等核心功能。

## 技术栈

- **框架**: Gin Web Framework
- **数据库**: MySQL + GORM
- **缓存**: Redis
- **音视频**: LiveKit
- **认证**: JWT Token

## 项目结构

```
tgo-call-server/
├── main.go                          # 主程序入口
├── go.mod                           # Go 模块定义
├── .env.example                     # 环境变量示例
├── internal/
│   ├── config/
│   │   └── config.go               # 配置管理
│   ├── database/
│   │   ├── db.go                   # 数据库初始化
│   │   └── redis.go                # Redis 初始化
│   ├── models/
│   │   ├── room.go                 # 房间模型
│   │   └── participant.go          # 参与者模型
│   ├── service/
│   │   ├── room_service.go         # 房间业务逻辑
│   │   └── participant_service.go  # 参与者业务逻辑
│   ├── handler/
│   │   ├── room_handler.go         # 房间 API 处理器
│   │   └── participant_handler.go  # 参与者 API 处理器
│   ├── livekit/
│   │   └── token.go                # LiveKit Token 生成
│   └── router/
│       └── router.go               # 路由配置
└── 数据库字段说明.md                # 数据库字段文档
```

## 快速开始

### 1. 环境准备

```bash
# 复制环境变量文件
cp .env.example .env

# 编辑 .env 文件，配置数据库和 LiveKit 信息
nano .env
```

### 2. 安装依赖

```bash
go mod download
```

### 3. 启动服务

```bash
go run main.go
```

服务将在 `http://localhost:8080` 启动。

## API 接口

### 健康检查

#### GET /health

检查服务健康状态。

**响应示例**:
```json
{
  "status": "ok"
}
```

---

## 房间管理 API

### 创建房间

#### POST /api/rooms

创建一个新的音视频房间。

**请求体**:
```json
{
  "source_channel_id": "channel_123",
  "source_channel_type": 0,
  "creator": "user_001",
  "room_name": "meeting_001",
  "call_type": 1,
  "invite_on": 1
}
```

**响应示例**:
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

### 获取房间信息

#### GET /api/rooms/:room_name

获取指定房间的详细信息。

**响应示例**:
```json
{
  "code": 0,
  "msg": "获取房间信息成功",
  "data": {
    "id": 1,
    "source_channel_id": "channel_123",
    "source_channel_type": 0,
    "creator": "user_001",
    "room_name": "meeting_001",
    "call_type": 1,
    "invite_on": 1,
    "status": 0,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
}
```

### 列出房间列表

#### GET /api/rooms?limit=10&offset=0

获取房间列表。

**查询参数**:
- `limit`: 每页数量（默认 10）
- `offset`: 偏移量（默认 0）

**响应示例**:
```json
{
  "code": 0,
  "msg": "获取房间列表成功",
  "data": {
    "rooms": [...],
    "total": 100
  }
}
```

### 更新房间状态

#### PUT /api/rooms/:room_name/status

更新房间状态。

**请求体**:
```json
{
  "status": 1
}
```

**状态值**:
- `0`: 未开始
- `1`: 进行中
- `2`: 已结束
- `3`: 已取消

### 结束房间

#### POST /api/rooms/:room_name/end

结束指定房间。

---

## 参与者管理 API

### 加入房间

#### POST /api/participants/join

参与者加入房间。

**请求体**:
```json
{
  "room_name": "meeting_001",
  "uid": "user_002"
}
```

**响应示例**:
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

### 离开房间

#### POST /api/participants/leave

参与者离开房间。

**请求体**:
```json
{
  "room_name": "meeting_001",
  "uid": "user_002"
}
```

### 获取参与者列表

#### GET /api/rooms/:room_name/participants

获取房间内的参与者列表。

**响应示例**:
```json
{
  "code": 0,
  "msg": "获取参与者列表成功",
  "data": [
    {
      "id": 1,
      "room_name": "meeting_001",
      "uid": "user_002",
      "status": 1,
      "join_time": 1705315800000,
      "leave_time": 0,
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

### 邀请参与者

#### POST /api/rooms/:room_name/invite

邀请参与者加入房间。

**请求体**:
```json
{
  "uids": ["user_002", "user_003"]
}
```

### 更新参与者状态

#### PUT /api/rooms/:room_name/participants/:uid/status

更新参与者状态。

**请求体**:
```json
{
  "status": 1
}
```

**状态值**:
- `0`: 邀请中
- `1`: 已加入
- `2`: 已拒绝
- `3`: 已挂断
- `4`: 超时未加入
- `5`: 通话中未接听
- `6`: 已取消

---

## 数据库表结构

详见 [数据库字段说明.md](数据库字段说明.md)

## 部署指南

详见 [部署架构指南.md](部署架构指南.md)

## 常见问题

详见 [部署常见问题.md](部署常见问题.md)

