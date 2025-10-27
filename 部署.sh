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

generate_caddy_config() {
    log_info "生成 Caddy 配置..."
    
    local config_file="${CONFIG_DIR}/Caddyfile"
    
    cat > "$config_file" << EOF
$DOMAIN {
    reverse_proxy localhost:7880 {
        header_up X-Forwarded-For {http.request.remote}
        header_up X-Forwarded-Proto {http.request.proto}
    }
}
EOF
    
    log_success "Caddy 配置已生成"
}

################################################################################
# 生成 Docker Compose 配置
################################################################################

generate_docker_compose() {
    log_info "生成 Docker Compose 配置..."
    
    local compose_file="${DEPLOYMENT_DIR}/docker-compose.yml"
    local nodes_array=(${NODES//,/ })
    local num_nodes=${#nodes_array[@]}
    
    # 检查是否为单机部署
    if [ $num_nodes -eq 1 ] && ([ "${nodes_array[0]}" == "localhost" ] || [ "${nodes_array[0]}" == "127.0.0.1" ]); then
        log_info "检测到单机部署模式"
        generate_docker_compose_single "$compose_file"
    else
        log_info "检测到分布式部署模式 ($num_nodes 个节点)"
        generate_docker_compose_multi "$compose_file" "$num_nodes"
    fi
}

generate_docker_compose_single() {
    local compose_file=$1
    
    cat > "$compose_file" << 'EOF'
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    container_name: livekit-caddy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/Caddyfile:/etc/caddy/Caddyfile:ro
      - ./volumes/caddy/data:/data
      - ./volumes/caddy/config:/config
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
    restart: unless-stopped

networks:
  livekit:
    driver: bridge

volumes:
  redis_data:
  caddy_data:
  caddy_config:
EOF
    
    log_success "单机 Docker Compose 配置已生成"
}

generate_docker_compose_multi() {
    local compose_file=$1
    local num_nodes=$2
    
    cat > "$compose_file" << 'EOF'
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    container_name: livekit-caddy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/Caddyfile:/etc/caddy/Caddyfile:ro
      - ./volumes/caddy/data:/data
      - ./volumes/caddy/config:/config
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
    generate_api_keys
    generate_livekit_config
    generate_redis_config
    generate_caddy_config
    generate_docker_compose

    log_info "启动 Docker 容器..."
    cd "$DEPLOYMENT_DIR"
    docker-compose up -d

    sleep 5

    log_success "部署完成！"
    log_info "API 密钥: $LIVEKIT_API_KEY"
    log_info "API 密钥密码: $LIVEKIT_API_SECRET"
    log_info "访问地址: https://$DOMAIN"
}

deploy_caddy_service_only() {
    log_info "开始部署 Caddy + 业务服务..."

    check_dependencies
    init_directories
    generate_caddy_config

    log_info "生成 Caddy 专用的 Docker Compose..."
    local compose_file="${DEPLOYMENT_DIR}/docker-compose.yml"

    cat > "$compose_file" << 'EOF'
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    container_name: livekit-caddy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/Caddyfile:/etc/caddy/Caddyfile:ro
      - ./volumes/caddy/data:/data
      - ./volumes/caddy/config:/config
    networks:
      - livekit
    restart: unless-stopped

networks:
  livekit:
    driver: bridge

volumes:
  caddy_data:
  caddy_config:
EOF

    log_info "启动 Caddy 容器..."
    cd "$DEPLOYMENT_DIR"
    docker-compose up -d

    sleep 5

    log_success "Caddy 部署完成！"
    log_info "访问地址: https://$DOMAIN"
    log_info ""
    log_info "下一步："
    log_info "1. 在这台机器上部署你的业务服务"
    log_info "2. 确保业务服务监听在 localhost:8080"
    log_info "3. 更新 Caddyfile 中的反向代理配置"
}

deploy_caddy_only() {
    log_info "开始部署 Caddy 反向代理..."

    check_dependencies
    init_directories
    generate_caddy_config

    log_info "生成 Caddy 专用的 Docker Compose..."
    local compose_file="${DEPLOYMENT_DIR}/docker-compose.yml"

    cat > "$compose_file" << 'EOF'
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    container_name: livekit-caddy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/Caddyfile:/etc/caddy/Caddyfile:ro
      - ./volumes/caddy/data:/data
      - ./volumes/caddy/config:/config
    networks:
      - livekit
    restart: unless-stopped

networks:
  livekit:
    driver: bridge

volumes:
  caddy_data:
  caddy_config:
EOF

    log_info "启动 Caddy 容器..."
    cd "$DEPLOYMENT_DIR"
    docker-compose up -d

    sleep 5

    log_success "Caddy 部署完成！"
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
  单机部署（2 台机器）：
    机器 1: Caddy + 业务服务
    机器 2: LiveKit + Redis

  分布式部署（4+ 台机器）：
    机器 1: Caddy + 业务服务
    机器 2, 3, 4: LiveKit + Redis

示例:
  # 单机部署 - 机器 1（Caddy + 业务服务）
  $0 deploy-caddy-service-only
  go run 你的服务.go

  # 单机部署 - 机器 2（LiveKit）
  NODES=localhost $0 deploy-livekit-only

  # 分布式部署 - 机器 1（Caddy + 业务服务）
  $0 deploy-caddy-service-only
  go run 你的服务.go

  # 分布式部署 - 机器 2, 3, 4（LiveKit）
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
        deploy-caddy-service-only)
            deploy_caddy_service_only
            ;;
        deploy-caddy-only)
            deploy_caddy_only
            ;;
        deploy-livekit-only)
            deploy_livekit_only
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

