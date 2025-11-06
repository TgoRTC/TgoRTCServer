# Nginx 反向代理配置指南

本文档介绍如何为 TgoRTC Server 配置 Nginx 反向代理，实现域名访问和 HTTPS 加密。

---

## 📋 目录

- [为什么需要 Nginx](#为什么需要-nginx)
- [环境要求](#环境要求)
- [配置方案](#配置方案)
- [安装 Nginx](#安装-nginx)
- [配置 HTTP 反向代理](#配置-http-反向代理)
- [配置 HTTPS（Let's Encrypt）](#配置-httpsletsencrypt)
- [WebSocket 支持](#websocket-支持)
- [负载均衡配置](#负载均衡配置)
- [常见问题](#常见问题)

---

## 为什么需要 Nginx

### 优势

✅ **域名访问** - 使用域名代替 IP:端口  
✅ **HTTPS 加密** - 保护数据传输安全  
✅ **负载均衡** - 支持多实例部署  
✅ **静态资源缓存** - 提升性能  
✅ **访问控制** - IP 白名单、限流等  
✅ **日志记录** - 详细的访问日志  

### 架构对比

**部署前：**
```
客户端 → http://47.117.96.203:8080/api/v1/rooms
```

**部署后：**
```
客户端 → https://api.yourdomain.com/api/v1/rooms
         ↓
      Nginx (443)
         ↓
   TgoRTC Server (8080)
```

---

## 环境要求

- 已部署 TgoRTC Server（参考 [PRODUCTION_DEPLOYMENT_GUIDE.md](./PRODUCTION_DEPLOYMENT_GUIDE.md)）
- 域名（已解析到服务器 IP）
- 服务器开放 80 和 443 端口

---

## 配置方案

### 方案 1：单域名配置（推荐）

所有服务使用同一个域名，通过路径区分：

```
https://yourdomain.com/api/v1/*        → TgoRTC API
https://yourdomain.com/swagger/*       → Swagger 文档
https://yourdomain.com/livekit/*       → LiveKit 服务
```

### 方案 2：多域名配置

不同服务使用不同子域名：

```
https://api.yourdomain.com/*           → TgoRTC API
https://livekit.yourdomain.com/*       → LiveKit 服务
```

本文档以**方案 2（多域名）**为例。

---

## 安装 Nginx

### CentOS/RHEL

```bash
# 安装 Nginx
yum install -y nginx

# 启动 Nginx
systemctl start nginx
systemctl enable nginx

# 验证安装
nginx -v
```

### Ubuntu/Debian

```bash
# 更新包列表
apt update

# 安装 Nginx
apt install -y nginx

# 启动 Nginx
systemctl start nginx
systemctl enable nginx

# 验证安装
nginx -v
```

### 验证 Nginx 运行

```bash
# 检查 Nginx 状态
systemctl status nginx

# 浏览器访问
curl http://localhost
# 应该看到 Nginx 欢迎页面
```

---

## 配置 HTTP 反向代理

### 步骤 1：创建 TgoRTC API 配置文件

```bash
vim /etc/nginx/conf.d/tgortc-api.conf
```

**内容：**

```nginx
# TgoRTC API 服务配置
upstream tgortc_backend {
    server 127.0.0.1:8080;
    # 如果有多个实例，可以添加更多服务器
    # server 127.0.0.1:8081;
    # server 127.0.0.1:8082;
}

server {
    listen 80;
    server_name api.yourdomain.com;  # 替换为你的域名

    # 访问日志
    access_log /var/log/nginx/tgortc-api-access.log;
    error_log /var/log/nginx/tgortc-api-error.log;

    # 客户端最大上传大小
    client_max_body_size 10M;

    # API 接口
    location / {
        proxy_pass http://tgortc_backend;
        proxy_http_version 1.1;
        
        # 传递真实客户端信息
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 超时设置
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Swagger 文档
    location /swagger/ {
        proxy_pass http://tgortc_backend/swagger/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # 健康检查
    location /health {
        proxy_pass http://tgortc_backend/health;
        access_log off;  # 不记录健康检查日志
    }
}
```

### 步骤 2：创建 LiveKit 配置文件

```bash
vim /etc/nginx/conf.d/livekit.conf
```

**内容：**

```nginx
# LiveKit 服务配置
upstream livekit_backend {
    server 127.0.0.1:7880;
}

server {
    listen 80;
    server_name livekit.yourdomain.com;  # 替换为你的域名

    # 访问日志
    access_log /var/log/nginx/livekit-access.log;
    error_log /var/log/nginx/livekit-error.log;

    location / {
        proxy_pass http://livekit_backend;
        proxy_http_version 1.1;
        
        # WebSocket 支持（LiveKit 需要）
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # 传递真实客户端信息
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 超时设置（WebSocket 需要更长的超时时间）
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
    }
}
```

### 步骤 3：测试配置

```bash
# 测试 Nginx 配置
nginx -t

# 预期输出：
# nginx: the configuration file /etc/nginx/nginx.conf syntax is ok
# nginx: configuration file /etc/nginx/nginx.conf test is successful
```

### 步骤 4：重载 Nginx

```bash
# 重载配置
systemctl reload nginx

# 或者重启 Nginx
systemctl restart nginx
```

### 步骤 5：验证 HTTP 访问

```bash
# 测试 API
curl http://api.yourdomain.com/health

# 测试 Swagger
curl -I http://api.yourdomain.com/swagger/index.html

# 测试 LiveKit
curl -I http://livekit.yourdomain.com
```

---

## 配置 HTTPS（Let's Encrypt）

### 步骤 1：安装 Certbot

#### CentOS/RHEL

```bash
# 安装 EPEL 仓库
yum install -y epel-release

# 安装 Certbot
yum install -y certbot python3-certbot-nginx
```

#### Ubuntu/Debian

```bash
# 安装 Certbot
apt install -y certbot python3-certbot-nginx
```

### 步骤 2：申请 SSL 证书

```bash
# 为 TgoRTC API 申请证书
certbot --nginx -d api.yourdomain.com

# 为 LiveKit 申请证书
certbot --nginx -d livekit.yourdomain.com
```

**交互式提示：**
1. 输入邮箱地址（用于证书过期提醒）
2. 同意服务条款（输入 `Y`）
3. 选择是否重定向 HTTP 到 HTTPS（推荐选择 `2`）

### 步骤 3：验证 HTTPS

```bash
# 测试 HTTPS
curl https://api.yourdomain.com/health

# 浏览器访问
# https://api.yourdomain.com/swagger/index.html
```

### 步骤 4：自动续期

Let's Encrypt 证书有效期 90 天，需要自动续期。

```bash
# 测试自动续期
certbot renew --dry-run

# 查看定时任务（Certbot 会自动创建）
systemctl list-timers | grep certbot
```

### 完整的 HTTPS 配置示例

Certbot 会自动修改配置文件，最终的配置类似：

```nginx
# TgoRTC API HTTPS 配置
server {
    listen 443 ssl http2;
    server_name api.yourdomain.com;

    # SSL 证书
    ssl_certificate /etc/letsencrypt/live/api.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.yourdomain.com/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    # 访问日志
    access_log /var/log/nginx/tgortc-api-access.log;
    error_log /var/log/nginx/tgortc-api-error.log;

    # 客户端最大上传大小
    client_max_body_size 10M;

    location / {
        proxy_pass http://tgortc_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# HTTP 重定向到 HTTPS
server {
    listen 80;
    server_name api.yourdomain.com;
    return 301 https://$server_name$request_uri;
}
```

---

## WebSocket 支持

LiveKit 需要 WebSocket 支持，确保配置中包含以下内容：

```nginx
location / {
    proxy_pass http://livekit_backend;
    proxy_http_version 1.1;
    
    # WebSocket 必需配置
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    
    # 长连接超时
    proxy_connect_timeout 7d;
    proxy_send_timeout 7d;
    proxy_read_timeout 7d;
}
```

---

## 负载均衡配置

如果你有多个 TgoRTC Server 实例，可以配置负载均衡：

```nginx
upstream tgortc_backend {
    # 负载均衡策略：轮询（默认）
    server 127.0.0.1:8080;
    server 127.0.0.1:8081;
    server 127.0.0.1:8082;
    
    # 或使用 IP Hash（同一客户端总是访问同一服务器）
    # ip_hash;
    
    # 或使用最少连接
    # least_conn;
    
    # 健康检查（需要 nginx-plus 或第三方模块）
    # health_check interval=5s fails=3 passes=2;
}
```

---

## 常见问题

### 问题 1：502 Bad Gateway

**原因：** Nginx 无法连接到后端服务

**解决方案：**
```bash
# 检查后端服务是否运行
curl http://localhost:8080/health

# 检查防火墙
systemctl status firewalld

# 检查 SELinux（CentOS）
getenforce
# 如果是 Enforcing，需要配置 SELinux
setsebool -P httpd_can_network_connect 1
```

### 问题 2：413 Request Entity Too Large

**原因：** 上传文件超过 Nginx 限制

**解决方案：**
```nginx
# 在 server 或 location 块中添加
client_max_body_size 100M;
```

### 问题 3：WebSocket 连接失败

**原因：** 缺少 WebSocket 配置

**解决方案：**
```nginx
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
proxy_http_version 1.1;
```

### 问题 4：HTTPS 证书过期

**检查证书有效期：**
```bash
certbot certificates
```

**手动续期：**
```bash
certbot renew
systemctl reload nginx
```

---

## 性能优化

### 1. 启用 Gzip 压缩

```nginx
# 在 http 块中添加
gzip on;
gzip_vary on;
gzip_min_length 1024;
gzip_types text/plain text/css application/json application/javascript text/xml application/xml;
```

### 2. 启用缓存

```nginx
# 静态资源缓存
location ~* \.(jpg|jpeg|png|gif|ico|css|js)$ {
    expires 30d;
    add_header Cache-Control "public, immutable";
}
```

### 3. 限流配置

```nginx
# 在 http 块中定义限流区域
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;

# 在 location 中应用
location /api/ {
    limit_req zone=api_limit burst=20 nodelay;
    proxy_pass http://tgortc_backend;
}
```

---

## 安全加固

### 1. 隐藏 Nginx 版本

```nginx
# 在 http 块中添加
server_tokens off;
```

### 2. 添加安全头

```nginx
# 在 server 块中添加
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "no-referrer-when-downgrade" always;
```

### 3. IP 白名单

```nginx
# 只允许特定 IP 访问
location /admin/ {
    allow 192.168.1.0/24;
    allow 10.0.0.1;
    deny all;
    proxy_pass http://tgortc_backend;
}
```

---

## 配置文件模板

完整的生产环境配置模板已保存在：
- `deployment/nginx/tgortc-api.conf`
- `deployment/nginx/livekit.conf`

---

## 总结

本文档介绍了：

✅ Nginx 安装和基础配置  
✅ HTTP 反向代理配置  
✅ HTTPS 证书申请和配置  
✅ WebSocket 支持  
✅ 负载均衡配置  
✅ 性能优化和安全加固  

### 配置检查清单

- [ ] Nginx 已安装并运行
- [ ] 域名已解析到服务器 IP
- [ ] HTTP 反向代理配置完成
- [ ] HTTPS 证书申请成功
- [ ] WebSocket 配置正确
- [ ] 防火墙已开放 80 和 443 端口
- [ ] SELinux 配置正确（CentOS）
- [ ] 证书自动续期已配置

### 相关文档

- [生产环境部署指南](./PRODUCTION_DEPLOYMENT_GUIDE.md)
- [HTTPS 配置详解](./HTTPS_CONFIGURATION.md)
- [性能优化指南](./PERFORMANCE_OPTIMIZATION.md)

---

**文档版本**: 1.0  
**更新日期**: 2025-11-05  
**作者**: TgoRTC Team

