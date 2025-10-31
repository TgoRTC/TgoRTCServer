#!/bin/bash

################################################################################
# LiveKit 统一部署脚本
# 支持单机和分布式部署
# 通过 NODES 配置控制部署方式
################################################################################

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 脚本配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOYMENT_DIR="${SCRIPT_DIR}/livekit-deployment"
CONFIG_DIR="${DEPLOYMENT_DIR}/config"
BACKUP_DIR="${DEPLOYMENT_DIR}/backups"
LOG_FILE="${DEPLOYMENT_DIR}/deploy.log"

# 加载环境变量
if [ -f "${SCRIPT_DIR}/.env" ]; then
    source "${SCRIPT_DIR}/.env"
else
    echo -e "${RED}[ERROR]${NC} .env 文件不存在，请先复制 .env.example 为 .env"
    exit 1
fi

################################################################################
# 日志函数
################################################################################

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$LOG_FILE"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$LOG_FILE"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$LOG_FILE"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$LOG_FILE"
}

################################################################################
# 检查依赖
################################################################################

check_dependencies() {
    log_info "检查依赖..."
    
    local missing_deps=()
    
    if ! command -v docker &> /dev/null; then
        missing_deps+=("docker")
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        missing_deps+=("docker-compose")
    fi
    
    if ! command -v curl &> /dev/null; then
        missing_deps+=("curl")
    fi
    
    if ! command -v openssl &> /dev/null; then
        missing_deps+=("openssl")
    fi
    
    if [ ${#missing_deps[@]} -gt 0 ]; then
        log_error "缺少以下依赖: ${missing_deps[*]}"
        exit 1
    fi
    
    log_success "所有依赖已安装"
}

################################################################################
# 初始化目录
################################################################################

init_directories() {
    log_info "初始化目录结构..."
    mkdir -p "$DEPLOYMENT_DIR"
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$BACKUP_DIR"
    mkdir -p "$(dirname "$LOG_FILE")"
    touch "$LOG_FILE"
    log_success "目录已初始化"
}

################################################################################
# 生成 API 密钥
################################################################################

generate_api_keys() {
    if [ -z "$LIVEKIT_API_KEY" ]; then
        LIVEKIT_API_KEY=$(openssl rand -hex 12)
    fi
    
    if [ -z "$LIVEKIT_API_SECRET" ]; then
        LIVEKIT_API_SECRET=$(openssl rand -hex 32)
    fi
    
    log_info "API 密钥已生成"
}

################################################################################
# 生成配置文件
################################################################################

generate_livekit_config() {
    log_info "生成 LiveKit 配置..."

    local config_file="${CONFIG_DIR}/livekit.yaml"

    # 生成基础配置
    cat > "$config_file" << EOF
port: 7880
bind_addresses:
  - 0.0.0.0

keys:
  $LIVEKIT_API_KEY: $LIVEKIT_API_SECRET

redis:
  address: $REDIS_HOST:$REDIS_PORT
  password: $REDIS_PASSWORD
  db: $REDIS_DB

room:
  auto_create: $LIVEKIT_AUTO_CREATE_ROOM
  empty_timeout: $LIVEKIT_EMPTY_TIMEOUT
  max_participants: $LIVEKIT_MAX_PARTICIPANTS

logging:
  level: $LIVEKIT_LOG_LEVEL

turn:
  enabled: $TURN_ENABLED
  domain: $TURN_DOMAIN
  external_tls_port: $TURN_EXTERNAL_TLS_PORT
  external_udp_port: $TURN_EXTERNAL_UDP_PORT
EOF

    # 如果启用了 Webhook，添加 Webhook 配置
    if [ "$WEBHOOK_ENABLED" = "true" ] && [ -n "$WEBHOOK_URLS" ]; then
        log_info "添加 Webhook 配置..."
        cat >> "$config_file" << EOF

webhook:
  api_key: $WEBHOOK_API_KEY
  urls:
EOF
        # 处理逗号分隔的 URLs
        IFS=',' read -ra URLS <<< "$WEBHOOK_URLS"
        for url in "${URLS[@]}"; do
            url=$(echo "$url" | xargs)  # 去除空格
            echo "    - $url" >> "$config_file"
        done
    fi

    log_success "LiveKit 配置已生成"
}

generate_redis_config() {
    log_info "生成 Redis 配置..."
    
    local config_file="${CONFIG_DIR}/redis.conf"
    
    cat > "$config_file" << EOF
port 6379
bind 0.0.0.0
maxmemory $REDIS_MAXMEMORY
maxmemory-policy $REDIS_MAXMEMORY_POLICY
appendonly yes
appendfsync everysec
EOF
    
    log_success "Redis 配置已生成"
}

generate_nginx_config() {
    log_info "生成 Nginx 配置..."

    local config_file="${CONFIG_DIR}/nginx.conf"
    local livekit_servers=""

    # 生成 LiveKit 上游服务器配置
    if [ -z "$LIVEKIT_NODES" ]; then
        # 单机模式：使用内置 LiveKit
        log_info "使用内置 LiveKit 服务（单机模式）"
        livekit_servers="        server localhost:7880 max_fails=3 fail_timeout=30s;"
    else
        # 集群模式：使用远程 LiveKit 节点
        log_info "使用远程 LiveKit 节点（集群模式）: $LIVEKIT_NODES"
        IFS=',' read -ra nodes <<< "$LIVEKIT_NODES"
        for node in "${nodes[@]}"; do
            node=$(echo "$node" | xargs)  # 去除空格
            livekit_servers+="        server $node max_fails=3 fail_timeout=30s;"$'\n'
        done
    fi

    cat > "$config_file" << 'EOF'
user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for"';

    access_log /var/log/nginx/access.log main;

    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;
    client_max_body_size 20M;

    # 上游服务器配置
    upstream livekit_backend {
        least_conn;
LIVEKIT_SERVERS_PLACEHOLDER
    }

    # HTTP 重定向到 HTTPS
    server {
        listen 80;
        server_name _;

        location /.well-known/acme-challenge/ {
            root /var/www/certbot;
        }

        location / {
            return 301 https://$host$request_uri;
        }
    }

    # HTTPS 服务器
    server {
        listen 443 ssl http2;
        server_name _;

        ssl_certificate /etc/letsencrypt/live/DOMAIN/fullchain.pem;
        ssl_certificate_key /etc/letsencrypt/live/DOMAIN/privkey.pem;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;
        ssl_prefer_server_ciphers on;

        location / {
            proxy_pass http://livekit_backend;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_read_timeout 86400;
        }
    }
}
EOF

    # 替换占位符
    sed -i "s|DOMAIN|$DOMAIN|g" "$config_file"
    sed -i "s|LIVEKIT_SERVERS_PLACEHOLDER|$livekit_servers|g" "$config_file"

    log_success "Nginx 配置已生成"
}

################################################################################
# 生成 Docker Compose 配置
################################################################################

generate_docker_compose() {
    log_info "生成 Docker Compose 配置..."

    local compose_file="${DEPLOYMENT_DIR}/docker-compose.yml"

    # 根据 LIVEKIT_NODES 判断是单机还是集群模式
    if [ -z "$LIVEKIT_NODES" ]; then
        log_info "单机模式：生成包含内置 LiveKit 的 Docker Compose"
        generate_docker_compose_single "$compose_file"
    else
        log_info "集群模式：生成不包含 LiveKit 的 Docker Compose"
        generate_docker_compose_single "$compose_file"
    fi
}

generate_docker_compose_single() {
    local compose_file=$1
    local livekit_service=""
    local redis_service=""
    local nginx_depends=""

    # 如果没有配置远程 LiveKit 节点，则使用内置 LiveKit
    if [ -z "$LIVEKIT_NODES" ]; then
        log_info "生成包含内置 LiveKit 的 Docker Compose"

        redis_service='
  redis:
    image: redis:7-alpine
    container_name: livekit-redis
    ports:
      - "6379:6379"
    volumes:
      - ./config/redis.conf:/usr/local/etc/redis/redis.conf:ro
      - ./volumes/redis:/data
    command: redis-server /usr/local/etc/redis/redis.conf
    networks:
      - livekit
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5'

        livekit_service='
  livekit:
    image: livekit/livekit-server:latest
    container_name: livekit-server
    ports:
      - "7880:7880"
      - "7881:7880"
      - "7882:7880"
      - "50000-60000:50000-60000/udp"
      - "3478:3478/udp"
    volumes:
      - ./config/livekit.yaml:/etc/livekit.yaml:ro
    environment:
      - LIVEKIT_CONFIG=/etc/livekit.yaml
    command: --config /etc/livekit.yaml
    networks:
      - livekit
    depends_on:
      redis:
        condition: service_healthy
    restart: unless-stopped'

        nginx_depends="    depends_on:
      - livekit"
    else
        log_info "使用远程 LiveKit 节点，不部署内置 LiveKit"
    fi

    cat > "$compose_file" << EOF
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    container_name: livekit-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./volumes/letsencrypt:/etc/letsencrypt:ro
      - ./volumes/certbot:/var/www/certbot:ro
    networks:
      - livekit
    restart: unless-stopped
$nginx_depends

  certbot:
    image: certbot/certbot:latest
    container_name: livekit-certbot
    volumes:
      - ./volumes/letsencrypt:/etc/letsencrypt
      - ./volumes/certbot:/var/www/certbot
    entrypoint: /bin/sh -c "trap exit TERM; while :; do certbot renew --webroot -w /var/www/certbot --quiet; sleep 12h & wait \$\${!}; done"
    networks:
      - livekit
    restart: unless-stopped
$redis_service
$livekit_service

networks:
  livekit:
    driver: bridge

volumes:
  redis_data:
  letsencrypt:
  certbot:
EOF

    log_success "Docker Compose 配置已生成"
}

generate_docker_compose_multi() {
    local compose_file=$1
    local num_nodes=$2

    cat > "$compose_file" << 'EOF'
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    container_name: livekit-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./volumes/letsencrypt:/etc/letsencrypt:ro
      - ./volumes/certbot:/var/www/certbot:ro
    networks:
      - livekit
    restart: unless-stopped

  certbot:
    image: certbot/certbot:latest
    container_name: livekit-certbot
    volumes:
      - ./volumes/letsencrypt:/etc/letsencrypt
      - ./volumes/certbot:/var/www/certbot
    entrypoint: /bin/sh -c "trap exit TERM; while :; do certbot renew --webroot -w /var/www/certbot --quiet; sleep 12h & wait $${!}; done"
    networks:
      - livekit
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    container_name: livekit-redis
    ports:
      - "6379:6379"
    volumes:
      - ./config/redis.conf:/usr/local/etc/redis/redis.conf:ro
      - ./volumes/redis:/data
    command: redis-server /usr/local/etc/redis/redis.conf
    networks:
      - livekit
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
EOF
    
    # 生成多个节点配置
    for ((i=1; i<=num_nodes; i++)); do
        local port_base=$((7880 + i - 1))
        local udp_port_base=$((50000 + (i-1)*100))
        local turn_port=$((3478 + i - 1))
        
        cat >> "$compose_file" << EOF

  livekit-node-$i:
    image: livekit/livekit-server:latest
    container_name: livekit-node-$i
    ports:
      - "$port_base:7880"
      - "$((port_base+1)):7880"
      - "$((port_base+2)):7880"
      - "$udp_port_base-$((udp_port_base+99)):50000-50100/udp"
      - "$turn_port:3478/udp"
    volumes:
      - ./config/livekit.yaml:/etc/livekit.yaml:ro
    environment:
      - LIVEKIT_CONFIG=/etc/livekit.yaml
      - NODE_ID=node-$i
    command: --config /etc/livekit.yaml
    networks:
      - livekit
    depends_on:
      redis:
        condition: service_healthy
    restart: unless-stopped
EOF
    done
    
    cat >> "$compose_file" << 'EOF'

networks:
  livekit:
    driver: bridge

volumes:
  redis_data:
  caddy_data:
  caddy_config:
EOF
    
    log_success "分布式 Docker Compose 配置已生成 ($num_nodes 个节点)"
}

################################################################################
# 部署函数
################################################################################

deploy() {
    log_info "开始部署..."

    check_dependencies
    init_directories
    generate_nginx_config

    # 如果没有配置远程 LiveKit 节点，则部署内置 LiveKit
    if [ -z "$LIVEKIT_NODES" ]; then
        log_info "单机模式：部署内置 LiveKit"
        generate_api_keys
        generate_livekit_config
        generate_redis_config
    else
        log_info "集群模式：使用远程 LiveKit 节点"
        log_info "远程 LiveKit 节点: $LIVEKIT_NODES"
    fi

    generate_docker_compose

    log_info "启动 Docker 容器..."
    cd "$DEPLOYMENT_DIR"
    docker-compose up -d

    sleep 5

    log_success "部署完成！"

    if [ -z "$LIVEKIT_NODES" ]; then
        log_info "API 密钥: $LIVEKIT_API_KEY"
        log_info "API 密钥密码: $LIVEKIT_API_SECRET"
    fi

    log_info "访问地址: https://$DOMAIN"
}

deploy_nginx_service_only() {
    log_info "开始部署 Nginx + 业务服务..."

    check_dependencies
    init_directories
    generate_nginx_config

    log_info "生成 Nginx 专用的 Docker Compose..."
    local compose_file="${DEPLOYMENT_DIR}/docker-compose.yml"

    cat > "$compose_file" << 'EOF'
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    container_name: livekit-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./volumes/letsencrypt:/etc/letsencrypt:ro
      - ./volumes/certbot:/var/www/certbot:ro
    networks:
      - livekit
    restart: unless-stopped

  certbot:
    image: certbot/certbot:latest
    container_name: livekit-certbot
    volumes:
      - ./volumes/letsencrypt:/etc/letsencrypt
      - ./volumes/certbot:/var/www/certbot
    entrypoint: /bin/sh -c "trap exit TERM; while :; do certbot renew --webroot -w /var/www/certbot --quiet; sleep 12h & wait $${!}; done"
    networks:
      - livekit
    restart: unless-stopped

networks:
  livekit:
    driver: bridge

volumes:
  letsencrypt:
  certbot:
EOF

    log_info "启动 Nginx 容器..."
    cd "$DEPLOYMENT_DIR"
    docker-compose up -d

    sleep 5

    log_success "Nginx 部署完成！"
    log_info "访问地址: https://$DOMAIN"
    log_info ""
    log_info "下一步："
    log_info "1. 在这台机器上部署你的业务服务"
    log_info "2. 确保业务服务监听在 localhost:8080"
    log_info "3. 运行 certbot 申请 HTTPS 证书"
}

deploy_nginx_only() {
    log_info "开始部署 Nginx 反向代理..."

    check_dependencies
    init_directories
    generate_nginx_config

    log_info "生成 Nginx 专用的 Docker Compose..."
    local compose_file="${DEPLOYMENT_DIR}/docker-compose.yml"

    cat > "$compose_file" << 'EOF'
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    container_name: livekit-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./volumes/letsencrypt:/etc/letsencrypt:ro
      - ./volumes/certbot:/var/www/certbot:ro
    networks:
      - livekit
    restart: unless-stopped

  certbot:
    image: certbot/certbot:latest
    container_name: livekit-certbot
    volumes:
      - ./volumes/letsencrypt:/etc/letsencrypt
      - ./volumes/certbot:/var/www/certbot
    entrypoint: /bin/sh -c "trap exit TERM; while :; do certbot renew --webroot -w /var/www/certbot --quiet; sleep 12h & wait $${!}; done"
    networks:
      - livekit
    restart: unless-stopped

networks:
  livekit:
    driver: bridge

volumes:
  letsencrypt:
  certbot:
EOF

    log_info "启动 Nginx 容器..."
    cd "$DEPLOYMENT_DIR"
    docker-compose up -d

    sleep 5

    log_success "Nginx 部署完成！"
    log_info "访问地址: https://$DOMAIN"
}

deploy_livekit_only() {
    log_info "开始部署 LiveKit 节点..."

    check_dependencies
    init_directories
    generate_api_keys
    generate_livekit_config
    generate_redis_config
    generate_docker_compose

    log_info "启动 Docker 容器..."
    cd "$DEPLOYMENT_DIR"
    docker-compose up -d

    sleep 5

    log_success "LiveKit 部署完成！"
    log_info "API 密钥: $LIVEKIT_API_KEY"
    log_info "API 密钥密码: $LIVEKIT_API_SECRET"
}

################################################################################
# 管理函数
################################################################################

init_https_cert() {
    log_info "初始化 HTTPS 证书..."

    cd "$DEPLOYMENT_DIR"

    # 创建临时 nginx 配置用于证书申请
    mkdir -p ./volumes/certbot

    # 启动 nginx 和 certbot
    docker-compose up -d nginx certbot

    sleep 5

    # 申请证书
    log_info "申请 Let's Encrypt 证书..."
    docker-compose exec -T certbot certbot certonly --webroot -w /var/www/certbot \
        -d "$DOMAIN" \
        --email admin@"$DOMAIN" \
        --agree-tos \
        --non-interactive \
        --quiet

    if [ $? -eq 0 ]; then
        log_success "HTTPS 证书申请成功"
        log_info "证书位置: ./volumes/letsencrypt/live/$DOMAIN/"
    else
        log_error "HTTPS 证书申请失败，请检查域名和网络配置"
        return 1
    fi
}

start_services() {
    log_info "启动服务..."
    cd "$DEPLOYMENT_DIR"
    docker-compose up -d
    log_success "服务已启动"
}

stop_services() {
    log_info "停止服务..."
    cd "$DEPLOYMENT_DIR"
    docker-compose down
    log_success "服务已停止"
}

restart_services() {
    log_info "重启服务..."
    cd "$DEPLOYMENT_DIR"
    docker-compose restart
    log_success "服务已重启"
}

view_logs() {
    cd "$DEPLOYMENT_DIR"
    docker-compose logs -f "$1"
}

backup_data() {
    log_info "备份数据..."
    
    local backup_name="backup-$(date +%Y%m%d-%H%M%S)"
    local backup_path="${BACKUP_DIR}/${backup_name}"
    
    mkdir -p "$backup_path"
    
    cd "$DEPLOYMENT_DIR"
    
    docker-compose exec -T redis redis-cli BGSAVE
    sleep 2
    cp -r volumes/redis "$backup_path/" 2>/dev/null || true
    cp -r config "$backup_path/"
    
    log_success "数据已备份到: $backup_path"
}

restore_data() {
    log_info "恢复数据..."
    
    if [ -z "$1" ] || [ ! -d "$1" ]; then
        log_error "备份目录不存在: $1"
        exit 1
    fi
    
    cd "$DEPLOYMENT_DIR"
    
    docker-compose down
    
    rm -rf volumes/redis
    cp -r "$1/redis" volumes/ 2>/dev/null || true
    cp -r "$1/config" .
    
    docker-compose up -d
    
    log_success "数据已恢复"
}

verify_deployment() {
    log_info "验证部署..."
    
    cd "$DEPLOYMENT_DIR"
    
    log_info "检查容器状态..."
    docker-compose ps
    
    sleep 5
    
    log_info "检查 LiveKit 健康状态..."
    if curl -s http://localhost:7880/ > /dev/null; then
        log_success "LiveKit 服务正常运行"
    else
        log_warning "LiveKit 服务未响应"
    fi
    
    log_info "检查 Redis 连接..."
    if docker-compose exec -T redis redis-cli ping > /dev/null 2>&1; then
        log_success "Redis 连接正常"
    else
        log_warning "Redis 连接失败"
    fi
    
    log_success "验证完成"
}

################################################################################
# 帮助函数
################################################################################

show_help() {
    cat << EOF
LiveKit 统一部署脚本

用法: $0 [命令]

命令:
  deploy                      部署 LiveKit（包括 Caddy 和 LiveKit）
  deploy-caddy-service-only   只部署 Caddy + 业务服务（推荐用于分布式）
  deploy-caddy-only           只部署 Caddy 反向代理
  deploy-livekit-only         只部署 LiveKit 节点
  start                       启动服务
  stop                        停止服务
  restart                     重启服务
  logs                        查看日志
  backup                      备份数据
  restore                     恢复数据
  verify                      验证部署
  help                        显示帮助信息

部署方式:
  单机部署（1 台机器，内置 LiveKit）：
    $0 deploy
    go run 你的服务.go

  集群部署（多台机器，远程 LiveKit）：
    机器 1: 本服务 + Nginx（指向远程 LiveKit）
    机器 2, 3, 4: LiveKit 节点

示例:
  # 单机部署（内置 LiveKit）
  $0 deploy
  $0 init-https
  go run 你的服务.go

  # 集群部署 - 机器 1（本服务 + Nginx）
  LIVEKIT_NODES=192.168.1.2:7880,192.168.1.3:7880,192.168.1.4:7880 $0 deploy
  $0 init-https
  go run 你的服务.go

  # 集群部署 - 机器 2, 3, 4（LiveKit 节点）
  NODES=192.168.1.2,192.168.1.3,192.168.1.4 $0 deploy-livekit-only

  # 查看日志
  $0 logs livekit

  # 备份数据
  $0 backup

  # 恢复数据
  $0 restore /path/to/backup

EOF
}

################################################################################
# 主函数
################################################################################

main() {
    mkdir -p "$(dirname "$LOG_FILE")"
    touch "$LOG_FILE"
    
    local command="${1:-help}"
    
    case "$command" in
        deploy)
            deploy
            ;;
        deploy-nginx-service-only)
            deploy_nginx_service_only
            ;;
        deploy-nginx-only)
            deploy_nginx_only
            ;;
        deploy-caddy-service-only)
            deploy_nginx_service_only
            ;;
        deploy-caddy-only)
            deploy_nginx_only
            ;;
        deploy-livekit-only)
            deploy_livekit_only
            ;;
        init-https)
            init_https_cert
            ;;
        start)
            start_services
            ;;
        stop)
            stop_services
            ;;
        restart)
            restart_services
            ;;
        logs)
            view_logs "${2:-livekit}"
            ;;
        backup)
            backup_data
            ;;
        restore)
            restore_data "$2"
            ;;
        verify)
            verify_deployment
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "未知命令: $command"
            show_help
            exit 1
            ;;
    esac
    
    log_info "=========================================="
    log_info "操作完成"
    log_info "=========================================="
}

main "$@"

