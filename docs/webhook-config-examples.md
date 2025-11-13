# Business Webhook 配置说明

## 概述

TgoRTC 服务器支持向业务系统发送 webhook 事件通知。从 v2.0 开始，支持为每个 webhook URL 配置独立的签名密钥。

---

## 配置方式

### 方式 1：新方式（推荐）- 支持多密钥

使用 `BUSINESS_WEBHOOK_ENDPOINTS` 环境变量，配置 JSON 格式的端点列表。

**优点：**
- ✅ 支持每个 URL 配置独立的签名密钥
- ✅ 适合发送到多个不同的第三方服务
- ✅ 更灵活、更安全

**配置示例：**

```bash
# 单个端点
BUSINESS_WEBHOOK_ENDPOINTS='[{"url":"http://service-a.com/webhook","secret":"secret-key-a"}]'

# 多个端点（不同密钥）
BUSINESS_WEBHOOK_ENDPOINTS='[
  {"url":"http://service-a.com/webhook","secret":"secret-key-a"},
  {"url":"http://service-b.com/webhook","secret":"secret-key-b"},
  {"url":"http://service-c.com/webhook","secret":"secret-key-c"}
]'

# 多个端点（不同密钥 + 不同超时）
BUSINESS_WEBHOOK_ENDPOINTS='[
  {"url":"http://fast-service.com/webhook","secret":"secret-a","timeout":5},
  {"url":"http://slow-service.com/webhook","secret":"secret-b","timeout":30}
]'
```

**JSON 格式说明：**

```json
[
  {
    "url": "http://service-a.com/webhook",    // Webhook URL
    "secret": "secret-key-a",                  // 该 URL 对应的签名密钥
    "timeout": 10                              // 该端点的超时时间（秒），可选，默认使用 BUSINESS_WEBHOOK_TIMEOUT
  },
  {
    "url": "http://service-b.com/webhook",
    "secret": "secret-key-b",
    "timeout": 30                              // 可以为不同端点配置不同的超时时间
  }
]
```

---

### 方式 2：旧方式（向后兼容）- 共享密钥

使用 `BUSINESS_WEBHOOK_URLS` 和 `BUSINESS_WEBHOOK_SECRET` 环境变量。

**优点：**
- ✅ 配置简单
- ✅ 适合同一业务系统的多个实例（负载均衡）

**缺点：**
- ❌ 所有 URL 使用相同的签名密钥
- ❌ 不适合发送到不同的第三方服务

**配置示例：**

```bash
# 单个 URL
BUSINESS_WEBHOOK_URLS=http://example.com/webhook
BUSINESS_WEBHOOK_SECRET=my-shared-secret-key

# 多个 URL（用逗号分隔，所有 URL 使用相同的密钥）
BUSINESS_WEBHOOK_URLS=http://server1.example.com/webhook,http://server2.example.com/webhook,http://server3.example.com/webhook
BUSINESS_WEBHOOK_SECRET=my-shared-secret-key
```

---

## 其他配置项

```bash
# Webhook 请求超时时间（秒），默认 10 秒
# 注意：
# - 使用新方式（BUSINESS_WEBHOOK_ENDPOINTS）时，此配置作为默认值
# - 如果端点配置中指定了 timeout，则使用端点的 timeout
# - 如果端点配置中未指定 timeout，则使用此全局默认值
BUSINESS_WEBHOOK_TIMEOUT=10

# Webhook 日志保留天数，默认 7 天
BUSINESS_WEBHOOK_LOG_RETENTION_DAYS=7

# 是否启用日志自动清理，默认 false
BUSINESS_WEBHOOK_LOG_CLEANUP_ENABLED=true

# 日志清理间隔（秒），默认 86400（1 天）
BUSINESS_WEBHOOK_LOG_CLEANUP_INTERVAL=86400
```

---

## 完整配置示例

### 示例 1：使用新方式（多密钥）

```bash
# .env 文件
PORT=8080
ENV=production

# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=tgo_rtc

# Redis 配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# LiveKit 配置
LIVEKIT_URL=http://localhost:7880
LIVEKIT_API_KEY=your_api_key
LIVEKIT_API_SECRET=your_api_secret

# Business Webhook 配置（新方式）
BUSINESS_WEBHOOK_ENDPOINTS='[
  {"url":"http://service-a.com/webhook","secret":"secret-a"},
  {"url":"http://service-b.com/webhook","secret":"secret-b"}
]'
BUSINESS_WEBHOOK_TIMEOUT=10
BUSINESS_WEBHOOK_LOG_RETENTION_DAYS=7
BUSINESS_WEBHOOK_LOG_CLEANUP_ENABLED=true
```

### 示例 2：使用旧方式（共享密钥）

```bash
# .env 文件
PORT=8080
ENV=production

# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=tgo_rtc

# Redis 配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# LiveKit 配置
LIVEKIT_URL=http://localhost:7880
LIVEKIT_API_KEY=your_api_key
LIVEKIT_API_SECRET=your_api_secret

# Business Webhook 配置（旧方式）
BUSINESS_WEBHOOK_URLS=http://server1.example.com/webhook,http://server2.example.com/webhook
BUSINESS_WEBHOOK_SECRET=my-shared-secret-key
BUSINESS_WEBHOOK_TIMEOUT=10
BUSINESS_WEBHOOK_LOG_RETENTION_DAYS=7
BUSINESS_WEBHOOK_LOG_CLEANUP_ENABLED=true
```

---

## 签名验证

### 签名计算方式

TgoRTC 服务器会为每个 webhook 请求计算 HMAC-SHA256 签名，并通过 `X-Signature` 请求头发送。

**签名计算公式：**

```
signature = HMAC-SHA256(payload, secret)
```

- `payload`: 请求体（JSON 格式）
- `secret`: 该端点配置的签名密钥
- `signature`: 十六进制编码的签名字符串

### 业务系统验证签名示例（Go）

```go
package main

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "io"
    "net/http"
)

func webhookHandler(w http.ResponseWriter, r *http.Request) {
    // 读取请求体
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    // 获取签名
    receivedSignature := r.Header.Get("X-Signature")

    // 计算期望的签名
    secret := "your-secret-key" // 从配置中获取
    h := hmac.New(sha256.New, []byte(secret))
    h.Write(body)
    expectedSignature := hex.EncodeToString(h.Sum(nil))

    // 验证签名
    if receivedSignature != expectedSignature {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // 签名验证通过，处理事件
    // ...

    w.WriteHeader(http.StatusOK)
}
```

### 业务系统验证签名示例（Node.js）

```javascript
const crypto = require('crypto');
const express = require('express');

const app = express();

app.post('/webhook', express.json(), (req, res) => {
    // 获取签名
    const receivedSignature = req.headers['x-signature'];

    // 计算期望的签名
    const secret = 'your-secret-key'; // 从配置中获取
    const payload = JSON.stringify(req.body);
    const expectedSignature = crypto
        .createHmac('sha256', secret)
        .update(payload)
        .digest('hex');

    // 验证签名
    if (receivedSignature !== expectedSignature) {
        return res.status(401).send('Invalid signature');
    }

    // 签名验证通过，处理事件
    // ...

    res.status(200).send('OK');
});

app.listen(3000);
```

---

## 迁移指南

### 从旧方式迁移到新方式

如果你当前使用的是旧方式（`BUSINESS_WEBHOOK_URLS` + `BUSINESS_WEBHOOK_SECRET`），可以按以下步骤迁移：

**步骤 1：准备新配置**

```bash
# 旧配置
BUSINESS_WEBHOOK_URLS=http://server1.example.com/webhook,http://server2.example.com/webhook
BUSINESS_WEBHOOK_SECRET=my-shared-secret-key

# 转换为新配置
BUSINESS_WEBHOOK_ENDPOINTS='[
  {"url":"http://server1.example.com/webhook","secret":"my-shared-secret-key"},
  {"url":"http://server2.example.com/webhook","secret":"my-shared-secret-key"}
]'
```

**步骤 2：更新环境变量**

1. 添加 `BUSINESS_WEBHOOK_ENDPOINTS` 环境变量
2. 删除 `BUSINESS_WEBHOOK_URLS` 和 `BUSINESS_WEBHOOK_SECRET` 环境变量（可选，保留也不影响）

**步骤 3：重启服务**

```bash
# 重启 TgoRTC 服务器
docker-compose restart
```

**步骤 4：验证**

检查日志，确认 webhook 事件正常发送。

---

## 常见问题

### Q1: 可以同时使用新旧两种配置方式吗？

**A:** 不建议。如果同时配置了 `BUSINESS_WEBHOOK_ENDPOINTS` 和 `BUSINESS_WEBHOOK_URLS`，系统会优先使用 `BUSINESS_WEBHOOK_ENDPOINTS`，忽略 `BUSINESS_WEBHOOK_URLS`。

### Q2: 如果某个端点的密钥为空会怎样？

**A:** 签名会使用空字符串作为密钥进行计算。建议为每个端点配置非空的密钥以确保安全性。

### Q3: 如何为不同的端点配置不同的超时时间？

**A:** 使用新方式（`BUSINESS_WEBHOOK_ENDPOINTS`）时，可以为每个端点配置独立的超时时间：

```bash
BUSINESS_WEBHOOK_ENDPOINTS='[
  {"url":"http://fast-service.com/webhook","secret":"secret-a","timeout":5},
  {"url":"http://slow-service.com/webhook","secret":"secret-b","timeout":30}
]'
```

如果端点配置中未指定 `timeout`，则使用全局默认值 `BUSINESS_WEBHOOK_TIMEOUT`。

### Q4: 签名验证失败怎么办？

**A:** 请检查：
1. 密钥配置是否正确
2. 请求体是否被修改（必须使用原始请求体计算签名）
3. 签名算法是否正确（HMAC-SHA256）
4. 编码方式是否正确（十六进制小写）

---

## 相关文档

- [Webhook 事件类型说明](./webhook-events.md)
- [API 文档](./swagger.json)
- [部署文档](./deployment.md)

