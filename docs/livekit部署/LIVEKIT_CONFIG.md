# LiveKit 配置指南

## 快速理解

### API Key 和 Secret 的区别

- **API Key**：可以是任意字符串，你自己起的名字（如 `devkey`、`prodkey`、`myapp`）
- **Secret**：必须是强密码，使用 `openssl rand -base64 32` 生成

### 快速示例

```bash
# 1. 生成一个随机 Secret
openssl rand -base64 32
# 输出：Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9

# 2. 在 livekit.yaml 中配置
# keys:
#   myapp: Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9
#   ↑      ↑
#   API Key Secret（上面生成的）

# 3. 在 .env 中使用相同的值
# LIVEKIT_API_KEY=myapp
# LIVEKIT_API_SECRET=Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9
```

---

## LiveKit API Key 和 Secret 详细说明

### 重要概念

LiveKit 的 API Key 和 Secret **不是从 LiveKit 官网获取的**，而是在部署 LiveKit Server 时**自己定义**的。

### 配置方式

#### 1. 在 LiveKit Server 中定义

编辑 `livekit.yaml` 配置文件：

```yaml
port: 7880

# 定义 API Key 和 Secret
keys:
  devkey: secret
  # 可以定义多个 key，用于不同的环境或应用
  # prodkey: production_secret_here
  # testkey: test_secret_here
```

**格式说明：**
- `keys:` 下面的每一行定义一个 API Key/Secret 对
- 冒号前面是 **API Key**（例如：`devkey`）
- 冒号后面是 **Secret**（例如：`secret`）

#### 2. 在 TgoRTC Server 中使用

编辑 `.env` 文件，配置与 LiveKit 相同的 Key 和 Secret：

```env
LIVEKIT_URL=http://livekit:7880
LIVEKIT_API_KEY=devkey
LIVEKIT_API_SECRET=secret
```

**对应关系：**

| livekit.yaml | .env 文件 |
|--------------|-----------|
| `devkey: secret` | `LIVEKIT_API_KEY=devkey`<br>`LIVEKIT_API_SECRET=secret` |
| `prodkey: prod123` | `LIVEKIT_API_KEY=prodkey`<br>`LIVEKIT_API_SECRET=prod123` |

### 生成安全的 Secret

**重要：** `openssl rand -base64 32` 生成的是 **Secret**（密钥），不是 API Key！

- **API Key**：可以是任意字符串，通常是有意义的名称（如 `devkey`、`prodkey`）
- **Secret**：必须是强密码，使用以下方法生成

#### 方法一：使用 OpenSSL（推荐）

```bash
# 生成 32 位随机字符串作为 Secret
openssl rand -base64 32

# 输出示例（这是 Secret，不是 API Key）：
# 8xK9mP2nQ5vR7wT1yU3zA6bC4dE8fG0h
```

**使用示例：**
```yaml
# livekit.yaml
keys:
  prodkey: 8xK9mP2nQ5vR7wT1yU3zA6bC4dE8fG0h
  # ↑       ↑
  # API Key  Secret（使用 openssl 生成的）
```

#### 方法二：使用 Python

```bash
# 生成随机 Secret
python3 -c "import secrets; print(secrets.token_urlsafe(32))"

# 输出示例（这是 Secret）：
# Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9
```

#### 方法三：在线生成

访问：https://www.random.org/strings/

**配置参数建议：**
- Length: 32
- Characters: Alphanumeric (a-z, A-Z, 0-9)

### 配置示例

#### 开发环境

**livekit.yaml：**
```yaml
port: 7880
rtc:
  port_range_start: 50000
  port_range_end: 50100
  use_external_ip: true

keys:
  devkey: secret
```

**.env：**
```env
LIVEKIT_URL=http://localhost:7880
LIVEKIT_API_KEY=devkey
LIVEKIT_API_SECRET=secret
```

#### 生产环境

**步骤 1：生成 Secret**
```bash
# 生成一个强密码作为 Secret
openssl rand -base64 32
# 输出示例：8xK9mP2nQ5vR7wT1yU3zA6bC4dE8fG0hXj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9
```

**步骤 2：配置 livekit.yaml**
```yaml
port: 7880
rtc:
  port_range_start: 50000
  port_range_end: 50100
  use_external_ip: true

keys:
  prodkey: 8xK9mP2nQ5vR7wT1yU3zA6bC4dE8fG0hXj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9
  # ↑       ↑
  # API Key  Secret（步骤 1 生成的）
  #
  # API Key 说明：
  # - 可以是任意字符串（如 prodkey、myapp、service1）
  # - 建议使用有意义的名称，方便管理
  # - 区分大小写

# 生产环境建议配置 Redis（用于集群）
redis:
  address: redis:6379
  password: your_redis_password
  db: 0
```

**步骤 3：配置 .env**
```env
LIVEKIT_URL=https://livekit.yourdomain.com
LIVEKIT_API_KEY=prodkey
LIVEKIT_API_SECRET=8xK9mP2nQ5vR7wT1yU3zA6bC4dE8fG0hXj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9
```

**说明：**
- `LIVEKIT_API_KEY=prodkey` - 这是你自己起的名字，可以是任意字符串
- `LIVEKIT_API_SECRET=8xK9...` - 这是用 `openssl rand -base64 32` 生成的强密码

### 多应用场景

如果你有多个应用需要连接到同一个 LiveKit Server，可以为每个应用分配不同的 API Key：

**livekit.yaml：**
```yaml
keys:
  app1_key: app1_secret_here
  app2_key: app2_secret_here
  app3_key: app3_secret_here
```

**应用 1 的 .env：**
```env
LIVEKIT_API_KEY=app1_key
LIVEKIT_API_SECRET=app1_secret_here
```

**应用 2 的 .env：**
```env
LIVEKIT_API_KEY=app2_key
LIVEKIT_API_SECRET=app2_secret_here
```

### 使用 LiveKit Cloud（官方托管服务）

如果你使用 LiveKit Cloud（https://cloud.livekit.io），则 API Key 和 Secret 是从官方控制台获取的：

1. 访问 https://cloud.livekit.io
2. 注册并登录
3. 创建项目
4. 在项目设置中找到 **API Key** 和 **API Secret**
5. 复制到 `.env` 文件中

**使用 LiveKit Cloud 的配置：**

```env
# 使用 LiveKit Cloud 提供的 URL
LIVEKIT_URL=wss://your-project.livekit.cloud

# 从 LiveKit Cloud 控制台获取
LIVEKIT_API_KEY=APIxxxxxxxxxx
LIVEKIT_API_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**注意：** 使用 LiveKit Cloud 时，不需要自己部署 LiveKit Server，也不需要 `livekit.yaml` 配置文件。

## 验证配置

### 1. 检查 LiveKit Server 是否启动

```bash
# 查看 LiveKit 容器日志
docker-compose logs livekit

# 应该看到类似输出：
# livekit-server version x.x.x
# starting LiveKit server on port 7880
```

### 2. 测试 API Key 是否有效

```bash
# 使用 curl 测试（需要安装 livekit-cli）
livekit-cli create-token \
  --api-key devkey \
  --api-secret secret \
  --room test-room \
  --identity test-user

# 如果配置正确，会输出一个 JWT token
```

### 3. 检查 TgoRTC Server 连接

```bash
# 查看 TgoRTC Server 日志
docker-compose logs tgo-rtc-server

# 应该看到：
# LiveKit connected successfully
# 或者没有 LiveKit 连接错误
```

### 4. 测试创建房间

```bash
curl -X POST http://localhost:8080/api/v1/rooms \
  -H 'Content-Type: application/json' \
  -d '{
    "source_channel_id": "test_ch_001",
    "source_channel_type": 0,
    "creator": "test_user_001",
    "rtc_type": 1,
    "invite_on": 1,
    "max_participants": 3,
    "uids": ["test_user_002"]
  }'

# 如果返回 room_id 和 token，说明配置正确
```

## 常见问题

### 1. API Key 不匹配

**错误：** `invalid API key`

**原因：** `.env` 中的 `LIVEKIT_API_KEY` 与 `livekit.yaml` 中的 key 不一致

**解决：** 确保两边配置完全一致（区分大小写）

### 2. Secret 不匹配

**错误：** `invalid signature` 或 `authentication failed`

**原因：** `.env` 中的 `LIVEKIT_API_SECRET` 与 `livekit.yaml` 中的 secret 不一致

**解决：** 确保两边配置完全一致

### 3. LiveKit 连接失败

**错误：** `connection refused` 或 `timeout`

**原因：** `LIVEKIT_URL` 配置错误或 LiveKit Server 未启动

**解决：**
```bash
# 检查 LiveKit 是否运行
docker-compose ps livekit

# 检查 URL 配置
# Docker Compose 内部通信使用服务名：http://livekit:7880
# 外部访问使用 IP 或域名：http://your-server-ip:7880
```

### 4. Token 生成失败

**错误：** `failed to generate token`

**原因：** API Key 或 Secret 配置错误

**解决：** 检查 `.env` 文件中的配置，确保与 `livekit.yaml` 一致

## 安全建议

### 开发环境

- ✅ 可以使用简单的 key 和 secret（如 `devkey: secret`）
- ✅ 方便调试和测试

### 生产环境

- ⚠️ **必须使用强密码**（至少 32 位随机字符串）
- ⚠️ **不要在代码中硬编码** API Key 和 Secret
- ⚠️ **使用环境变量**（.env 文件）
- ⚠️ **不要将 .env 文件提交到 Git**
- ⚠️ **定期更换** Secret
- ⚠️ **使用 HTTPS/WSS** 连接 LiveKit

### 最佳实践

1. **使用不同的 Key** - 开发、测试、生产环境使用不同的 API Key
2. **限制权限** - 如果 LiveKit 支持，为不同的 Key 配置不同的权限
3. **监控使用** - 记录 API Key 的使用情况，及时发现异常
4. **备份配置** - 保存 `livekit.yaml` 的备份，但要加密存储

## 总结

- ✅ LiveKit API Key 和 Secret 是在 `livekit.yaml` 中**自己定义**的
- ✅ TgoRTC Server 的 `.env` 文件中配置**相同的值**
- ✅ 开发环境可以使用简单的 `devkey: secret`
- ✅ 生产环境必须使用强密码（使用 `openssl rand -base64 32` 生成）
- ✅ 使用 LiveKit Cloud 时，从官方控制台获取 API Key 和 Secret

