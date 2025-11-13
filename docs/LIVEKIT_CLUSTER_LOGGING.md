# LiveKit 集群日志说明

## 📋 概述

TgoRTC Server 现在会记录详细的 LiveKit 连接信息，帮助你了解客户端被分配到哪个 LiveKit 服务器。

---

## 🔍 日志类型

### 1. Token 生成日志

**位置**: `internal/livekit/token.go`

**触发时机**: 每次为用户生成 LiveKit Token 时

**日志示例**:
```json
{
  "level": "info",
  "msg": "LiveKit Token 生成成功",
  "room_id": "abc123def456",
  "uid": "user_001",
  "livekit_url": "http://47.117.96.203:80",
  "backend_url": "http://host.docker.internal:80",
  "timeout": 3600
}
```

**字段说明**:
- `room_id`: 房间 ID
- `uid`: 用户 ID
- `livekit_url`: 返回给前端的 LiveKit 连接地址（客户端将连接到这个地址）
- `backend_url`: 后端调用 LiveKit API 的地址（通过 Nginx 负载均衡）
- `timeout`: Token 超时时间（秒）

---

### 2. 创建房间日志

**位置**: `internal/handler/room_handler.go`

**触发时机**: 房间创建成功后

**日志示例**:
```json
{
  "level": "info",
  "msg": "房间创建成功",
  "room_id": "abc123def456",
  "creator": "user_001",
  "source_channel_id": "channel_123",
  "livekit_url": "http://47.117.96.203:80",
  "invited_uids": ["user_002", "user_003"],
  "status": 0,
  "language": "zh-CN"
}
```

**字段说明**:
- `room_id`: 房间 ID
- `creator`: 创建者 UID
- `source_channel_id`: 来源频道 ID
- `livekit_url`: LiveKit 连接地址
- `invited_uids`: 被邀请的用户列表
- `status`: 房间状态（0=未开始, 1=进行中, 2=已结束）
- `language`: 客户端语言

---

### 3. 加入房间日志

**位置**: `internal/handler/participant_handler.go`

**触发时机**: 参与者成功加入房间后

**日志示例**:
```json
{
  "level": "info",
  "msg": "参与者加入房间成功",
  "room_id": "abc123def456",
  "uid": "user_002",
  "creator": "user_001",
  "livekit_url": "http://47.117.96.203:80",
  "room_status": 1,
  "language": "zh-CN"
}
```

**字段说明**:
- `room_id`: 房间 ID
- `uid`: 加入者 UID
- `creator`: 房间创建者
- `livekit_url`: LiveKit 连接地址
- `room_status`: 房间状态
- `language`: 客户端语言

---

## 📊 查看日志的方法

### 方法 1: 实时查看日志（推荐）

```bash
# 查看所有日志
docker logs -f tgo-rtc-server

# 只查看 LiveKit 相关日志
docker logs -f tgo-rtc-server | grep -E "LiveKit|livekit_url"

# 只查看 Token 生成日志
docker logs -f tgo-rtc-server | grep "LiveKit Token 生成成功"

# 只查看房间创建日志
docker logs -f tgo-rtc-server | grep "房间创建成功"

# 只查看参与者加入日志
docker logs -f tgo-rtc-server | grep "参与者加入房间成功"
```

### 方法 2: 查看历史日志

```bash
# 查看最近 100 行日志
docker logs --tail 100 tgo-rtc-server

# 查看最近 1 小时的日志
docker logs --since 1h tgo-rtc-server

# 查看特定时间段的日志
docker logs --since "2025-11-12T10:00:00" --until "2025-11-12T12:00:00" tgo-rtc-server
```

### 方法 3: 搜索特定用户的日志

```bash
# 查看特定用户的所有操作
docker logs tgo-rtc-server | grep "user_001"

# 查看特定房间的所有操作
docker logs tgo-rtc-server | grep "abc123def456"
```

### 方法 4: 导出日志到文件

```bash
# 导出所有日志
docker logs tgo-rtc-server > /tmp/tgo-rtc-server.log

# 导出 LiveKit 相关日志
docker logs tgo-rtc-server | grep -E "LiveKit|livekit_url" > /tmp/livekit-connections.log
```

---

## 🎯 实际使用场景

### 场景 1: 查看客户端连接到哪个 LiveKit 服务器

当客户端创建或加入房间时，查看日志中的 `livekit_url` 字段：

```bash
docker logs -f tgo-rtc-server | grep "livekit_url"
```

输出示例：
```
"livekit_url": "http://47.117.96.203:80"
```

这表示客户端将连接到 `http://47.117.96.203:80`，然后 Nginx 会将请求负载均衡到：
- `127.0.0.1:7880` (本地 LiveKit)
- `39.103.125.196:7880` (远程 LiveKit)

### 场景 2: 验证负载均衡是否工作

1. 创建多个房间
2. 查看 Nginx 日志：
```bash
sudo tail -f /var/log/nginx/livekit-cluster-access.log
```

3. 查看 LiveKit 节点日志：
```bash
# 服务器 B 的 LiveKit
docker logs -f tgo-rtc-livekit | grep "participant_joined"

# 服务器 A 的 LiveKit
docker logs -f livekit-server | grep "participant_joined"
```

如果两个节点都有日志输出，说明负载均衡正常工作。

### 场景 3: 排查连接问题

如果客户端无法连接，检查以下日志：

```bash
# 1. 检查 Token 是否生成成功
docker logs tgo-rtc-server | grep "LiveKit Token 生成成功"

# 2. 检查返回的 URL 是否正确
docker logs tgo-rtc-server | grep "livekit_url"

# 3. 检查 Nginx 是否收到请求
sudo tail -f /var/log/nginx/livekit-cluster-access.log

# 4. 检查 LiveKit 是否收到连接
docker logs -f tgo-rtc-livekit | grep -E "participant_joined|room_started"
```

---

## 📈 日志分析示例

### 完整的房间创建流程日志

```json
// 1. Token 生成
{
  "level": "info",
  "msg": "LiveKit Token 生成成功",
  "room_id": "abc123",
  "uid": "user_001",
  "livekit_url": "http://47.117.96.203:80",
  "backend_url": "http://host.docker.internal:80",
  "timeout": 3600
}

// 2. 房间创建成功
{
  "level": "info",
  "msg": "房间创建成功",
  "room_id": "abc123",
  "creator": "user_001",
  "livekit_url": "http://47.117.96.203:80",
  "invited_uids": ["user_002"],
  "status": 0
}

// 3. 参与者加入
{
  "level": "info",
  "msg": "参与者加入房间成功",
  "room_id": "abc123",
  "uid": "user_002",
  "livekit_url": "http://47.117.96.203:80",
  "room_status": 1
}
```

---

## 🔧 日志配置

日志级别由环境变量 `LOG_LEVEL` 控制：

```bash
# .env 文件
LOG_LEVEL=info  # debug, info, warn, error
```

- `debug`: 显示所有日志（包括调试信息）
- `info`: 显示信息、警告和错误（推荐）
- `warn`: 只显示警告和错误
- `error`: 只显示错误

---

## 💡 提示

1. **生产环境建议**: 使用 `LOG_LEVEL=info`，既能看到关键信息，又不会产生过多日志
2. **调试时**: 使用 `LOG_LEVEL=debug` 查看更详细的信息
3. **日志轮转**: 建议配置 Docker 日志轮转，避免日志文件过大：

```yaml
# docker-compose.yml
services:
  tgo-rtc-server:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

4. **集中日志管理**: 生产环境建议使用 ELK、Loki 等日志收集系统

---

## 📝 总结

现在你可以通过日志清楚地看到：
- ✅ 每个用户被分配到哪个 LiveKit URL
- ✅ Token 生成的详细信息
- ✅ 房间创建和参与者加入的完整流程
- ✅ 负载均衡是否正常工作

这些日志将帮助你监控和调试 LiveKit 集群的运行状态。

