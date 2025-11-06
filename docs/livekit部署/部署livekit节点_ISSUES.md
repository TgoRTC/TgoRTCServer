# LiveKit 分布式部署问题排查记录

## 问题 1：Nginx 启动失败 - 80 端口被占用

**错误信息：**
```
nginx: [emerg] bind() to 0.0.0.0:80 failed (98: Address already in use)
```

**原因：**
80 端口被 Apache (httpd) 服务占用

**解决方案：**
```bash
systemctl stop httpd
systemctl disable httpd
systemctl start nginx
```

---

## 问题 2：Nginx 返回默认页面，未代理到 LiveKit

**现象：**
执行 `curl -I http://localhost` 返回 Nginx 默认页面，不是 LiveKit

**原因：**
Nginx 主配置文件 `/etc/nginx/nginx.conf` 中有默认的 server 配置，监听 80 端口，与自定义配置冲突

**解决方案：**
```bash
# 编辑主配置文件
vim /etc/nginx/nginx.conf

# 找到默认 server 块（约 38-57 行），注释掉整个 server 块
# server {
#     listen       80;
#     ...
# }

# 重载 Nginx
nginx -t
systemctl reload nginx
```

---

## 问题 3：B 服务器 LiveKit 无法连接 Redis

**错误信息：**
```
unable to connect to redis: dial tcp 47.117.96.203:6380: i/o timeout
```

**原因：**
1. A 服务器 Redis 只监听 localhost，未对外开放
2. 阿里云安全组未开放 6380 端口

**解决方案：**

步骤 1：修改 A 服务器 Redis 配置
```bash
vim /opt/tgo-rtc/docker-compose.yml

# 修改 redis 服务的 ports 配置
redis:
  ports:
    - "0.0.0.0:6380:6379"

# 重启 Redis
docker compose restart redis
```

步骤 2：配置阿里云安全组
- 端口：`6380`
- 协议：`TCP`
- 授权对象：`39.103.125.196/32`（B 服务器 IP）

步骤 3：验证连接
```bash
# 在 B 服务器测试
telnet 47.117.96.203 6380

# 重启 LiveKit
docker compose restart
```

---

## 问题 4：配置文件内容写错位置

**错误信息：**
```
validating /opt/livekit/docker-compose.yml: additional properties 'port', 'rtc', 'keys', 'redis' not allowed
```

**原因：**
将 `livekit.yaml` 的内容错误地写到了 `docker-compose.yml` 文件中

**解决方案：**
需要创建两个独立的文件：

`docker-compose.yml`：
```yaml
version: '3.8'

services:
  livekit:
    image: livekit/livekit-server:latest
    container_name: livekit-node
    restart: always
    command: --config /etc/livekit.yaml
    volumes:
      - ./livekit.yaml:/etc/livekit.yaml
    ports:
      - "7880:7880"
      - "7881:7881"
      - "50000-50100:50000-50100/udp"
    network_mode: host
```

`livekit.yaml`：
```yaml
port: 7880
rtc:
  port_range_start: 50000
  port_range_end: 50100
  use_external_ip: true

keys:
  prodkey: Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9

redis:
  address: 47.117.96.203:6380
  password: TgoRedis@2024
  db: 0
```

---

## 问题 5：B 服务器未安装 Docker

**错误信息：**
```
Command 'docker' not found
```

**解决方案：**
```bash
# 使用阿里云镜像安装 Docker
apt-get update
apt-get install -y apt-transport-https ca-certificates curl software-properties-common

curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | apt-key add -
add-apt-repository "deb [arch=amd64] https://mirrors.aliyun.com/docker-ce/linux/ubuntu $(lsb_release -cs) stable"

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

systemctl start docker
systemctl enable docker
```

---

## 验证部署成功

### 1. B 服务器 LiveKit 容器正常运行
```bash
docker ps
# STATUS 显示 "Up X seconds/minutes"
```

### 2. LiveKit 日志显示服务启动
```bash
docker logs livekit-node --tail 20
# 应该看到：starting LiveKit server
```

### 3. Redis 连接成功
```bash
docker exec livekit-node sh -c "echo 'PING' | nc 47.117.96.203 6380"
# 返回：-NOAUTH Authentication required.
```

### 4. Nginx 负载均衡正常
```bash
curl http://localhost
# 返回：OK
```

### 5. Nginx 日志记录访问
```bash
tail /var/log/nginx/livekit-cluster-access.log
```

---

## 最终架构

```
客户端
  ↓
A 服务器 Nginx (80)
  ├─→ A 服务器 LiveKit (7880) ──┐
  └─→ B 服务器 LiveKit (7880) ──┤
                                ├─→ A 服务器 Redis (6380)
                                └─→ 共享房间状态
```

**特点：**
- ✅ 负载均衡：Nginx 分发请求到两个 LiveKit 节点
- ✅ 状态共享：通过 Redis 实现跨节点房间状态同步
- ✅ 高可用：单个节点故障不影响整体服务
- ✅ 可扩展：可以继续添加更多 LiveKit 节点

