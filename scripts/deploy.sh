#!/bin/bash
#
# TgoRTC Server 一键部署脚本
#
# 使用方式：
#   curl -fsSL https://raw.githubusercontent.com/xxx/deploy.sh | bash
#   或
#   chmod +x deploy.sh && ./deploy.sh
#
# 功能：
#   1. 自动生成密码和密钥
#   2. 创建 .env、docker-compose.yml、livekit.yaml、nginx.conf
#   3. 支持 LiveKit 集群部署（Nginx 负载均衡）
#   4. 自动启动 Docker 服务
#

set -e

# ============================================================================
# 错误处理
# ============================================================================
# 捕获错误并打印详细信息
trap 'handle_error $? $LINENO "$BASH_COMMAND"' ERR

handle_error() {
    local exit_code=$1
    local line_number=$2
    local command=$3
    
    echo ""
    echo -e "\033[0;31m╔════════════════════════════════════════════════════════════════╗\033[0m"
    echo -e "\033[0;31m║                    ❌ 部署失败                                  ║\033[0m"
    echo -e "\033[0;31m╚════════════════════════════════════════════════════════════════╝\033[0m"
    echo ""
    echo -e "\033[0;31m[ERROR] 错误详情：\033[0m"
    echo "  • 退出码:   $exit_code"
    echo "  • 行号:     $line_number"
    echo "  • 命令:     $command"
    echo ""
    echo -e "\033[1;33m[提示] 常见问题排查：\033[0m"
    echo "  1. Docker 未运行:      sudo systemctl start docker"
    echo "  2. 端口被占用:         lsof -i :80 / lsof -i :8080"
    echo "  3. 镜像拉取失败:       检查网络或使用代理"
    echo "  4. 权限不足:           sudo ./deploy.sh"
    echo ""
    echo -e "\033[1;33m[调试] 查看详细日志：\033[0m"
    echo "  • sudo docker compose logs -f"
    echo "  • sudo docker compose ps"
    echo ""
    
    # 如果有部分启动的容器，提示清理
    if sudo docker compose ps -q 2>/dev/null | grep -q .; then
        echo -e "\033[1;33m[清理] 停止已启动的服务：\033[0m"
        echo "  sudo docker compose down"
    fi
    
    exit $exit_code
}

# ============================================================================
# 颜色定义
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# ============================================================================
# 配置变量
# ============================================================================
# 部署目录（默认当前目录，或 ~/tgortc）
if [ -z "$DEPLOY_DIR" ]; then
    # 如果当前目录是 home 目录，则使用 ~/tgortc
    if [ "$(pwd)" = "$HOME" ]; then
        DEPLOY_DIR="$HOME/tgortc"
        mkdir -p "$DEPLOY_DIR"
    else
        DEPLOY_DIR="$(pwd)"
    fi
fi
# Docker 镜像地址
# 默认使用阿里云公开镜像，可通过环境变量覆盖
# 示例: DOCKER_IMAGE=your-image:tag ./deploy.sh
DOCKER_IMAGE="${DOCKER_IMAGE:-crpi-4ja8peh93d2yb8c8.cn-shanghai.personal.cr.aliyuncs.com/slun/tgortc:latest}"

# LiveKit 集群节点配置（可通过环境变量覆盖）
# 格式: "ip1:port1,ip2:port2,..."
LIVEKIT_NODES="${LIVEKIT_NODES:-}"

# 服务器地址（用于客户端连接）
SERVER_HOST="${SERVER_HOST:-}"

# ============================================================================
# 参数解析（必须在最开始处理，以便后续使用）
# ============================================================================
# 中国镜像模式（通过 --cn 参数或环境变量启用）
USE_CN_MIRROR="${USE_CN_MIRROR:-false}"

# 解析命令行参数
SCRIPT_ARGS=("$@")
REMAINING_ARGS=()
for arg in "${SCRIPT_ARGS[@]}"; do
    case "$arg" in
        --cn|--china)
            USE_CN_MIRROR="true"
            ;;
        *)
            REMAINING_ARGS+=("$arg")
            ;;
    esac
done
set -- "${REMAINING_ARGS[@]}"

# 如果使用中国镜像模式，立即显示提示
if [ "$USE_CN_MIRROR" = "true" ]; then
    echo -e "\033[0;32m[CN] 使用中国镜像加速模式\033[0m"
fi

# 检测是否为交互模式（管道执行时为非交互模式）
is_interactive() {
    [ -t 0 ]
}

# ============================================================================
# Docker 命令包装器（自动处理 sudo 权限）
# ============================================================================
# 检测是否需要 sudo 运行 docker
need_docker_sudo() {
    # 如果已经是 root 用户，不需要 sudo
    if [ "$(id -u)" = "0" ]; then
        return 1
    fi
    # 检查当前用户是否在 docker 组中且可以访问 docker socket
    if docker info &>/dev/null; then
        return 1
    fi
    return 0
}

# Docker 命令包装器
docker_cmd() {
    if need_docker_sudo; then
        sudo docker "$@"
    else
        docker "$@"
    fi
}

# Docker Compose 命令包装器
docker_compose_cmd() {
    if need_docker_sudo; then
        sudo docker compose "$@"
    else
        docker compose "$@"
    fi
}

# ============================================================================
# 工具函数
# ============================================================================
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 生成随机密码 (16位，字母数字)
generate_password() {
    openssl rand -base64 16 | tr -d '/+=' | head -c 16
}

# 生成随机密钥 (32位)
generate_secret() {
    openssl rand -base64 32 | tr -d '/+='
}

# 获取服务器公网 IP
get_public_ip() {
    local ip=""
    
    # 尝试多个服务获取公网 IP
    ip=$(curl -sf --connect-timeout 3 https://ifconfig.me 2>/dev/null) || \
    ip=$(curl -sf --connect-timeout 3 https://api.ipify.org 2>/dev/null) || \
    ip=$(curl -sf --connect-timeout 3 https://icanhazip.com 2>/dev/null) || \
    ip=$(curl -sf --connect-timeout 3 http://checkip.amazonaws.com 2>/dev/null)
    
    # 清理换行符
    ip=$(echo "$ip" | tr -d '\n\r')
    
    echo "$ip"
}

# 获取服务器地址（公网IP或用户指定）
# 注意：此函数只输出 IP 地址，不输出日志（避免污染变量捕获）
get_server_host() {
    # 1. 优先使用环境变量
    if [ -n "$SERVER_HOST" ]; then
        echo "$SERVER_HOST"
        return
    fi
    
    # 2. 尝试获取公网 IP
    local public_ip=$(get_public_ip)
    
    if [ -n "$public_ip" ]; then
        echo "$public_ip"
        return
    fi
    
    # 3. 获取内网 IP 作为备选
    local private_ip=""
    if command -v hostname &> /dev/null; then
        private_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
    fi
    if [ -z "$private_ip" ]; then
        private_ip=$(ip route get 1 2>/dev/null | awk '{print $7; exit}')
    fi
    
    if [ -n "$private_ip" ]; then
        echo "$private_ip"
        return
    fi
    
    # 4. 都失败则使用占位符
    echo "YOUR_SERVER_IP"
}

# 检测操作系统类型
detect_os() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "macos"
    elif [ -f /etc/os-release ]; then
        . /etc/os-release
        case "$ID" in
            ubuntu|debian|linuxmint) echo "debian" ;;
            centos|rhel|fedora|rocky|almalinux) echo "rhel" ;;
            *) echo "unknown" ;;
        esac
    else
        echo "unknown"
    fi
}

# 配置 Docker 镜像加速器（国内服务器使用）
configure_docker_mirror() {
    log_info "配置 Docker 镜像加速器（国内加速）..."
    
    sudo mkdir -p /etc/docker
    
    # 检查是否已有配置
    if [ -f /etc/docker/daemon.json ]; then
        # 备份原配置
        sudo cp /etc/docker/daemon.json /etc/docker/daemon.json.bak
    fi
    
    # 写入镜像加速配置
    sudo tee /etc/docker/daemon.json > /dev/null <<-'EOF'
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://dockerhub.timeweb.cloud",
    "https://docker.1ms.run",
    "https://hub.rat.dev"
  ]
}
EOF
    
    # 重启 Docker 使配置生效
    sudo systemctl daemon-reload
    sudo systemctl restart docker
    
    log_success "Docker 镜像加速器配置完成"
}

# 安装 Docker
install_docker() {
    local os_type=$(detect_os)
    
    log_info "检测到操作系统: $os_type"
    log_info "开始安装 Docker..."
    
    case "$os_type" in
        debian)
            # Ubuntu/Debian 系统
            log_info "使用 apt 安装 Docker..."
            sudo apt-get update
            sudo apt-get install -y ca-certificates curl gnupg lsb-release
            
            sudo mkdir -p /etc/apt/keyrings
            
            if [ "$USE_CN_MIRROR" = "true" ]; then
                # 使用阿里云镜像安装 Docker
                log_info "使用阿里云镜像源..."
                curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
                echo \
                  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu \
                  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
                  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
            else
                # 使用 Docker 官方源
                curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
                echo \
                  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
                  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
                  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
            fi
            
            # 安装 Docker
            sudo apt-get update
            sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
            
        rhel)
            # CentOS/RHEL/Fedora 系统
            log_info "使用 yum/dnf 安装 Docker..."
            sudo yum install -y yum-utils || sudo dnf install -y dnf-plugins-core
            
            if [ "$USE_CN_MIRROR" = "true" ]; then
                # 使用阿里云镜像源
                log_info "使用阿里云镜像源..."
                sudo yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo || \
                sudo dnf config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
            else
                sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo || \
                sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            fi
            
            sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin || \
            sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
            
        macos)
            log_error "macOS 请手动安装 Docker Desktop"
            echo ""
            echo "  下载地址: https://www.docker.com/products/docker-desktop/"
            echo ""
            echo "  或使用 Homebrew:"
            echo "    brew install --cask docker"
            echo ""
            exit 1
            ;;
            
        *)
            log_error "无法识别的操作系统，请手动安装 Docker"
            echo "  安装指南: https://docs.docker.com/get-docker/"
            exit 1
            ;;
    esac
    
    # 启动 Docker 服务
    log_info "启动 Docker 服务..."
    sudo systemctl start docker
    sudo systemctl enable docker
    
    # 配置镜像加速器（国内服务器必需）
    configure_docker_mirror
    
    # 将当前用户添加到 docker 组（避免每次使用 sudo）
    if [ -n "$SUDO_USER" ]; then
        sudo usermod -aG docker "$SUDO_USER"
        log_warn "已将用户 $SUDO_USER 添加到 docker 组"
        log_warn "请重新登录或执行: newgrp docker"
    elif [ "$USER" != "root" ]; then
        sudo usermod -aG docker "$USER"
        log_warn "已将用户 $USER 添加到 docker 组"
        log_warn "请重新登录或执行: newgrp docker"
    fi
    
    log_success "Docker 安装完成"
}

# ============================================================================
# 环境检查
# ============================================================================
check_requirements() {
    log_info "检查系统环境..."
    
    # 检查 Docker
    if ! command -v docker &> /dev/null; then
        log_warn "Docker 未安装"
        echo ""
        local install_docker_confirm
        if is_interactive; then
            read -p "是否自动安装 Docker？[Y/n]: " install_docker_confirm
        else
            log_info "非交互模式，自动安装 Docker..."
            install_docker_confirm="Y"
        fi
        if [[ ! "$install_docker_confirm" =~ ^[Nn]$ ]]; then
            install_docker
        else
            log_error "Docker 未安装，无法继续部署"
            echo "  安装指南: https://docs.docker.com/get-docker/"
            exit 1
        fi
    fi
    log_success "  Docker 已安装: $(docker --version | head -1)"
    
    # 检查是否配置了镜像加速器（中国镜像模式或未配置时）
    if ! grep -q "registry-mirrors" /etc/docker/daemon.json 2>/dev/null; then
        if [ "$USE_CN_MIRROR" = "true" ]; then
            # 使用 --cn 参数时，自动配置镜像加速器
            log_info "使用中国镜像模式，自动配置镜像加速器..."
            configure_docker_mirror
        elif is_interactive; then
            log_warn "  未配置 Docker 镜像加速器"
            read -p "是否配置镜像加速器（国内服务器推荐）？[Y/n]: " config_mirror
            if [[ ! "$config_mirror" =~ ^[Nn]$ ]]; then
                configure_docker_mirror
            fi
        else
            # 非交互模式，默认配置
            log_info "非交互模式，自动配置镜像加速器..."
            configure_docker_mirror
        fi
    else
        log_success "  Docker 镜像加速器已配置"
    fi
    
    # 检查 Docker Compose（等待 Docker 完全启动）
    log_info "检查 Docker Compose..."
    local compose_retry=0
    local compose_max_retry=3
    while [ $compose_retry -lt $compose_max_retry ]; do
        # 使用 timeout 防止命令挂起
        if timeout 10 sudo docker compose version &> /dev/null 2>&1; then
            break
        fi
        compose_retry=$((compose_retry + 1))
        if [ $compose_retry -lt $compose_max_retry ]; then
            log_warn "  Docker Compose 检测失败，等待重试 ($compose_retry/$compose_max_retry)..."
            sleep 2
        fi
    done
    
    if [ $compose_retry -ge $compose_max_retry ]; then
        log_error "Docker Compose 未安装或无法正常工作"
        echo "  Docker Compose 通常随 Docker 一起安装"
        echo "  请尝试: sudo docker compose version"
        echo "  如果失败，请重启服务器后重试"
        exit 1
    fi
    
    local compose_version
    compose_version=$(timeout 10 sudo docker compose version --short 2>/dev/null || echo "unknown")
    log_success "  Docker Compose 已安装: $compose_version"
    
    # 检查 Docker 是否运行
    if ! docker info &> /dev/null; then
        log_warn "Docker 未运行，正在尝试启动..."
        if sudo systemctl start docker 2>/dev/null; then
            log_success "  Docker 已启动"
        else
            log_error "Docker 启动失败"
            echo "  请手动启动: sudo systemctl start docker"
            echo "  或检查 Docker 安装是否正确"
            exit 1
        fi
    fi
    log_success "  Docker 运行中"
    
    # 检查端口占用
    log_info "检查端口占用..."
    local ports_in_use=()
    local required_ports=(80 8080 8081 3307 6380 7880)
    
    for port in "${required_ports[@]}"; do
        if lsof -i ":$port" -sTCP:LISTEN &>/dev/null 2>&1 || \
           netstat -tuln 2>/dev/null | grep -q ":$port " || \
           ss -tuln 2>/dev/null | grep -q ":$port "; then
            ports_in_use+=($port)
        fi
    done
    
    if [ ${#ports_in_use[@]} -gt 0 ]; then
        log_warn "以下端口已被占用: ${ports_in_use[*]}"
        echo ""
        echo "  端口用途说明:"
        echo "    80   - Nginx (LiveKit 负载均衡)"
        echo "    8080 - TgoRTC API 服务"
        echo "    8081 - Adminer (数据库管理)"
        echo "    3307 - MySQL"
        echo "    6380 - Redis"
        echo "    7880 - LiveKit"
        echo ""
        echo "  查看占用进程: lsof -i :端口号"
        echo "  停止占用进程: kill -9 \$(lsof -t -i :端口号)"
        echo ""
        if is_interactive; then
            read -p "是否继续部署？（可能会失败）[y/N]: " continue_deploy
            if [[ ! "$continue_deploy" =~ ^[Yy]$ ]]; then
                log_info "部署已取消"
                exit 0
            fi
        else
            log_warn "非交互模式，忽略端口冲突继续部署..."
        fi
    else
        log_success "  端口检查通过"
    fi
    
    log_success "环境检查完成"
}

# ============================================================================
# 生成配置
# ============================================================================
generate_configs() {
    log_info "生成配置文件..."
    
    cd "$DEPLOY_DIR"
    
    # 获取服务器地址
    log_info "检测服务器公网 IP..."
    local server_host=$(get_server_host)
    if [ "$server_host" = "YOUR_SERVER_IP" ]; then
        log_warn "  无法自动检测 IP，请手动配置 SERVER_HOST"
    else
        log_success "  检测到 IP: ${server_host}"
    fi
    
    # 生成随机密码和密钥
    DB_PASSWORD=$(generate_password)
    REDIS_PASSWORD=$(generate_password)
    LIVEKIT_API_KEY="TgoRTCKey$(openssl rand -hex 4)"
    LIVEKIT_API_SECRET=$(generate_secret)
    
    log_info "  - Docker 镜像: ${DOCKER_IMAGE}"
    log_info "  - 服务器地址: ${server_host}"
    log_info "  - 数据库密码: ${DB_PASSWORD}"
    log_info "  - Redis密码: ${REDIS_PASSWORD}"
    log_info "  - LiveKit Key: ${LIVEKIT_API_KEY}"
    log_info "  - LiveKit Secret: ${LIVEKIT_API_SECRET:0:20}..."
    
    # ========== 创建 .env 文件 ==========
    cat > .env << EOF
# ============================================================================
# TgoRTC Server 配置文件
# 自动生成时间: $(date '+%Y-%m-%d %H:%M:%S')
# ============================================================================

# MySQL 配置
DB_USER=root
DB_PASSWORD=${DB_PASSWORD}
DB_NAME=tgo_rtc

# Redis 配置
REDIS_PASSWORD=${REDIS_PASSWORD}

# LiveKit 配置
LIVEKIT_API_KEY=${LIVEKIT_API_KEY}
LIVEKIT_API_SECRET=${LIVEKIT_API_SECRET}
# 客户端连接地址（通过 Nginx 负载均衡）
# 注意：这是返回给客户端的地址，必须是服务器公网IP或域名
LIVEKIT_CLIENT_URL=ws://${server_host}:80
LIVEKIT_TIMEOUT=10

# 服务器地址
SERVER_HOST=${server_host}

# 参与者超时检测间隔(秒)
PARTICIPANT_TIMEOUT_CHECK_INTERVAL=5

# Docker 镜像
DOCKER_IMAGE=${DOCKER_IMAGE}

# ============================================================================
# LiveKit 集群配置
# ============================================================================
# 外部 LiveKit 节点列表（逗号分隔）
# 格式: ip1:port1,ip2:port2
# 示例: 39.103.125.196:7880,192.168.1.100:7880
# 留空表示只使用本地节点
LIVEKIT_NODES=${LIVEKIT_NODES}

# ============================================================================
# 业务 Webhook 配置
# ============================================================================
# Webhook 端点 (可选，JSON数组格式)
# 示例: [{"url":"https://api.example.com/webhook","secret":"your-secret","timeout":10}]
BUSINESS_WEBHOOK_ENDPOINTS=
EOF
    log_success "  创建 .env"
    
    # ========== 创建 docker-compose.yml ==========
    cat > docker-compose.yml << 'EOF'
# TgoRTC Server Docker Compose 配置
# 自动生成，请勿手动修改（如需自定义请编辑 .env 文件）
#
# 包含服务：
#   - MySQL: 数据库
#   - Redis: 缓存
#   - LiveKit: 实时音视频服务
#   - Nginx: LiveKit 集群负载均衡
#   - Adminer: 数据库管理
#   - TgoRTC Server: 主应用服务

version: '3.8'

services:
  mysql:
    image: mysql:8.0
    container_name: tgo-rtc-mysql
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_PASSWORD}
      MYSQL_DATABASE: ${DB_NAME}
      TZ: Asia/Shanghai
    volumes:
      - mysql_data:/var/lib/mysql
    ports:
      - "3307:3306"
    networks:
      - tgo-rtc-network
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: tgo-rtc-redis
    restart: always
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    ports:
      - "6380:6379"
    networks:
      - tgo-rtc-network
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "${REDIS_PASSWORD}", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  livekit:
    image: livekit/livekit-server:latest
    container_name: tgo-rtc-livekit
    restart: always
    command: --config /etc/livekit.yaml
    volumes:
      - ./livekit.yaml:/etc/livekit.yaml:ro
    ports:
      - "7880:7880"
      - "7881:7881"
      - "3478:3478/udp"
      - "5349:5349"
      - "50000-50100:50000-50100/udp"
    networks:
      - tgo-rtc-network

  # Nginx 负载均衡 - LiveKit 集群入口
  nginx:
    image: nginx:alpine
    container_name: tgo-rtc-nginx
    restart: always
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/conf.d/default.conf:ro
      - nginx_logs:/var/log/nginx
    ports:
      - "80:80"
    depends_on:
      - livekit
    networks:
      - tgo-rtc-network
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  adminer:
    image: adminer
    container_name: tgo-rtc-adminer
    restart: unless-stopped
    ports:
      - "8081:8080"
    networks:
      - tgo-rtc-network
    environment:
      - ADMINER_DEFAULT_SERVER=tgo-rtc-mysql

  tgo-rtc-server:
    image: ${DOCKER_IMAGE}
    container_name: tgo-rtc-server
    restart: always
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=mysql
      - DB_PORT=3306
      - DB_USER=${DB_USER}
      - DB_PASSWORD=${DB_PASSWORD}
      - DB_NAME=${DB_NAME}
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - REDIS_DB=0
      # LiveKit 内部通信走 Nginx 负载均衡
      - LIVEKIT_URL=http://nginx:80
      - LIVEKIT_CLIENT_URL=${LIVEKIT_CLIENT_URL:-ws://localhost:80}
      - LIVEKIT_API_KEY=${LIVEKIT_API_KEY}
      - LIVEKIT_API_SECRET=${LIVEKIT_API_SECRET}
      - LIVEKIT_TIMEOUT=${LIVEKIT_TIMEOUT:-10}
      - PARTICIPANT_TIMEOUT_CHECK_INTERVAL=${PARTICIPANT_TIMEOUT_CHECK_INTERVAL:-5}
      - PORT=8080
      - BUSINESS_WEBHOOK_ENDPOINTS=${BUSINESS_WEBHOOK_ENDPOINTS}
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_healthy
      nginx:
        condition: service_started
    networks:
      - tgo-rtc-network
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  mysql_data:
  redis_data:
  nginx_logs:

networks:
  tgo-rtc-network:
    driver: bridge
EOF
    log_success "  创建 docker-compose.yml"
    
    # ========== 创建 livekit.yaml ==========
    cat > livekit.yaml << EOF
# LiveKit Server 配置
# 自动生成时间: $(date '+%Y-%m-%d %H:%M:%S')

port: 7880

rtc:
  port_range_start: 50000
  port_range_end: 50100
  # 使用固定的节点 IP（公网 IP）
  node_ip: ${server_host}
  tcp_port: 7881

turn:
  enabled: true
  # TURN 域名设置为服务器 IP（如有域名可替换）
  domain: ${server_host}
  udp_port: 3478

keys:
  ${LIVEKIT_API_KEY}: ${LIVEKIT_API_SECRET}

# Redis 配置（集群模式必需，用于房间分配）
redis:
  address: redis:6379
  password: ${REDIS_PASSWORD}
  db: 0

# Webhook 回调配置（通知 TgoRTC 服务）
webhook:
  api_key: ${LIVEKIT_API_KEY}
  urls:
    - http://tgo-rtc-server:8080/api/v1/webhooks/livekit

logging:
  level: info
EOF
    log_success "  创建 livekit.yaml"
    
    # ========== 创建 nginx 配置目录 ==========
    mkdir -p nginx
    
    # ========== 创建 nginx.conf ==========
    # 构建 upstream 节点列表
    local upstream_servers=""
    
    # 本地节点（主节点）
    upstream_servers="    server livekit:7880 max_fails=3 fail_timeout=10s; # 本地主节点"
    
    # 如果配置了外部节点
    if [ -n "$LIVEKIT_NODES" ]; then
        IFS=',' read -ra NODES <<< "$LIVEKIT_NODES"
        for node in "${NODES[@]}"; do
            node=$(echo "$node" | xargs)  # trim whitespace
            if [ -n "$node" ]; then
                upstream_servers="${upstream_servers}
    server ${node} max_fails=3 fail_timeout=10s; # 集群节点"
            fi
        done
    fi
    
    cat > nginx/nginx.conf << EOF
# TgoRTC LiveKit 集群 Nginx 配置
# 自动生成时间: $(date '+%Y-%m-%d %H:%M:%S')
#
# LiveKit 集群负载均衡配置
# 如需添加更多节点，编辑 upstream livekit_cluster 块

upstream livekit_cluster {
${upstream_servers}
    
    # IP Hash 保证同一客户端连接到同一节点
    ip_hash;
    
    # 保持长连接
    keepalive 32;
}

server {
    listen 80;
    server_name _;

    access_log /var/log/nginx/livekit-cluster-access.log;
    error_log /var/log/nginx/livekit-cluster-error.log;

    # LiveKit WebSocket 代理
    location / {
        proxy_pass http://livekit_cluster;
        proxy_http_version 1.1;

        # WebSocket 升级
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";

        # 传递原始请求信息
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

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
EOF
    log_success "  创建 nginx/nginx.conf"
    
    log_success "配置文件生成完成"
}

# ============================================================================
# 启动服务
# ============================================================================
start_services() {
    log_info "拉取 Docker 镜像（可能需要几分钟）..."
    echo "  提示: 如果长时间无响应，可以 Ctrl+C 中断后手动执行: sudo docker compose pull"
    echo ""
    # 直接输出到终端，不重定向，这样可以看到实时进度
    if ! docker_compose_cmd pull; then
        log_error "镜像拉取失败，请检查网络连接"
        echo "  尝试手动拉取: sudo docker compose pull"
        return 1
    fi
    echo ""
    
    log_info "启动服务..."
    if ! docker_compose_cmd up -d; then
        log_error "服务启动失败"
        echo "  查看容器日志: sudo docker compose logs"
        return 1
    fi
    
    log_success "服务启动中..."
}

# ============================================================================
# 等待服务就绪
# ============================================================================
wait_for_services() {
    log_info "等待服务启动..."
    sleep 5  # 给容器一些启动时间
    
    local max_attempts=30
    local attempt=0
    
    # 等待主服务就绪
    echo -n "  等待 TgoRTC API"
    while [ $attempt -lt $max_attempts ]; do
        if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
            echo " ✓"
            break
        fi
        attempt=$((attempt + 1))
        echo -n "."
        sleep 2
    done
    
    if [ $attempt -ge $max_attempts ]; then
        echo " ✗"
        log_warn "TgoRTC API 启动超时，请检查日志: sudo docker compose logs tgo-rtc-server"
    fi
}

# ============================================================================
# 健康检查
# ============================================================================
health_check() {
    echo ""
    log_info "执行服务健康检查..."
    echo ""
    
    local all_healthy=true
    local check_results=()
    
    # 1. 检查 TgoRTC API
    echo -n "  [1/6] TgoRTC API (http://localhost:8080/health) ... "
    if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 正常${NC}"
        check_results+=("API:OK")
    else
        echo -e "${RED}✗ 失败${NC}"
        check_results+=("API:FAIL")
        all_healthy=false
    fi
    
    # 2. 检查 Nginx
    echo -n "  [2/6] Nginx (http://localhost:80/health) ... "
    if curl -sf http://localhost:80/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 正常${NC}"
        check_results+=("Nginx:OK")
    else
        echo -e "${RED}✗ 失败${NC}"
        check_results+=("Nginx:FAIL")
        all_healthy=false
    fi
    
    # 3. 检查 LiveKit
    echo -n "  [3/6] LiveKit (http://localhost:7880) ... "
    if curl -sf http://localhost:7880 > /dev/null 2>&1 || \
       curl -sf -o /dev/null -w "%{http_code}" http://localhost:7880 2>/dev/null | grep -q "4.."; then
        echo -e "${GREEN}✓ 正常${NC}"
        check_results+=("LiveKit:OK")
    else
        echo -e "${RED}✗ 失败${NC}"
        check_results+=("LiveKit:FAIL")
        all_healthy=false
    fi
    
    # 4. 检查 MySQL
    echo -n "  [4/6] MySQL (localhost:3307) ... "
    if docker_cmd exec tgo-rtc-mysql mysqladmin ping -h localhost -u root -p"$DB_PASSWORD" --silent 2>/dev/null; then
        echo -e "${GREEN}✓ 正常${NC}"
        check_results+=("MySQL:OK")
    else
        # 备用检查方式
        if docker_compose_cmd ps mysql 2>/dev/null | grep -q "healthy\|running"; then
            echo -e "${GREEN}✓ 正常${NC}"
            check_results+=("MySQL:OK")
        else
            echo -e "${RED}✗ 失败${NC}"
            check_results+=("MySQL:FAIL")
            all_healthy=false
        fi
    fi
    
    # 5. 检查 Redis
    echo -n "  [5/6] Redis (localhost:6380) ... "
    if docker_cmd exec tgo-rtc-redis redis-cli -a "$REDIS_PASSWORD" ping 2>/dev/null | grep -q "PONG"; then
        echo -e "${GREEN}✓ 正常${NC}"
        check_results+=("Redis:OK")
    else
        # 备用检查方式
        if docker_compose_cmd ps redis 2>/dev/null | grep -q "healthy\|running"; then
            echo -e "${GREEN}✓ 正常${NC}"
            check_results+=("Redis:OK")
        else
            echo -e "${RED}✗ 失败${NC}"
            check_results+=("Redis:FAIL")
            all_healthy=false
        fi
    fi
    
    # 6. 检查 Adminer
    echo -n "  [6/6] Adminer (http://localhost:8081) ... "
    if curl -sf http://localhost:8081 > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 正常${NC}"
        check_results+=("Adminer:OK")
    else
        echo -e "${YELLOW}⚠ 可选服务${NC}"
        check_results+=("Adminer:OPTIONAL")
    fi
    
    echo ""
    
    # 显示容器状态
    log_info "容器状态:"
    docker_compose_cmd ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || docker_compose_cmd ps
    
    echo ""
    
    # 汇总结果
    if $all_healthy; then
        echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║              ✅ 所有服务健康检查通过                           ║${NC}"
        echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
        return 0
    else
        echo -e "${YELLOW}╔════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${YELLOW}║              ⚠️  部分服务检查未通过                             ║${NC}"
        echo -e "${YELLOW}╚════════════════════════════════════════════════════════════════╝${NC}"
        echo ""
        echo "排查建议："
        for result in "${check_results[@]}"; do
            if [[ "$result" == *":FAIL" ]]; then
                service_name="${result%%:*}"
                case "$service_name" in
                    API)
                        echo "  • TgoRTC API: sudo docker compose logs tgo-rtc-server"
                        ;;
                    Nginx)
                        echo "  • Nginx: sudo docker compose logs nginx"
                        ;;
                    LiveKit)
                        echo "  • LiveKit: sudo docker compose logs livekit"
                        ;;
                    MySQL)
                        echo "  • MySQL: sudo docker compose logs mysql"
                        ;;
                    Redis)
                        echo "  • Redis: sudo docker compose logs redis"
                        ;;
                esac
            fi
        done
        echo ""
        return 1
    fi
}

# ============================================================================
# 显示结果
# ============================================================================
show_result() {
    # 从 .env 读取服务器地址
    local display_host="${SERVER_HOST:-localhost}"
    if [ -f .env ]; then
        source .env 2>/dev/null
        display_host="${SERVER_HOST:-localhost}"
    fi
    
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              🎉 TgoRTC Server 部署完成！                       ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BLUE}服务器地址: ${display_host}${NC}"
    echo ""
    echo -e "${BLUE}服务地址（外网访问）：${NC}"
    echo "  • API 服务:        http://${display_host}:8080"
    echo "  • Swagger 文档:    http://${display_host}:8080/swagger/index.html"
    echo "  • 数据库管理:      http://${display_host}:8081"
    echo "  • LiveKit (Nginx): ws://${display_host}:80  (集群负载均衡入口)"
    echo "  • LiveKit (直连):  ws://${display_host}:7880 (本地节点)"
    echo ""
    echo -e "${BLUE}服务地址（本地验证）：${NC}"
    echo "  • API 健康检查:    curl http://localhost:8080/health"
    echo "  • Nginx 健康检查:  curl http://localhost:80/health"
    echo ""
    echo -e "${BLUE}配置文件：${NC}"
    echo "  • .env               - 环境变量配置"
    echo "  • docker-compose.yml - Docker 编排配置"
    echo "  • livekit.yaml       - LiveKit 服务配置"
    echo "  • nginx/nginx.conf   - Nginx 负载均衡配置"
    echo ""
    echo -e "${BLUE}常用命令：${NC}"
    echo "  查看日志:   sudo docker compose logs -f"
    echo "  停止服务:   sudo docker compose down"
    echo "  重启服务:   sudo docker compose restart"
    echo "  查看状态:   sudo docker compose ps"
    echo "  健康检查:   ./deploy.sh check"
    echo ""
    echo -e "${YELLOW}⚠️  重要提示：${NC}"
    echo "  1. 密码已保存在 .env 文件中，请妥善保管"
    if [[ "$display_host" == "YOUR_SERVER_IP" ]]; then
        echo -e "  2. ${RED}请修改 .env 中的 SERVER_HOST 和 LIVEKIT_CLIENT_URL 为实际服务器地址${NC}"
    fi
    echo "  3. 如需配置 Webhook，请编辑 .env 中的 BUSINESS_WEBHOOK_ENDPOINTS"
    echo ""
    echo -e "${BLUE}📡 LiveKit 集群配置：${NC}"
    if [ -n "$LIVEKIT_NODES" ]; then
        echo "  已配置的集群节点："
        echo "    - livekit:7880 (本地主节点)"
        IFS=',' read -ra NODES <<< "$LIVEKIT_NODES"
        for node in "${NODES[@]}"; do
            node=$(echo "$node" | xargs)
            [ -n "$node" ] && echo "    - ${node} (外部节点)"
        done
    else
        echo "  当前为单节点模式"
        echo "  添加集群节点: 编辑 .env 中的 LIVEKIT_NODES"
        echo "  示例: LIVEKIT_NODES=39.103.125.196:7880,192.168.1.100:7880"
    fi
    echo ""
    
    echo -e "${YELLOW}🔓 需要开放的端口（请在云服务器安全组中配置）：${NC}"
    echo ""
    echo "  ┌────────────────────────────────────────────────────────────┐"
    echo "  │  端口          协议      服务              说明            │"
    echo "  ├────────────────────────────────────────────────────────────┤"
    echo "  │  80            TCP       Nginx             LiveKit入口     │"
    echo "  │  8080          TCP       TgoRTC API        API服务         │"
    echo "  │  8081          TCP       Adminer           数据库管理(可选)│"
    echo "  │  7880          TCP       LiveKit           信令服务        │"
    echo "  │  7881          TCP       LiveKit           WebRTC TCP      │"
    echo "  │  3478          UDP       LiveKit TURN      NAT穿透         │"
    echo "  │  5349          TCP       LiveKit TURN      TLS穿透         │"
    echo "  │  50000-50100   UDP       LiveKit RTC       媒体流端口      │"
    echo "  └────────────────────────────────────────────────────────────┘"
    echo ""
    echo "  必须开放: 80, 8080, 7880, 7881, 3478/UDP, 50000-50100/UDP"
    echo "  可选开放: 8081(数据库管理), 5349(TLS穿透)"
    echo ""
    echo -e "  ${YELLOW}提示: 运行 './deploy.sh firewall' 可自动配置服务器防火墙${NC}"
    echo -e "  ${YELLOW}      云安全组需要在云控制台手动配置${NC}"
    echo ""
}

# ============================================================================
# 主函数
# ============================================================================
main() {
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              TgoRTC Server 一键部署脚本                        ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # 进入部署目录
    log_info "部署目录: $DEPLOY_DIR"
    mkdir -p "$DEPLOY_DIR"
    cd "$DEPLOY_DIR"
    
    # 检查是否已有配置
    if [ -f "$DEPLOY_DIR/.env" ] && [ -f "$DEPLOY_DIR/docker-compose.yml" ]; then
        log_warn "检测到已有配置文件"
        local confirm=""
        if is_interactive; then
            read -p "是否覆盖现有配置？[y/N]: " confirm
        else
            # 非交互模式，默认使用现有配置启动
            log_info "非交互模式，使用现有配置启动服务..."
            confirm="n"
        fi
        if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
            log_info "使用现有配置启动服务..."
            cd "$DEPLOY_DIR"
            docker_compose_cmd up -d
            wait_for_services
            show_result
            exit 0
        fi
        # 备份旧配置
        backup_dir=".backup_$(date +%Y%m%d_%H%M%S)"
        mkdir -p "$backup_dir"
        mv .env docker-compose.yml "$backup_dir/" 2>/dev/null || true
        [ -f livekit.yaml ] && mv livekit.yaml "$backup_dir/"
        [ -d nginx ] && mv nginx "$backup_dir/"
        log_info "旧配置已备份到 $backup_dir"
    fi
    
    check_requirements
    generate_configs
    start_services
    wait_for_services
    
    # 从 .env 读取密码用于健康检查
    source .env 2>/dev/null || true
    
    health_check
    show_result
}

# ============================================================================
# 命令行参数处理
# ============================================================================
show_help() {
    echo ""
    echo "TgoRTC Server 一键部署脚本"
    echo ""
    echo "用法: $0 [--cn] [命令]"
    echo ""
    echo "通用参数:"
    echo "  --cn       使用中国镜像加速（Docker 安装源、镜像加速器等）"
    echo ""
    echo "部署命令:"
    echo "  deploy     首次部署服务（默认）"
    echo "  update     升级更新服务"
    echo "  rollback   回滚到上一版本"
    echo "  version    查看版本信息"
    echo ""
    echo "运维命令:"
    echo "  check      执行健康检查"
    echo "  status     查看服务状态"
    echo "  logs       查看服务日志"
    echo "  restart    重启所有服务"
    echo "  reload     重载 Nginx 配置（更新集群节点后使用）"
    echo "  firewall   配置服务器防火墙（自动开放端口）"
    echo "  stop       停止所有服务"
    echo "  clean      停止并清理所有数据（危险）"
    echo ""
    echo "其他命令:"
    echo "  help       显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0                    # 首次部署（国际网络）"
    echo "  $0 --cn               # 首次部署（中国镜像加速）"
    echo "  $0 --cn update        # 升级更新（中国镜像加速）"
    echo "  $0 check              # 健康检查"
    echo "  $0 logs               # 查看日志"
    echo "  $0 rollback           # 回滚到上一版本"
    echo ""
    echo "环境变量:"
    echo "  DOCKER_IMAGE    自定义镜像地址（可选，有默认值）"
    echo "  SERVER_HOST     服务器公网IP或域名（可选，自动检测）"
    echo "  LIVEKIT_NODES   LiveKit 集群节点，逗号分隔（可选）"
    echo "  USE_CN_MIRROR   设为 true 启用中国镜像（等效于 --cn 参数）"
    echo ""
    echo "默认镜像: crpi-4ja8peh93d2yb8c8.cn-shanghai.personal.cr.aliyuncs.com/slun/tgortc:latest"
    echo ""
    echo "示例:"
    echo "  # 快速部署（使用默认镜像，中国服务器推荐使用 --cn）"
    echo "  $0 --cn"
    echo ""
    echo "  # 使用自定义镜像"
    echo "  DOCKER_IMAGE=your-registry/image:tag $0"
    echo ""
    echo "  # 集群部署"
    echo "  LIVEKIT_NODES=192.168.1.100:7880,192.168.1.101:7880 $0"
    echo ""
}

cmd_check() {
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              TgoRTC Server 健康检查                            ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    
    cd "$DEPLOY_DIR"
    
    # 加载配置
    if [ -f .env ]; then
        source .env
    else
        log_warn "未找到 .env 文件，部分检查可能失败"
    fi
    
    health_check
}

cmd_status() {
    echo ""
    log_info "服务状态:"
    echo ""
    docker_compose_cmd ps
}

cmd_logs() {
    docker_compose_cmd logs -f
}

cmd_stop() {
    log_info "停止服务..."
    docker_compose_cmd down
    log_success "服务已停止"
}

cmd_restart() {
    log_info "重启服务..."
    docker_compose_cmd restart
    log_success "服务已重启"
    
    sleep 5
    source .env 2>/dev/null || true
    health_check
}

# 重新生成 Nginx 配置（用于更新集群节点）
cmd_reload_nginx() {
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              重新加载 Nginx 配置                               ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    cd "$DEPLOY_DIR"
    
    # 检查配置文件
    if [ ! -f .env ]; then
        log_error "未找到 .env 文件"
        exit 1
    fi
    
    # 加载配置
    source .env
    
    log_info "当前 LIVEKIT_NODES 配置: ${LIVEKIT_NODES:-（空，仅本地节点）}"
    
    # 重新生成 nginx.conf
    log_info "重新生成 nginx/nginx.conf..."
    
    mkdir -p nginx
    
    # 构建 upstream 节点列表
    local upstream_servers=""
    upstream_servers="    server livekit:7880 max_fails=3 fail_timeout=10s; # 本地主节点"
    
    if [ -n "$LIVEKIT_NODES" ]; then
        IFS=',' read -ra NODES <<< "$LIVEKIT_NODES"
        for node in "${NODES[@]}"; do
            node=$(echo "$node" | xargs)
            if [ -n "$node" ]; then
                upstream_servers="${upstream_servers}
    server ${node} max_fails=3 fail_timeout=10s; # 集群节点"
            fi
        done
    fi
    
    cat > nginx/nginx.conf << EOF
# TgoRTC LiveKit 集群 Nginx 配置
# 重新生成时间: $(date '+%Y-%m-%d %H:%M:%S')

upstream livekit_cluster {
${upstream_servers}
    
    ip_hash;
    keepalive 32;
}

server {
    listen 80;
    server_name _;

    access_log /var/log/nginx/livekit-cluster-access.log;
    error_log /var/log/nginx/livekit-cluster-error.log;

    location / {
        proxy_pass http://livekit_cluster;
        proxy_http_version 1.1;

        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        proxy_connect_timeout 5s;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;

        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503;
        proxy_next_upstream_timeout 10s;
        proxy_next_upstream_tries 2;

        proxy_buffering off;
    }

    location /health {
        access_log off;
        return 200 'OK';
        add_header Content-Type text/plain;
    }
}
EOF
    
    log_success "nginx.conf 已更新"
    
    # 显示当前配置的节点
    echo ""
    log_info "当前集群节点配置："
    echo "    - livekit:7880 (本地主节点)"
    if [ -n "$LIVEKIT_NODES" ]; then
        IFS=',' read -ra NODES <<< "$LIVEKIT_NODES"
        for node in "${NODES[@]}"; do
            node=$(echo "$node" | xargs)
            [ -n "$node" ] && echo "    - ${node} (外部节点)"
        done
    fi
    
    # 重启 Nginx
    echo ""
    log_info "重启 Nginx 服务..."
    docker_compose_cmd restart nginx
    
    sleep 3
    
    # 验证 Nginx 状态
    if curl -sf http://localhost:80/health > /dev/null 2>&1; then
        log_success "Nginx 重启成功"
    else
        log_warn "Nginx 可能未完全启动，请检查: sudo docker compose logs nginx"
    fi
    
    echo ""
}

cmd_clean() {
    log_warn "此操作将停止服务并删除所有数据（数据库、Redis缓存等）"
    read -p "确定要继续吗？[y/N]: " confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        log_info "停止并清理服务..."
        docker_compose_cmd down -v
        log_success "服务已停止，数据已清理"
    else
        log_info "操作已取消"
    fi
}

# 配置服务器防火墙
cmd_firewall() {
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              配置服务器防火墙                                  ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # 检测防火墙类型
    local firewall_type=""
    if command -v ufw &> /dev/null; then
        firewall_type="ufw"
    elif command -v firewall-cmd &> /dev/null; then
        firewall_type="firewalld"
    else
        log_warn "未检测到 ufw 或 firewalld，跳过防火墙配置"
        echo ""
        echo "请手动开放以下端口："
        echo "  TCP: 80, 8080, 7880, 7881, 5349, 8081"
        echo "  UDP: 3478, 50000-50100"
        return 0
    fi
    
    log_info "检测到防火墙: $firewall_type"
    echo ""
    echo "将开放以下端口："
    echo "  TCP: 80, 8080, 7880, 7881, 5349, 8081"
    echo "  UDP: 3478, 50000-50100"
    echo ""
    read -p "是否继续配置？[Y/n]: " confirm
    if [[ "$confirm" =~ ^[Nn]$ ]]; then
        log_info "配置已取消"
        return 0
    fi
    
    if [ "$firewall_type" = "ufw" ]; then
        log_info "配置 UFW 防火墙..."
        
        # TCP 端口
        sudo ufw allow 80/tcp comment 'TgoRTC Nginx'
        sudo ufw allow 8080/tcp comment 'TgoRTC API'
        sudo ufw allow 8081/tcp comment 'TgoRTC Adminer'
        sudo ufw allow 7880/tcp comment 'LiveKit Signal'
        sudo ufw allow 7881/tcp comment 'LiveKit WebRTC TCP'
        sudo ufw allow 5349/tcp comment 'LiveKit TURN TLS'
        
        # UDP 端口
        sudo ufw allow 3478/udp comment 'LiveKit TURN'
        sudo ufw allow 50000:50100/udp comment 'LiveKit RTC Media'
        
        # 启用防火墙
        sudo ufw --force enable
        
        log_success "UFW 防火墙配置完成"
        echo ""
        sudo ufw status numbered
        
    elif [ "$firewall_type" = "firewalld" ]; then
        log_info "配置 Firewalld 防火墙..."
        
        # TCP 端口
        sudo firewall-cmd --permanent --add-port=80/tcp
        sudo firewall-cmd --permanent --add-port=8080/tcp
        sudo firewall-cmd --permanent --add-port=8081/tcp
        sudo firewall-cmd --permanent --add-port=7880/tcp
        sudo firewall-cmd --permanent --add-port=7881/tcp
        sudo firewall-cmd --permanent --add-port=5349/tcp
        
        # UDP 端口
        sudo firewall-cmd --permanent --add-port=3478/udp
        sudo firewall-cmd --permanent --add-port=50000-50100/udp
        
        # 重载配置
        sudo firewall-cmd --reload
        
        log_success "Firewalld 防火墙配置完成"
        echo ""
        sudo firewall-cmd --list-ports
    fi
    
    echo ""
    echo -e "${YELLOW}⚠️  注意：云服务器安全组需要在云控制台单独配置！${NC}"
    echo ""
    echo "  腾讯云: https://console.cloud.tencent.com/cvm/securitygroup"
    echo "  阿里云: https://ecs.console.aliyun.com/ → 安全组"
    echo "  华为云: https://console.huaweicloud.com/ → 安全组"
    echo ""
}

# ============================================================================
# 升级更新命令
# ============================================================================
cmd_update() {
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              TgoRTC Server 升级更新                            ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    cd "$DEPLOY_DIR"
    
    # 检查配置文件
    if [ ! -f .env ] || [ ! -f docker-compose.yml ]; then
        log_error "未找到配置文件，请先执行部署: ./deploy.sh deploy"
        exit 1
    fi
    
    # 加载配置
    source .env 2>/dev/null || true
    
    log_info "当前镜像: ${DOCKER_IMAGE:-默认镜像}"
    echo ""
    
    # 选择更新类型
    echo "请选择更新类型："
    echo "  1) 快速更新 - 仅更新 TgoRTC Server 镜像（推荐）"
    echo "  2) 完整更新 - 更新所有服务镜像"
    echo "  3) 指定版本 - 更新到指定版本"
    echo "  4) 取消"
    echo ""
    read -p "请选择 [1-4]: " update_choice
    
    case "$update_choice" in
        1)
            update_tgortc_only
            ;;
        2)
            update_all_services
            ;;
        3)
            update_specific_version
            ;;
        *)
            log_info "更新已取消"
            exit 0
            ;;
    esac
}

# 仅更新 TgoRTC Server
update_tgortc_only() {
    log_info "开始快速更新 TgoRTC Server..."
    echo ""
    
    # 记录当前版本
    local current_image=$(docker_cmd inspect tgo-rtc-server --format='{{.Config.Image}}' 2>/dev/null || echo "unknown")
    log_info "当前版本: $current_image"
    
    # 拉取最新镜像
    log_info "拉取最新镜像..."
    if ! docker_compose_cmd pull tgo-rtc-server 2>&1 | tee /tmp/tgo-update.log; then
        log_error "镜像拉取失败"
        echo "  查看日志: cat /tmp/tgo-update.log"
        return 1
    fi
    
    # 备份当前容器日志
    log_info "备份当前日志..."
    docker_compose_cmd logs tgo-rtc-server > "/tmp/tgo-rtc-server-$(date +%Y%m%d_%H%M%S).log" 2>/dev/null || true
    
    # 重启服务（使用新镜像）
    log_info "重启服务..."
    docker_compose_cmd up -d tgo-rtc-server
    
    # 等待服务就绪
    log_info "等待服务就绪..."
    sleep 10
    
    # 健康检查
    health_check
    
    # 显示更新结果
    local new_image=$(docker_cmd inspect tgo-rtc-server --format='{{.Config.Image}}' 2>/dev/null || echo "unknown")
    echo ""
    log_success "更新完成！"
    echo "  • 更新前: $current_image"
    echo "  • 更新后: $new_image"
    echo ""
}

# 更新所有服务
update_all_services() {
    log_info "开始完整更新所有服务..."
    echo ""
    
    log_warn "此操作将更新所有服务镜像，可能需要较长时间"
    read -p "确定继续吗？[y/N]: " confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        log_info "更新已取消"
        return 0
    fi
    
    # 拉取所有镜像
    log_info "拉取所有镜像..."
    docker_compose_cmd pull
    
    # 备份日志
    log_info "备份当前日志..."
    docker_compose_cmd logs > "/tmp/tgo-all-services-$(date +%Y%m%d_%H%M%S).log" 2>/dev/null || true
    
    # 重启所有服务
    log_info "重启所有服务..."
    docker_compose_cmd up -d
    
    # 等待服务就绪
    log_info "等待服务就绪..."
    sleep 15
    
    # 健康检查
    health_check
    
    log_success "所有服务更新完成！"
}

# 更新到指定版本
update_specific_version() {
    echo ""
    read -p "请输入镜像版本标签 (例: v1.2.0, latest): " version_tag
    
    if [ -z "$version_tag" ]; then
        log_error "版本标签不能为空"
        return 1
    fi
    
    # 从 .env 读取镜像仓库地址
    source .env 2>/dev/null || true
    local base_image="${DOCKER_IMAGE%:*}"  # 去掉原有 tag
    local new_image="${base_image}:${version_tag}"
    
    log_info "将更新到: $new_image"
    read -p "确定继续吗？[y/N]: " confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        log_info "更新已取消"
        return 0
    fi
    
    # 更新 .env 中的镜像版本
    log_info "更新配置文件..."
    sed -i.bak "s|^DOCKER_IMAGE=.*|DOCKER_IMAGE=${new_image}|" .env
    
    # 拉取指定版本
    log_info "拉取镜像: $new_image"
    if ! docker_cmd pull "$new_image"; then
        log_error "镜像拉取失败: $new_image"
        # 回滚配置
        mv .env.bak .env
        return 1
    fi
    rm -f .env.bak
    
    # 重启服务
    log_info "重启服务..."
    docker_compose_cmd up -d tgo-rtc-server
    
    # 等待服务就绪
    log_info "等待服务就绪..."
    sleep 10
    
    # 健康检查
    health_check
    
    log_success "已更新到版本: $version_tag"
}

# 回滚到上一版本
cmd_rollback() {
    echo ""
    echo -e "${YELLOW}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${YELLOW}║              TgoRTC Server 版本回滚                            ║${NC}"
    echo -e "${YELLOW}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    cd "$DEPLOY_DIR"
    
    # 检查是否有备份的 .env
    if [ -f .env.bak ]; then
        log_info "发现配置备份，可以回滚到上一版本"
        local old_image=$(grep "^DOCKER_IMAGE=" .env.bak | cut -d= -f2)
        log_info "上一版本: $old_image"
        
        read -p "是否回滚到此版本？[y/N]: " confirm
        if [[ "$confirm" =~ ^[Yy]$ ]]; then
            mv .env.bak .env
            source .env
            docker_compose_cmd up -d tgo-rtc-server
            log_success "已回滚到: $old_image"
            sleep 10
            health_check
        fi
    else
        log_warn "未找到配置备份"
        echo ""
        echo "手动回滚方法："
        echo "  1. 编辑 .env 文件，修改 DOCKER_IMAGE 为旧版本"
        echo "  2. 执行: sudo docker compose up -d tgo-rtc-server"
        echo ""
        echo "查看可用镜像版本："
        echo "  docker images | grep tgortc"
    fi
}

# 查看版本信息
cmd_version() {
    echo ""
    log_info "版本信息："
    echo ""
    
    cd "$DEPLOY_DIR"
    
    # 配置的镜像版本
    if [ -f .env ]; then
        source .env
        echo "  配置镜像:  ${DOCKER_IMAGE:-未配置}"
    fi
    
    # 运行中的镜像版本
    local running_image=$(docker_cmd inspect tgo-rtc-server --format='{{.Config.Image}}' 2>/dev/null)
    if [ -n "$running_image" ]; then
        echo "  运行镜像:  $running_image"
        
        # 容器创建时间
        local created=$(docker_cmd inspect tgo-rtc-server --format='{{.Created}}' 2>/dev/null)
        echo "  创建时间:  $created"
    else
        echo "  运行镜像:  未运行"
    fi
    
    # 本地可用镜像
    echo ""
    echo "  本地可用镜像："
    docker_cmd images --format "    {{.Repository}}:{{.Tag}} ({{.Size}}, {{.CreatedSince}})" | grep -i tgortc || echo "    无"
    echo ""
}

# ============================================================================
# 命令分发
# ============================================================================
# 根据参数执行不同命令
case "${1:-deploy}" in
    deploy|"")
        main
        ;;
    update|upgrade)
        cmd_update
        ;;
    rollback)
        cmd_rollback
        ;;
    version|ver|-v)
        cmd_version
        ;;
    check|health)
        cmd_check
        ;;
    status|ps)
        cmd_status
        ;;
    logs)
        cmd_logs
        ;;
    stop|down)
        cmd_stop
        ;;
    restart)
        cmd_restart
        ;;
    reload|reload-nginx)
        cmd_reload_nginx
        ;;
    firewall|fw)
        cmd_firewall
        ;;
    clean|purge)
        cmd_clean
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        log_error "未知命令: $1"
        show_help
        exit 1
        ;;
esac
