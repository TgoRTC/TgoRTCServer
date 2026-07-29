# TgoRTC HTTPS + 域名配置指南

> 本文档基于 `deploy.sh` 一键部署后的环境，指导你将 HTTP 升级为 HTTPS。

## 前置条件

- 已通过 `deploy.sh` 完成部署，服务正常运行
- 拥有一个域名（以下以 `rtc.example.com` 为例）
- 服务器有公网 IP，且 80/443 端口可访问

---

## 第一步：域名解析

在你的域名服务商控制台添加 A 记录：

| 主机记录 | 记录类型 | 记录值         |
|---------|---------|---------------|
| rtc     | A       | 你的服务器公网IP |

添加后等待 DNS 生效（通常几分钟），验证：

```bash
ping rtc.example.com
```

---

## 第二步：申请 SSL 证书

### 方式一：Let's Encrypt 免费证书（推荐）

```bash
# 安装 certbot
# Ubuntu/Debian
sudo apt update && sudo apt install -y certbot

# CentOS/RHEL
sudo yum install -y certbot

# 申请前先暂停 Nginx（释放 80 端口）
cd ~/tgortc  # 或你的部署目录
sudo docker compose stop nginx

# 申请证书
sudo certbot certonly --standalone -d rtc.example.com

# 申请成功后证书路径：
#   证书:  /etc/letsencrypt/live/rtc.example.com/fullchain.pem
#   私钥:  /etc/letsencrypt/live/rtc.example.com/privkey.pem
```

### 方式二：云厂商证书

从腾讯云/阿里云等平台申请免费或付费证书，下载 Nginx 格式的证书文件，放到服务器上：

```bash
mkdir -p ~/tgortc/nginx/ssl
# 将证书文件上传到该目录
#   ~/tgortc/nginx/ssl/cert.pem      （证书文件）
#   ~/tgortc/nginx/ssl/cert.key      （私钥文件）
```

---

## 第三步：修改 .env

```bash
cd ~/tgortc  # 或你的部署目录
```

编辑 `.env` 文件，修改以下两项：

```ini
# 将 LIVEKIT_CLIENT_URL 改为 wss 协议 + 域名
# 修改前: LIVEKIT_CLIENT_URL=http://你的IP:80
LIVEKIT_CLIENT_URL=wss://rtc.example.com

# 将 SERVER_HOST 改为域名
# 修改前: SERVER_HOST=你的IP
SERVER_HOST=rtc.example.com
```

---

## 第四步：修改 nginx/nginx.conf

用以下内容替换 `nginx/nginx.conf`：

```nginx
upstream livekit_cluster {
    server livekit:7880 max_fails=3 fail_timeout=10s;
    ip_hash;
    keepalive 32;
}

# HTTP -> HTTPS 重定向
server {
    listen 80;
    server_name rtc.example.com;
    return 301 https://$host$request_uri;
}

# HTTPS 主配置
server {
    listen 443 ssl;
    server_name rtc.example.com;

    # ---------- SSL 证书配置 ----------
    # Let's Encrypt 证书（方式一）
    ssl_certificate     /etc/letsencrypt/live/rtc.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/rtc.example.com/privkey.pem;

    # 云厂商证书（方式二，使用时注释掉上面两行，取消下面两行的注释）
    # ssl_certificate     /etc/nginx/ssl/cert.pem;
    # ssl_certificate_key /etc/nginx/ssl/cert.key;

    # ---------- SSL 安全参数 ----------
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    access_log /var/log/nginx/livekit-cluster-access.log;
    error_log  /var/log/nginx/livekit-cluster-error.log;

    # LiveKit WebSocket 代理
    location / {
        proxy_pass http://livekit_cluster;
        proxy_http_version 1.1;

        # WebSocket 升级
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # 传递原始请求信息
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # 连接超时配置
        proxy_connect_timeout 5s;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;

        # 快速故障转移
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503;
        proxy_next_upstream_timeout 10s;
        proxy_next_upstream_tries 2;

        # 禁用缓冲（实时流媒体）
        proxy_buffering off;
    }

    # 健康检查端点
    location /health {
        access_log off;
        return 200 'OK';
        add_header Content-Type text/plain;
    }
}
```

> 注意：将所有 `rtc.example.com` 替换为你的实际域名。

---

## 第五步：修改 docker-compose.yml

编辑 `docker-compose.yml`，找到 `nginx` 服务部分，做两处修改：

### 1. 增加 443 端口映射

```yaml
  nginx:
    image: nginx:alpine
    container_name: tgo-rtc-nginx
    restart: always
    ports:
      - "80:80"
      - "443:443"    # 新增
```

### 2. 挂载证书目录

**Let's Encrypt 证书（方式一）：**

```yaml
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/conf.d/default.conf:ro
      - nginx_logs:/var/log/nginx
      - /etc/letsencrypt:/etc/letsencrypt:ro    # 新增
```

**云厂商证书（方式二）：**

```yaml
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/conf.d/default.conf:ro
      - nginx_logs:/var/log/nginx
      - ./nginx/ssl:/etc/nginx/ssl:ro           # 新增
```

---

## 第六步：修改 livekit.yaml

编辑 `livekit.yaml`，将 TURN 的 domain 改为域名：

```yaml
turn:
  enabled: true
  domain: rtc.example.com    # 将 IP 改为域名
  udp_port: 3478
  tls_port: 5349             # 新增：启用 TURN TLS
```

---

## 第七步：防火墙放通 443 端口

```bash
# UFW (Ubuntu)
sudo ufw allow 443/tcp

# firewalld (CentOS)
sudo firewall-cmd --permanent --add-port=443/tcp
sudo firewall-cmd --reload
```

如果使用云服务器，还需要在云控制台的安全组中放通 443/TCP：
- 腾讯云: https://console.cloud.tencent.com/cvm/securitygroup
- 阿里云: https://ecs.console.aliyun.com/ → 安全组
- 华为云: https://console.huaweicloud.com/ → 安全组

---

## 第八步：重启服务

```bash
cd ~/tgortc  # 或你的部署目录
sudo docker compose down
sudo docker compose up -d
```

---

## 验证

```bash
# 检查 HTTPS 是否正常
curl -I https://rtc.example.com/health

# 检查 HTTP 是否自动跳转
curl -I http://rtc.example.com/health
# 应返回 301 重定向到 https://

# 检查所有容器状态
sudo docker compose ps

# 查看 Nginx 日志
sudo docker compose logs nginx
```

---

## 证书自动续期（Let's Encrypt）

Let's Encrypt 证书有效期 90 天，设置 crontab 自动续期：

```bash
# 编辑 crontab
sudo crontab -e

# 添加以下行（每月 1 号凌晨 3 点自动续期）
0 3 1 * * certbot renew --pre-hook "cd ~/tgortc && docker compose stop nginx" --post-hook "cd ~/tgortc && docker compose start nginx" >> /var/log/certbot-renew.log 2>&1
```

---

## 修改汇总

| 文件                  | 修改内容                              |
|----------------------|--------------------------------------|
| `.env`               | `LIVEKIT_CLIENT_URL` 改为 `wss://域名` |
| `.env`               | `SERVER_HOST` 改为域名                 |
| `nginx/nginx.conf`   | 监听 443 + SSL 配置 + HTTP 跳转        |
| `docker-compose.yml` | 增加 443 端口、挂载证书目录              |
| `livekit.yaml`       | TURN domain 改为域名、启用 tls_port     |

---

## 常见问题

### Q: 证书申请失败？
确保 80 端口未被占用（`sudo lsof -i :80`），且域名已正确解析到服务器。

### Q: 浏览器提示证书不安全？
检查是否使用了正确的证书文件路径，以及证书是否已过期。

### Q: WebSocket 连接失败？
检查 `LIVEKIT_CLIENT_URL` 是否已改为 `wss://` 协议，以及客户端是否使用了正确的地址。

### Q: 重新部署后配置丢失？
`deploy.sh` 会重新生成配置文件。重新部署后需要重新执行本文档的步骤。建议提前备份修改过的配置文件。
