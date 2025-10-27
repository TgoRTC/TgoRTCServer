# LiveKit URL 配置详解

## 问题：LiveKitConfig 中的 URL 是哪个地址？

### 简短答案

**`URL` 是 Caddy 反向代理的地址，而不是三个服务器中的任何一个具体地址。**

```
URL = https://livekit.example.com  (Caddy 地址)
```

---

## 详细解释

### 分布式部署架构

```
┌─────────────────────────────────────────────────────────┐
│                    前端应用                              │
│              (Web Browser / Mobile App)                 │
└────────────────────┬────────────────────────────────────┘
                     │
                     │ 所有请求都发送到这个地址
                     │ https://livekit.example.com
                     │
        ┌────────────▼──────────────┐
        │   Caddy 反向代理           │
        │ https://livekit.example.com│
        │   (负载均衡)               │
        └────────────┬──────────────┘
                     │
        ┌────────────┼────────────┐
        │            │            │
    ┌───▼──┐     ┌───▼──┐    ┌───▼──┐
    │Node1 │     │Node2 │    │Node3 │
    │192... │     │192... │    │192... │
    │:7880 │     │:7880 │    │:7880 │
    └──────┘     └──────┘    └──────┘
        │            │            │
        └────────────┼────────────┘
                     │
            ┌────────▼────────┐
            │  Redis 主节点    │
            │  192.168.1.12   │
            │  :6379          │
            └─────────────────┘
```

### 三个服务器的 IP 地址

假设你有三台机器：

| 机器 | IP 地址 | 用途 |
|------|---------|------|
| 机器 1 | 192.168.1.10 | LiveKit Node 1 |
| 机器 2 | 192.168.1.11 | LiveKit Node 2 |
| 机器 3 | 192.168.1.12 | LiveKit Node 3 + Redis |

### LiveKitConfig.URL 的值

```go
// ❌ 错误的做法 - 返回具体的某台机器
URL: "https://192.168.1.10"  // ❌ 不对
URL: "https://192.168.1.11"  // ❌ 不对
URL: "https://192.168.1.12"  // ❌ 不对

// ✅ 正确的做法 - 返回 Caddy 地址
URL: "https://livekit.example.com"  // ✅ 正确
```

---

## 为什么要返回 Caddy 地址？

### 1. 负载均衡

Caddy 会自动将请求分配到最优的节点：

```
请求 1 → Caddy → Node 1
请求 2 → Caddy → Node 2
请求 3 → Caddy → Node 3
请求 4 → Caddy → Node 1 (循环)
```

### 2. 故障转移

如果某个节点故障，Caddy 会自动转移到其他节点：

```
Node 1 故障 ✗
  ↓
Caddy 检测到故障
  ↓
自动转移到 Node 2 或 Node 3 ✓
```

### 3. 透明性

前端无需关心后端有多少个节点，只需连接到一个地址：

```
前端: 我要连接到 https://livekit.example.com
Caddy: 好的，我来帮你分配到最优的节点
```

### 4. 可扩展性

添加新节点时，前端代码无需修改：

```
原来: 3 个节点
  前端 → Caddy → [Node1, Node2, Node3]

添加后: 5 个节点
  前端 → Caddy → [Node1, Node2, Node3, Node4, Node5]
  
前端代码: 无需修改 ✓
```

---

## 配置示例

### 环境变量配置

```bash
# 后端服务器上的环境变量
export LIVEKIT_API_KEY=your_api_key
export LIVEKIT_API_SECRET=your_api_secret
export LIVEKIT_URL=https://livekit.example.com  # ✅ Caddy 地址
```

### Go 代码

```go
// 从环境变量读取
liveKitConfig := LiveKitConfig{
    APIKey:    os.Getenv("LIVEKIT_API_KEY"),
    APISecret: os.Getenv("LIVEKIT_API_SECRET"),
    URL:       os.Getenv("LIVEKIT_URL"),  // https://livekit.example.com
}

// 返回给前端
response := CreateRoomResponse{
    RoomName:  req.RoomName,
    Token:     token,
    URL:       liveKitConfig.URL,        // https://livekit.example.com
    ServerURL: liveKitConfig.URL,        // https://livekit.example.com
}
```

### 前端代码

```javascript
// 后端返回的响应
const data = {
    room_name: "my-room",
    token: "eyJhbGc...",
    url: "https://livekit.example.com",        // ✅ Caddy 地址
    server_url: "https://livekit.example.com"  // ✅ Caddy 地址
};

// 使用返回的 URL 连接
const room = new LivekitClient.Room();
await room.connect(data.url, data.token, {
    audio: true,
    video: { resolution: { width: 640, height: 480 } }
});
```

---

## 完整的部署配置

### 机器 1、2、3 的 .env 配置

```bash
# 所有机器上都相同
DOMAIN=livekit.example.com
NODES=192.168.1.10,192.168.1.11,192.168.1.12

# Redis 指向主节点（机器 3）
REDIS_HOST=192.168.1.12
REDIS_PORT=6379
REDIS_PASSWORD=your_secure_password

# API 密钥（所有机器相同）
LIVEKIT_API_KEY=your_api_key
LIVEKIT_API_SECRET=your_api_secret
```

### 后端服务器的环境变量

```bash
# 后端服务器（可以在任何地方，不一定在 LiveKit 机器上）
export LIVEKIT_API_KEY=your_api_key
export LIVEKIT_API_SECRET=your_api_secret
export LIVEKIT_URL=https://livekit.example.com  # ✅ Caddy 地址
export PORT=8080
```

### Caddy 配置

```
livekit.example.com {
    reverse_proxy localhost:7880 {
        header_up X-Forwarded-For {http.request.remote}
        header_up X-Forwarded-Proto {http.request.proto}
    }
}
```

---

## 常见错误

### ❌ 错误 1: 返回具体的机器 IP

```go
// 错误的做法
response := CreateRoomResponse{
    URL: "https://192.168.1.10",  // ❌ 错误
}
```

**问题：**
- 如果这台机器故障，用户无法连接
- 无法进行负载均衡
- 添加新节点时需要修改代码

### ❌ 错误 2: 返回 localhost

```go
// 错误的做法
response := CreateRoomResponse{
    URL: "https://localhost:7880",  // ❌ 错误
}
```

**问题：**
- 前端无法访问（localhost 是本地地址）
- 跨域问题
- 无法从其他机器访问

### ❌ 错误 3: 返回 LiveKit 服务器的 IP

```go
// 错误的做法
response := CreateRoomResponse{
    URL: "https://192.168.1.10:7880",  // ❌ 错误
}
```

**问题：**
- 绕过了 Caddy 的负载均衡
- 绕过了 Caddy 的 HTTPS 证书
- 无法进行故障转移

### ✅ 正确的做法

```go
// 正确的做法
response := CreateRoomResponse{
    URL: "https://livekit.example.com",  // ✅ 正确
}
```

---

## 总结

| 项目 | 值 |
|------|-----|
| **LiveKitConfig.URL** | `https://livekit.example.com` |
| **是否是具体的机器 IP？** | ❌ 不是 |
| **是否是 Caddy 地址？** | ✅ 是 |
| **是否支持负载均衡？** | ✅ 是 |
| **是否支持故障转移？** | ✅ 是 |
| **是否支持扩展？** | ✅ 是 |

---

## 快速参考

### 单机部署

```bash
# .env
NODES=localhost
DOMAIN=livekit.example.com

# 后端环境变量
export LIVEKIT_URL=https://livekit.example.com
```

### 分布式部署

```bash
# 所有机器的 .env
NODES=192.168.1.10,192.168.1.11,192.168.1.12
DOMAIN=livekit.example.com

# 后端环境变量（无论后端在哪里）
export LIVEKIT_URL=https://livekit.example.com  # ✅ 总是 Caddy 地址
```

---

## 实际例子

### 场景：用户创建房间

```
1. 用户在前端输入房间名和用户名
   ↓
2. 前端调用后端 API: POST /api/create-room
   ↓
3. 后端生成 Token，返回响应：
   {
     "room_name": "my-room",
     "token": "eyJhbGc...",
     "url": "https://livekit.example.com",  ← ✅ Caddy 地址
     "server_url": "https://livekit.example.com"
   }
   ↓
4. 前端使用返回的 URL 和 Token 连接
   ↓
5. Caddy 接收连接请求
   ↓
6. Caddy 将请求转发到最优的节点（Node1、Node2 或 Node3）
   ↓
7. 用户成功连接到 LiveKit
```

---

## 总结

**`LiveKitConfig.URL` 是 Caddy 反向代理的地址，而不是三个服务器中的任何一个。**

这样做的好处：
- ✅ 自动负载均衡
- ✅ 自动故障转移
- ✅ 前端代码无需修改
- ✅ 支持灵活扩展

