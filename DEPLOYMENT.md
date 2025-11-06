# TgoRTC Server 部署指南

> 基于 LiveKit 的实时音视频通话服务

---

## 🚀 快速部署

### 1. 构建并推送镜像（本地）

```bash
# 克隆项目
git clone https://github.com/TgoRTC/TgoRTCServer.git
cd TgoRTCServer

# 构建镜像
make deploy

# 或手动构建
docker build -t registry.cn-shanghai.aliyuncs.com/yourname/tgo-rtc-server:latest . --platform linux/amd64
docker push registry.cn-shanghai.aliyuncs.com/yourname/tgo-rtc-server:latest
```

---

### 2. 服务器部署

```bash
# 下载 docker-compose.yml 和配置文件
wget https://raw.githubusercontent.com/TgoRTC/TgoRTCServer/main/docker-compose.prod.yml
wget https://raw.githubusercontent.com/TgoRTC/TgoRTCServer/main/livekit-deployment/config/livekit.yaml

# 修改 docker-compose.prod.yml 中的镜像地址
# image: registry.cn-shanghai.aliyuncs.com/yourname/tgo-rtc-server:latest

# 启动服务
docker compose -f docker-compose.prod.yml up -d

# 查看日志
docker compose -f docker-compose.prod.yml logs -f
```

---

## 📝 配置说明

### docker-compose.prod.yml

修改镜像地址和环境变量：

```yaml
services:
  tgo-rtc-server:
    image: registry.cn-shanghai.aliyuncs.com/yourname/tgo-rtc-server:latest  # 修改为你的镜像
    environment:
      - DB_PASSWORD=your_password  # 修改数据库密码
      - REDIS_PASSWORD=your_password  # 修改 Redis 密码
```

### livekit.yaml

如需外网访问，修改：

```yaml
rtc:
  use_external_ip: true
```

---

## 🔄 更新服务

```bash
# 本地：重新构建并推送
make deploy

# 服务器：拉取并重启
docker compose -f docker-compose.prod.yml pull tgo-rtc-server
docker compose -f docker-compose.prod.yml up -d tgo-rtc-server
```

---

## 📚 更多文档

- [详细部署指南](docs/guides/PRODUCTION_DEPLOYMENT_GUIDE.md)
- [API 文档](http://your-server-ip:8080/swagger/index.html)

---

## 📄 许可证

MIT License

