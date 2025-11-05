# Docker Compose 快速参考

## 🚀 一键部署

### 单机部署（推荐用于开发/测试）

```bash
# 1. 复制环境配置
cp .env.example .env

# 2. 编辑 .env（修改 DOMAIN）
nano .env

# 3. 生成配置并启动
./部署.sh deploy

# 4. 申请 HTTPS 证书
./部署.sh init-https

# 5. 启动业务服务
go run main.go

# 完成！访问 https://livekit.example.com
```

### 集群部署（推荐用于生产环境）

**机器 1（本服务 + Nginx）：**
```bash
# 编辑 .env
DOMAIN=livekit.example.com
LIVEKIT_NODES=192.168.1.3:7880,192.168.1.4:7880
REDIS_HOST=192.168.1.2

# 部署
./部署.sh deploy
./部署.sh init-https
go run main.go
```

**机器 2（Redis）：**
```bash
docker run -d \
  --name livekit-redis \
  -p 6379:6379 \
  -v redis_data:/data \
  redis:7-alpine
```

**机器 3, 4（LiveKit 节点）：**
```bash
# 编辑 .env
REDIS_HOST=192.168.1.2
LIVEKIT_NODES=

# 部署
./部署.sh deploy-livekit-only
```

---

## 📝 常用命令速查表

| 命令 | 说明 |
|------|------|
| `./部署.sh deploy` | 部署完整服务（Nginx + LiveKit + Redis） |
| `./部署.sh deploy-livekit-only` | 只部署 LiveKit 节点 |
| `./部署.sh deploy-nginx-service-only` | 只部署 Nginx + 业务服务 |
| `./部署.sh init-https` | 申请 HTTPS 证书 |
| `./部署.sh start` | 启动所有服务 |
| `./部署.sh stop` | 停止所有服务 |
| `./部署.sh restart` | 重启所有服务 |
| `./部署.sh logs livekit` | 查看 LiveKit 日志 |
| `./部署.sh backup` | 备份数据 |
| `./部署.sh restore /path/to/backup` | 恢复数据 |
| `./部署.sh verify` | 验证部署 |

---

## 🐳 Docker Compose 命令

```bash
# 进入部署目录
cd livekit-deployment

# 启动所有服务
docker-compose up -d

# 停止所有服务
docker-compose down

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f livekit
docker-compose logs -f redis
docker-compose logs -f nginx

# 进入容器
docker-compose exec livekit bash
docker-compose exec redis redis-cli

# 重启服务
docker-compose restart
docker-compose restart livekit

# 删除所有容器和卷
docker-compose down -v
```

---

## 🔍 故障排查速查表

| 问题 | 命令 |
|------|------|
| Nginx 502 错误 | `docker-compose logs nginx` |
| LiveKit 无法启动 | `docker-compose logs livekit` |
| Redis 连接失败 | `docker-compose exec redis redis-cli ping` |
| 证书申请失败 | `docker-compose logs certbot` |
| 检查网络连接 | `docker network inspect livekit` |
| 检查容器状态 | `docker-compose ps` |

---

## 📊 配置文件位置

```
livekit-deployment/
├── config/
│   ├── nginx.conf          # Nginx 配置
│   ├── livekit.yaml        # LiveKit 配置
│   └── redis.conf          # Redis 配置
├── volumes/
│   ├── letsencrypt/        # HTTPS 证书
│   ├── certbot/            # Certbot 数据
│   └── redis/              # Redis 数据
├── docker-compose.yml      # Docker Compose 配置
└── deploy.log              # 部署日志
```

---

## 🌐 访问地址

| 服务 | 地址 | 说明 |
|------|------|------|
| 业务服务 | http://localhost:8080 | 本地开发 |
| LiveKit | http://localhost:7880 | 本地开发 |
| Redis | localhost:6379 | 本地开发 |
| 生产环境 | https://livekit.example.com | 通过 Nginx 反向代理 |

---

## 🔐 安全建议

1. **修改 Redis 密码**
   ```bash
   # 编辑 .env
   REDIS_PASSWORD=your_strong_password
   ```

2. **限制 Redis 访问**
   ```bash
   # 编辑 livekit-deployment/config/redis.conf
   bind 127.0.0.1  # 只允许本地访问
   ```

3. **启用防火墙**
   ```bash
   # 只开放必要的端口
   ufw allow 80/tcp
   ufw allow 443/tcp
   ufw allow 7880/tcp
   ufw allow 50000:60000/udp
   ```

4. **定期备份**
   ```bash
   # 每天自动备份
   0 2 * * * cd /path/to/TgoRTCServer && ./部署.sh backup
   ```

---

## 📈 性能监控

```bash
# 查看容器资源使用
docker stats

# 查看 Redis 内存使用
docker-compose exec redis redis-cli INFO memory

# 查看 LiveKit 统计信息
curl http://localhost:7880/metrics

# 查看 Nginx 连接数
docker-compose exec nginx netstat -an | grep ESTABLISHED | wc -l
```

---

## 🆘 获取帮助

```bash
# 查看部署脚本帮助
./部署.sh help

# 查看部署日志
tail -f livekit-deployment/deploy.log

# 查看 Docker 日志
docker logs livekit-nginx
docker logs livekit-server
docker logs livekit-redis
```

---

## 📚 相关文档

- [完整部署指南](./DOCKER_COMPOSE_DEPLOYMENT.md)
- [集群部署指南](./CLUSTER_DEPLOYMENT.md)
- [Nginx 配置指南](./NGINX_CONFIGURATION.md)
- [LiveKit 官方文档](https://docs.livekit.io/)

