### 登录服务器并创建目录
```bash
mkdir -p /opt/livekit
cd /opt/livekit
```

### 创建 docker-compose.yml 文件
```bash
vim docker-compose.yml
```
复制以下内容并粘贴到文件中：
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
### 创建 livekit.yaml 文件
```bash
vim livekit.yaml
```
复制以下内容并粘贴到文件中：
```yaml
port: 7880
rtc:
  port_range_start: 50000
  port_range_end: 50100
  use_external_ip: true

keys:
  prodkey: Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9 ### 请修改为你的 Secret

redis:
  address: 0.0.0.0:6380 ### 请修改为你的 Redis 服务器地址和端口
  password: TgoRedis@2025 ### 请修改为你的 Redis 密码
  db: 0
port: 7880
rtc:
  port_range_start: 50000
  port_range_end: 50100
  use_external_ip: true

keys:
  prodkey: Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9

redis:
  address: 服务器IP:6380
  password: TgoRedis@2024
  db: 0

# ⚠️ 重要：添加 webhook 配置
webhook:
  api_key: prodkey
  urls:
    - http://服务器IP:8080/api/v1/webhooks/livekit
```
### 安装 Docker 
```bash
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh
```
### 使用阿里云镜像安装 Docker
```bash
# 安装必要的软件包
apt-get update
apt-get install -y apt-transport-https ca-certificates curl software-properties-common

# 添加阿里云 Docker 镜像源
curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | apt-key add -
add-apt-repository "deb [arch=amd64] https://mirrors.aliyun.com/docker-ce/linux/ubuntu $(lsb_release -cs) stable"

# 安装 Docker
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
```
### 启动 LiveKit 服务
```bash
docker-compose up -d
```

#### 问题
