# Webhook 配置指南

## 什么是 Webhook？

Webhook 是 LiveKit 在发生特定事件时，向你的服务器发送 HTTP POST 请求的机制。

### 支持的事件

| 事件 | 说明 |
|------|------|
| `room_started` | 房间创建 |
| `room_finished` | 房间销毁 |
| `participant_joined` | 参与者加入 |
| `participant_left` | 参与者离开 |
| `track_published` | 发布媒体轨道 |
| `track_unpublished` | 取消发布媒体轨道 |
| `recording_finished` | 录制完成 |

---

## 配置步骤

### 第 1 步：编辑 .env 文件

```bash
nano .env
```

添加以下配置：

```bash
# 启用 Webhook
WEBHOOK_ENABLED=true

# Webhook 接收地址（必须是公网可访问的地址）
WEBHOOK_URLS=https://livekit.example.com/api/webhook

# Webhook API Key（用于签名验证）
WEBHOOK_API_KEY=your_webhook_api_key
```

### 第 2 步：创建后端 Webhook 处理服务

参考 `Webhook处理示例.go` 创建你的 Webhook 处理服务。

关键点：
- ✅ 验证 Webhook 签名
- ✅ 使用 Redis 去重
- ✅ 快速响应

### 第 3 步：部署 LiveKit

```bash
# 单机部署
./部署.sh deploy

# 或分布式部署
./部署.sh deploy-livekit-only
```

### 第 4 步：验证 Webhook 配置

```bash
# 查看 LiveKit 日志
./监控.sh logs livekit

# 查看 Webhook 配置
docker-compose exec livekit cat /etc/livekit.yaml | grep -A 5 webhook
```

---

## 分布式部署中的 Webhook

### 问题

在分布式部署中，每台 LiveKit 服务器都会发送 Webhook 事件。如果所有机器都配置相同的 Webhook URL，同一个事件会被发送多次。

### 解决方案：Redis 去重

所有机器都配置相同的 Webhook URL，但在后端使用 Redis 去重：

```go
// 生成事件 ID
eventID := fmt.Sprintf("webhook:%d:%s", event.CreatedAt, event.Event)

// 检查是否已处理过
exists, _ := redisClient.Exists(ctx, eventID).Result()
if exists > 0 {
    // 已处理过，直接返回
    return
}

// 处理事件
handleEvent(event)

// 标记为已处理（1 小时过期）
redisClient.Set(ctx, eventID, "1", time.Hour)
```

### 配置示例

```yaml
# 所有 LiveKit 机器上的 livekit.yaml
webhook:
  api_key: your_webhook_api_key
  urls:
    - https://livekit.example.com/api/webhook
```

---

## Webhook 请求格式

### 请求头

```
POST /api/webhook HTTP/1.1
Host: livekit.example.com
Content-Type: application/json
Authorization: <JWT Token>
```

### 请求体示例

```json
{
  "event": "room_started",
  "creation_time": 1234567890,
  "room": {
    "sid": "RM_xxx",
    "name": "my-room",
    "emptyTimeout": 300,
    "maxParticipants": 100,
    "creationTime": 1234567890
  }
}
```

---

## 签名验证

Webhook 请求使用 JWT 签名。验证步骤：

```go
import "github.com/livekit/protocol/webhook"

// 验证签名
event, err := webhook.ParseEvent(
    apiSecret,           // LIVEKIT_API_SECRET
    authHeader,          // Authorization 头
    body,                // 请求体
)
if err != nil {
    // 签名验证失败
    return
}
```

---

## 常见问题

### Q1: Webhook 没有被触发

**检查清单：**
- [ ] `WEBHOOK_ENABLED=true`
- [ ] `WEBHOOK_URLS` 配置正确
- [ ] Webhook 接收服务器可以公网访问
- [ ] 防火墙允许 HTTPS 连接
- [ ] LiveKit 已重启

### Q2: 同一个事件被处理多次

**原因：** 分布式部署中多台机器都发送了相同的事件

**解决：** 使用 Redis 去重（参考 `Webhook处理示例.go`）

### Q3: 如何测试 Webhook？

```bash
# 1. 启动 Webhook 服务
go run Webhook处理示例.go

# 2. 创建房间（会触发 room_started 事件）
curl -X POST http://localhost:8080/api/create-room \
  -H "Content-Type: application/json" \
  -d '{"room_name": "test", "user_name": "user1"}'

# 3. 查看日志
tail -f livekit-deployment/deploy.log
```

### Q4: 如何禁用 Webhook？

```bash
# 编辑 .env
WEBHOOK_ENABLED=false

# 重启 LiveKit
./部署.sh restart
```

---

## 最佳实践

1. **快速响应** - Webhook 处理应该在 5 秒内完成
2. **异步处理** - 使用消息队列处理耗时操作
3. **错误处理** - 返回 200 OK，即使处理失败
4. **日志记录** - 记录所有 Webhook 事件便于调试
5. **去重处理** - 使用 Redis 或数据库去重
6. **签名验证** - 始终验证 Webhook 签名

---

## 完整示例

### 后端代码

参考 `Webhook处理示例.go`

### 前端代码

```javascript
// 创建房间
const response = await fetch('http://localhost:8080/api/create-room', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    room_name: 'my-room',
    user_name: 'john'
  })
});

const data = await response.json();

// 连接到 LiveKit
const room = await connect(data.url, data.token, {
  audio: true,
  video: true
});

// 当房间销毁时，Webhook 会被触发
// 后端会收到 room_finished 事件
```

---

## 部署检查清单

- [ ] Webhook 服务已启动
- [ ] `.env` 中配置了 `WEBHOOK_ENABLED=true`
- [ ] `WEBHOOK_URLS` 指向正确的地址
- [ ] `WEBHOOK_API_KEY` 已设置
- [ ] LiveKit 已重启
- [ ] 防火墙允许 HTTPS 连接
- [ ] DNS 解析正确
- [ ] Redis 已启动
- [ ] 测试 Webhook 事件已收到

---

## 总结

| 项目 | 说明 |
|------|------|
| **启用 Webhook** | 设置 `WEBHOOK_ENABLED=true` |
| **配置 URL** | 设置 `WEBHOOK_URLS` |
| **验证签名** | 使用 `webhook.ParseEvent()` |
| **去重处理** | 使用 Redis 存储已处理事件 ID |
| **分布式部署** | 所有机器配置相同 URL，后端去重 |
| **错误处理** | 快速响应，异步处理 |

