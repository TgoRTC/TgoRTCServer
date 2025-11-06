# 主服务器 Nginx 配置问题排查记录

## 问题 1：Nginx 未安装

**现象：**
```bash
nginx: command not found
```

**解决方案：**
```bash
# CentOS/RHEL
yum install -y nginx

# Ubuntu/Debian
apt-get install -y nginx

# 启动并设置开机自启
systemctl start nginx
systemctl enable nginx

# 验证安装
nginx -v
```

---

## 问题 2：Nginx 启动失败 - 80 端口被占用

**错误信息：**
```
nginx: [emerg] bind() to 0.0.0.0:80 failed (98: Address already in use)
```

**原因：**
80 端口被其他服务占用（通常是 Apache httpd）

**排查步骤：**
```bash
# 查看 80 端口占用情况
lsof -i:80

# 或者
netstat -tlnp | grep :80
```

**解决方案：**
```bash
# 如果是 httpd 占用
systemctl stop httpd
systemctl disable httpd

# 如果是其他服务，根据实际情况处理
# 然后启动 Nginx
systemctl start nginx
```

---

## 问题 3：Nginx 配置文件语法错误

**错误信息：**
```
nginx: [emerg] unknown directive "tream" in /etc/nginx/conf.d/livekit-cluster.conf:1
```

**原因：**
配置文件复制时出现错误，`upstream` 被错误复制为 `tream`

**解决方案：**
```bash
# 编辑配置文件
vim /etc/nginx/conf.d/livekit-cluster.conf

# 确保第一行是正确的
upstream livekit_cluster {
    ...
}

# 测试配置
nginx -t
```

---

## 问题 4：Nginx 返回默认页面，未代理到 LiveKit

**现象：**
```bash
curl -I http://localhost
# 返回 Nginx 默认页面（nginx/1.20.1）
# Content-Type: text/html
# Content-Length: 4833
```

**原因：**
Nginx 主配置文件 `/etc/nginx/nginx.conf` 中有默认的 server 配置，监听 80 端口，优先级高于自定义配置

**排查步骤：**
```bash
# 查看主配置文件中的 server 配置
cat /etc/nginx/nginx.conf | grep -A 5 "server {"
```

**解决方案：**
```bash
# 编辑主配置文件
vim /etc/nginx/nginx.conf

# 找到默认 server 块（约 38-57 行），注释掉整个 server 块
# 示例：
#    server {
#        listen       80;
#        listen       [::]:80;
#        server_name  _;
#        root         /usr/share/nginx/html;
#        ...
#    }

# 测试配置
nginx -t

# 重载 Nginx
systemctl reload nginx
```

---

## 问题 5：Nginx 配置冲突警告

**警告信息：**
```
nginx: [warn] conflicting server name "_" on 0.0.0.0:80, ignored
```

**原因：**
多个 server 块使用相同的 `server_name "_"`

**影响：**
这个警告不影响功能，可以忽略

**解决方案（可选）：**
```bash
# 修改自定义配置文件中的 server_name
vim /etc/nginx/conf.d/livekit-cluster.conf

# 将 server_name 改为具体域名或删除该行
server {
    listen 80;
    # server_name _;  # 删除或改为具体域名
    ...
}
```

---

## 问题 6：防火墙未开放端口

**现象：**
外部无法访问 Nginx（80/443 端口）

**排查步骤：**
```bash
# 检查防火墙状态
firewall-cmd --state

# 查看已开放端口
firewall-cmd --list-all
```

**解决方案：**

### 方案 1：使用 firewalld（CentOS/RHEL）
```bash
firewall-cmd --permanent --add-service=http
firewall-cmd --permanent --add-service=https
firewall-cmd --reload
```

### 方案 2：防火墙未运行
```
FirewallD is not running
```
说明服务器使用云服务商安全组管理端口，需要在云控制台配置

### 方案 3：阿里云安全组配置
1. 登录阿里云控制台
2. 找到 ECS 实例的安全组
3. 添加入方向规则：
   - 端口：`80/80`（HTTP）
   - 端口：`443/443`（HTTPS）
   - 协议：`TCP`
   - 授权对象：`0.0.0.0/0`

---

## 问题 7：SELinux 阻止 Nginx 代理

**现象：**
Nginx 配置正确，但无法连接到后端服务（LiveKit）

**错误日志：**
```
connect() to 127.0.0.1:7880 failed (13: Permission denied)
```

**排查步骤：**
```bash
# 检查 SELinux 状态
getenforce
```

**解决方案：**

### 如果 SELinux 是 Enforcing
```bash
# 允许 Nginx 网络连接
setsebool -P httpd_can_network_connect 1

# 重启 Nginx
systemctl restart nginx
```

### 如果 SELinux 是 Disabled
不需要处理

---

## 问题 8：Redis 未对外开放，从服务器无法连接

**现象：**
从服务器 LiveKit 无法连接到主服务器的 Redis

**错误信息：**
```
unable to connect to redis: dial tcp 47.117.96.203:6380: i/o timeout
```

**原因：**
主服务器 Redis 只监听 localhost（127.0.0.1），未对外开放

**排查步骤：**
```bash
# 查看 Redis 端口监听情况
netstat -tlnp | grep 6380

# 或者
docker compose ps
docker compose port redis 6379
```

**解决方案：**
```bash
# 编辑 docker-compose.yml
vim /opt/tgo-rtc/docker-compose.yml

# 修改 redis 服务的 ports 配置
redis:
  image: redis:7-alpine
  container_name: tgo-redis
  restart: always
  command: redis-server --requirepass ${REDIS_PASSWORD}
  ports:
    - "0.0.0.0:6380:6379"  # 修改这里，允许外部访问
  volumes:
    - redis_data:/data

# 重启 Redis
cd /opt/tgo-rtc
docker compose restart redis

# 验证端口监听
netstat -tlnp | grep 6380
# 应该看到：0.0.0.0:6380
```

**安全组配置：**
在阿里云控制台添加安全组规则：
- 端口：`6380`
- 协议：`TCP`
- 授权对象：`39.103.125.196/32`（从服务器 IP，不要用 0.0.0.0/0）

---

## 问题 9：Nginx 配置文件位置错误

**现象：**
配置文件创建后，Nginx 未加载

**原因：**
配置文件放在错误的目录

**正确位置：**
```bash
# 配置文件应该放在
/etc/nginx/conf.d/livekit-cluster.conf

# 不是
/etc/nginx/livekit-cluster.conf
```

**验证：**
```bash
# 查看主配置文件的 include 指令
cat /etc/nginx/nginx.conf | grep include

# 应该看到
include /etc/nginx/conf.d/*.conf;
```

---

## 问题 10：Nginx 日志文件不存在

**现象：**
无法查看 Nginx 访问日志

**解决方案：**
```bash
# 日志文件会在第一次访问时自动创建
# 如果不存在，手动创建目录
mkdir -p /var/log/nginx

# 重载 Nginx
systemctl reload nginx

# 测试访问
curl http://localhost

# 查看日志
tail -f /var/log/nginx/livekit-cluster-access.log
```

---

## 完整配置检查清单

### 1. Nginx 安装和启动
```bash
# 检查 Nginx 是否安装
nginx -v

# 检查 Nginx 是否运行
systemctl status nginx

# 检查 Nginx 是否开机自启
systemctl is-enabled nginx
```

### 2. 配置文件检查
```bash
# 检查配置文件是否存在
ls -l /etc/nginx/conf.d/livekit-cluster.conf

# 检查配置文件语法
nginx -t

# 查看配置文件内容
cat /etc/nginx/conf.d/livekit-cluster.conf
```

### 3. 端口检查
```bash
# 检查 80 端口是否被 Nginx 监听
netstat -tlnp | grep :80

# 检查 LiveKit 端口
netstat -tlnp | grep :7880

# 检查 Redis 端口
netstat -tlnp | grep :6380
```

### 4. 防火墙和安全组检查
```bash
# 检查本地防火墙
firewall-cmd --list-all

# 检查 SELinux
getenforce
```

### 5. 功能测试
```bash
# 测试 Nginx 代理
curl -I http://localhost

# 测试 LiveKit
curl http://localhost:7880

# 测试 Redis（从从服务器）
telnet 47.117.96.203 6380
```

### 6. 日志检查
```bash
# Nginx 错误日志
tail -f /var/log/nginx/error.log

# Nginx 访问日志
tail -f /var/log/nginx/livekit-cluster-access.log

# LiveKit 日志
docker logs -f livekit
```

---

## 常用管理命令

### Nginx 管理
```bash
# 启动
systemctl start nginx

# 停止
systemctl stop nginx

# 重启
systemctl restart nginx

# 重载配置（不中断服务）
systemctl reload nginx

# 查看状态
systemctl status nginx

# 测试配置
nginx -t

# 查看版本
nginx -v
```

### 配置文件管理
```bash
# 编辑配置
vim /etc/nginx/conf.d/livekit-cluster.conf

# 备份配置
cp /etc/nginx/conf.d/livekit-cluster.conf /etc/nginx/conf.d/livekit-cluster.conf.bak

# 恢复配置
cp /etc/nginx/conf.d/livekit-cluster.conf.bak /etc/nginx/conf.d/livekit-cluster.conf
```

### 日志管理
```bash
# 查看访问日志
tail -f /var/log/nginx/livekit-cluster-access.log

# 查看错误日志
tail -f /var/log/nginx/livekit-cluster-error.log

# 清空日志
> /var/log/nginx/livekit-cluster-access.log
```

---

## 最终验证

所有配置完成后，执行以下命令验证：

```bash
# 1. Nginx 状态
systemctl status nginx

# 2. 配置测试
nginx -t

# 3. 端口监听
netstat -tlnp | grep -E '80|7880|6380'

# 4. 功能测试
curl http://localhost

# 5. 查看日志
tail /var/log/nginx/livekit-cluster-access.log
```

**预期结果：**
- Nginx 状态：`active (running)`
- 配置测试：`syntax is ok` 和 `test is successful`
- 端口监听：80、7880、6380 都在监听
- 功能测试：返回 `OK`
- 日志：有访问记录

