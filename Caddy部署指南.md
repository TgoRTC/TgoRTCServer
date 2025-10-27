# Caddy 反向代理部署指南

## 问题：Caddy 需要单独部署吗？

**答案：是的，需要单独部署和配置。**

---

## Caddy 的作用

Caddy 是反向代理和负载均衡器，负责：
- ✅ HTTPS 证书自动申请和续期（Let's Encrypt）
- ✅ 负载均衡（将请求分配到多个 LiveKit 节点）
- ✅ 故障转移（节点故障时自动转移）
- ✅ 请求转发

---

## 部署架构

```
┌─────────────────────────────────────────────┐
│         前端应用                             │
│    (Web Browser / Mobile App)               │
└────────────────┬────────────────────────────┘
                 │
                 │ 所有请求
                 │
    ┌────────────▼──────────────┐
    │   Caddy 反向代理           │
    │ https://livekit.example.com│
    │   (负载均衡 + HTTPS)       │
    │   (可以在任何机器上)       │
    └────────────┬──────────────┘
                 │
        ┌────────┼────────┐
        │        │        │
    ┌───▼──┐ ┌──▼───┐ ┌──▼───┐
    │Node1 │ │Node2 │ │Node3 │
    │192... │ │192... │ │192... │
    │:7880 │ │:7880 │ │:7880 │
    └──────┘ └──────┘ └──────┘
```

---

## 部署选项

### 选项 1：Caddy 与 LiveKit 在同一台机器上（推荐用于单机）

```
机器 1:
  ├─ Caddy (端口 80, 443)
  └─ LiveKit (端口 7880)
```

**配置：**
```bash
NODES=localhost
DOMAIN=livekit.example.com
```

---

### 选项 2：Caddy 与 LiveKit 在不同机器上（推荐用于分布式）

```
机器 0 (Caddy 专用):
  └─ Caddy (端口 80, 443)

机器 1, 2, 3 (LiveKit 节点):
  ├─ LiveKit Node 1 (端口 7880)
  ├─ LiveKit Node 2 (端口 7880)
  └─ LiveKit Node 3 (端口 7880)
```

**配置：**
```bash
# 在机器 1, 2, 3 上
NODES=192.168.1.10,192.168.1.11,192.168.1.12
DOMAIN=livekit.example.com

# 在 Caddy 机器上
CADDY_UPSTREAM=192.168.1.10:7880,192.168.1.11:7880,192.168.1.12:7880
```

---

### 选项 3：Caddy 与 LiveKit 在同一台机器上（分布式）

```
机器 1:
  ├─ Caddy (端口 80, 443)
  └─ LiveKit Node 1 (端口 7880)

机器 2:
  └─ LiveKit Node 2 (端口 7880)

机器 3:
  └─ LiveKit Node 3 (端口 7880)
```

**配置：**
```bash
# 所有机器上
NODES=192.168.1.10,192.168.1.11,192.168.1.12
DOMAIN=livekit.example.com
```

---

## 部署步骤

### 单机部署（Caddy + LiveKit 在同一台机器）

```bash
# 1. 准备
chmod +x *.sh
cp .env.example .env

# 2. 配置
nano .env
# 设置：
# NODES=localhost
# DOMAIN=livekit.example.com

# 3. 部署（自动部署 Caddy 和 LiveKit）
./deploy.sh deploy

# 4. 验证
./deploy.sh verify
```

---

### 分布式部署（Caddy 与 LiveKit 分离）

#### 第 1 步：在 Caddy 机器上部署

```bash
# 在 Caddy 专用机器上（例如 192.168.1.100）

chmod +x *.sh
cp .env.example .env

# 编辑 .env
nano .env
# 设置：
# DOMAIN=livekit.example.com
# NODES=192.168.1.10,192.168.1.11,192.168.1.12

# 只部署 Caddy（不部署 LiveKit）
./deploy.sh deploy-caddy-only
```

#### 第 2 步：在 LiveKit 机器上部署

```bash
# 在机器 1, 2, 3 上都执行

chmod +x *.sh
cp .env.example .env

# 编辑 .env
nano .env
# 设置：
# DOMAIN=livekit.example.com
# NODES=192.168.1.10,192.168.1.11,192.168.1.12
# REDIS_HOST=192.168.1.12
# REDIS_PASSWORD=your_password

# 只部署 LiveKit（不部署 Caddy）
./deploy.sh deploy-livekit-only
```

---

## Caddy 配置详解

### 自动生成的 Caddyfile

```
livekit.example.com {
    reverse_proxy localhost:7880 {
        header_up X-Forwarded-For {http.request.remote}
        header_up X-Forwarded-Proto {http.request.proto}
    }
}
```

### 分布式部署的 Caddyfile

```
livekit.example.com {
    reverse_proxy 192.168.1.10:7880 192.168.1.11:7880 192.168.1.12:7880 {
        header_up X-Forwarded-For {http.request.remote}
        header_up X-Forwarded-Proto {http.request.proto}
        policy random_choose
    }
}
```

**说明：**
- `reverse_proxy` - 反向代理到多个后端
- `header_up` - 添加请求头
- `policy random_choose` - 随机选择后端（负载均衡）

---

## DNS 配置

### 必须配置 DNS

```bash
# 在 DNS 服务商中配置
livekit.example.com  A  192.168.1.100  # Caddy 机器的 IP
```

### 验证 DNS

```bash
nslookup livekit.example.com
# 应该返回 Caddy 机器的 IP
```

---

## HTTPS 证书

### 自动申请（推荐）

Caddy 会自动从 Let's Encrypt 申请证书：

```bash
# 查看证书
docker-compose exec caddy caddy list-certs

# 查看证书过期时间
docker-compose exec caddy caddy trust
```

### 手动配置

如果需要使用自己的证书：

```
livekit.example.com {
    tls /path/to/cert.pem /path/to/key.pem
    reverse_proxy localhost:7880
}
```

---

## 常见问题

### Q1: Caddy 无法申请证书

**原因：** DNS 未正确配置或端口 80/443 未开放

**解决：**
```bash
# 检查 DNS
nslookup livekit.example.com

# 检查端口
netstat -tlnp | grep -E ':(80|443)'

# 检查防火墙
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

### Q2: 如何修改 Caddy 配置？

**方法：**
```bash
# 编辑配置文件
nano livekit-deployment/config/Caddyfile

# 重启 Caddy
docker-compose restart caddy
```

### Q3: 如何查看 Caddy 日志？

**方法：**
```bash
./deploy.sh logs caddy
```

---

## 部署检查清单

- [ ] DNS 已配置，指向 Caddy 机器
- [ ] 端口 80 和 443 已开放
- [ ] Caddy 容器已启动
- [ ] HTTPS 证书已申请
- [ ] LiveKit 节点已启动
- [ ] Redis 已启动
- [ ] 前端可以访问 https://livekit.example.com
- [ ] 创建房间接口返回 Caddy 地址

---

## 快速参考

### 单机部署
```bash
NODES=localhost
DOMAIN=livekit.example.com
./deploy.sh deploy
```

### 分布式部署
```bash
# Caddy 机器
NODES=192.168.1.10,192.168.1.11,192.168.1.12
DOMAIN=livekit.example.com
./deploy.sh deploy-caddy-only

# LiveKit 机器
NODES=192.168.1.10,192.168.1.11,192.168.1.12
DOMAIN=livekit.example.com
REDIS_HOST=192.168.1.12
./deploy.sh deploy-livekit-only
```

---

## 总结

| 项目 | 说明 |
|------|------|
| **Caddy 作用** | 反向代理、负载均衡、HTTPS |
| **部署位置** | 可以与 LiveKit 在同一机器或不同机器 |
| **DNS 配置** | 必须指向 Caddy 机器 |
| **证书** | 自动从 Let's Encrypt 申请 |
| **负载均衡** | 自动分配到多个 LiveKit 节点 |

