#!/bin/bash
#
# LiveKit 集群节点 一键部署脚本
#
# 使用方式：
#   国外服务器:
#     curl -fsSL https://raw.githubusercontent.com/TgoRTC/TgoRTCServer/main/scripts/deploy-livekit-node.sh | sudo bash -s -- \
#         --master-ip <主服务器IP> --redis-password <Redis密码> \
#         --livekit-key <LiveKit API Key> --livekit-secret <LiveKit API Secret>
#   国内服务器:
#     curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy-livekit-node.sh | sudo bash -s -- \
#         --cn --master-ip <主服务器IP> --redis-password <Redis密码> \
#         --livekit-key <LiveKit API Key> --livekit-secret <LiveKit API Secret>
#   本地执行（交互模式）:
#     chmod +x deploy-livekit-node.sh && sudo ./deploy-livekit-node.sh [--cn]
#
# 功能：
#   1. 部署独立的 LiveKit 节点
#   2. 连接到主服务器的 Redis（集群同步）
#   3. 配置 Webhook 回调到 TgoRTC Server
#   4. 自动检测公网 IP
#   5. 支持中国镜像加速（--cn）
#

set -e

# ============================================================================
# 错误处理
# ============================================================================
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
    echo "  2. 端口被占用:         lsof -i :7880"
    echo "  3. Redis 连接失败:     检查主服务器防火墙是否开放 6380 端口"
    echo "  4. 镜像拉取失败:       使用 --cn 参数启用国内镜像"
    echo ""
    
    exit $exit_code
}

# ============================================================================
# 颜色定义
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ============================================================================
# 日志函数
# ============================================================================
log_info() {
    echo -e "${BLUE}[INFO]${NC}    $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC}    $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC}   $1"
}

# ============================================================================
# 配置变量
# ============================================================================
DEPLOY_DIR="${DEPLOY_DIR:-}"
USE_CN_MIRROR="${USE_CN_MIRROR:-false}"

# 主服务器配置
MASTER_IP=""
REDIS_PORT="${REDIS_PORT:-6380}"
REDIS_PASSWORD=""
LIVEKIT_API_KEY=""
LIVEKIT_API_SECRET=""
TGORTC_PORT="${TGORTC_PORT:-8080}"
# TgoRTC Server 地址（可以是 IP:端口 或 域名，用于 Webhook）
TGORTC_URL=""

# 本节点配置
NODE_IP=""

# ============================================================================
# 辅助函数
# ============================================================================

# 检测是否交互模式
is_interactive() {
    [ -t 0 ] && [ -t 1 ]
}

# 检测是否需要 sudo 执行 docker
need_docker_sudo() {
    if docker info >/dev/null 2>&1; then
        return 1  # 不需要 sudo
    else
        return 0  # 需要 sudo
    fi
}

# Docker 命令包装器
docker_cmd() {
    if need_docker_sudo; then
        sudo docker "$@"
    else
        docker "$@"
    fi
}

docker_compose_cmd() {
    if need_docker_sudo; then
        sudo docker compose "$@"
    else
        docker compose "$@"
    fi
}

# 获取公网 IP
get_public_ip() {
    local ip=""
    
    # 尝试多个服务获取公网 IP
    for service in "ifconfig.me" "ipinfo.io/ip" "api.ipify.org" "icanhazip.com"; do
        ip=$(curl -s --connect-timeout 5 "https://$service" 2>/dev/null | tr -d '[:space:]')
        if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "$ip"
            return 0
        fi
    done
    
    # 如果无法获取公网 IP，返回私网 IP
    ip=$(hostname -I 2>/dev/null | awk '{print $1}')
    if [ -n "$ip" ]; then
        echo "$ip"
        return 0
    fi
    
    echo ""
    return 1
}

# 检测操作系统
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        case "$ID" in
            ubuntu|debian|linuxmint)
                echo "debian"
                ;;
            centos|rhel|fedora|rocky|almalinux)
                echo "rhel"
                ;;
            *)
                echo "unknown"
                ;;
        esac
    elif [ "$(uname)" = "Darwin" ]; then
        echo "macos"
    else
        echo "unknown"
    fi
}

# 配置 Docker 镜像加速器
configure_docker_mirror() {
    log_info "配置 Docker 镜像加速器（国内加速）..."
    
    local daemon_json="/etc/docker/daemon.json"
    local mirrors='["https://docker.1ms.run","https://docker.xuanyuan.me"]'
    
    if [ -f "$daemon_json" ]; then
        # 备份原配置
        sudo cp "$daemon_json" "${daemon_json}.bak"
        
        # 检查是否已有 registry-mirrors
        if grep -q "registry-mirrors" "$daemon_json"; then
            log_info "Docker 镜像加速器已配置"
            return 0
        fi
        
        # 添加 registry-mirrors 到现有配置
        sudo python3 -c "
import json
with open('$daemon_json', 'r') as f:
    config = json.load(f)
config['registry-mirrors'] = $mirrors
with open('$daemon_json', 'w') as f:
    json.dump(config, f, indent=2)
" 2>/dev/null || {
            # 如果 python3 不可用，使用 jq 或直接覆盖
            if command -v jq &> /dev/null; then
                sudo jq ". + {\"registry-mirrors\": $mirrors}" "$daemon_json" > /tmp/daemon.json.tmp
                sudo mv /tmp/daemon.json.tmp "$daemon_json"
            else
                # 直接创建新配置
                sudo tee "$daemon_json" > /dev/null << EOF
{
  "registry-mirrors": ["https://docker.1ms.run", "https://docker.xuanyuan.me"]
}
EOF
            fi
        }
    else
        # 创建新配置
        sudo mkdir -p /etc/docker
        sudo tee "$daemon_json" > /dev/null << EOF
{
  "registry-mirrors": ["https://docker.1ms.run", "https://docker.xuanyuan.me"]
}
EOF
    fi
    
    # 重启 Docker
    sudo systemctl daemon-reload 2>/dev/null || true
    sudo systemctl restart docker 2>/dev/null || true
    
    log_success "Docker 镜像加速器配置完成"
}

# 安装 Docker
install_docker() {
    local os_type=$(detect_os)
    
    log_info "正在安装 Docker..."
    
    case "$os_type" in
        debian)
            if [ "$USE_CN_MIRROR" = "true" ]; then
                # 使用阿里云镜像
                curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg 2>/dev/null || true
                echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
                sudo apt-get update -qq
                sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
            else
                curl -fsSL https://get.docker.com | sudo sh
            fi
            ;;
        rhel)
            if [ "$USE_CN_MIRROR" = "true" ]; then
                sudo yum install -y yum-utils
                sudo yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
                sudo yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
            else
                curl -fsSL https://get.docker.com | sudo sh
            fi
            ;;
        *)
            log_error "不支持的操作系统，请手动安装 Docker"
            exit 1
            ;;
    esac
    
    # 启动 Docker
    log_info "启动 Docker 服务..."
    sudo systemctl enable docker 2>/dev/null || true
    sudo systemctl start docker
    
    # 配置镜像加速（国内）
    if [ "$USE_CN_MIRROR" = "true" ]; then
        configure_docker_mirror
    fi
    
    # 添加当前用户到 docker 组
    if [ -n "$SUDO_USER" ]; then
        sudo usermod -aG docker "$SUDO_USER" 2>/dev/null || true
        log_warn "已将用户 $SUDO_USER 添加到 docker 组"
        log_warn "请重新登录或执行: newgrp docker"
    fi
    
    log_success "Docker 安装完成"
}

# ============================================================================
# 参数解析
# ============================================================================
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --cn|--china)
                USE_CN_MIRROR="true"
                shift
                ;;
            --master-ip)
                MASTER_IP="$2"
                shift 2
                ;;
            --redis-port)
                REDIS_PORT="$2"
                shift 2
                ;;
            --redis-password)
                REDIS_PASSWORD="$2"
                shift 2
                ;;
            --livekit-key)
                LIVEKIT_API_KEY="$2"
                shift 2
                ;;
            --livekit-secret)
                LIVEKIT_API_SECRET="$2"
                shift 2
                ;;
            --tgortc-port)
                TGORTC_PORT="$2"
                shift 2
                ;;
            --tgortc-url|--webhook-url)
                TGORTC_URL="$2"
                shift 2
                ;;
            --node-ip)
                NODE_IP="$2"
                shift 2
                ;;
            --dir)
                DEPLOY_DIR="$2"
                shift 2
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    cat << 'EOF'
LiveKit 集群节点 一键部署脚本

使用方式:
  # 首次部署 - 使用命令行参数（推荐用于自动化）
  sudo ./deploy-livekit-node.sh \
      --master-ip <主服务器IP> \
      --redis-password <Redis密码> \
      --livekit-key <LiveKit API Key> \
      --livekit-secret <LiveKit API Secret>

  # 首次部署 - 使用交互模式
  sudo ./deploy-livekit-node.sh

  # 部署后管理
  ./deploy-livekit-node.sh <子命令>

子命令:
  info               显示本节点服务器信息（IP、端口等）
  status             查看服务状态
  logs               查看服务日志
  restart            重启服务
  stop               停止服务
  update             更新 LiveKit 镜像
  firewall           配置系统防火墙

必需参数（首次部署）:
  --master-ip        主服务器 IP 地址
  --redis-password   Redis 密码（与主服务器相同）
  --livekit-key      LiveKit API Key（与主服务器相同）
  --livekit-secret   LiveKit API Secret（与主服务器相同）

可选参数:
  --redis-port       Redis 端口（默认: 6380）
  --tgortc-port      TgoRTC Server 端口（默认: 8080）
  --tgortc-url       TgoRTC Server 完整地址（用于 Webhook）
                     例如: https://api.example.com 或 http://192.168.1.100:8080
                     如不指定，默认使用 http://<master-ip>:<tgortc-port>
  --node-ip          本节点公网 IP（默认: 自动检测）
  --dir              部署目录（默认: ~/livekit-node）
  --cn, --china      使用中国镜像加速
  -h, --help         显示此帮助信息

示例:
  # 基本部署（使用 IP）
  sudo ./deploy-livekit-node.sh \
      --master-ip 192.168.1.100 \
      --redis-password "MyRedisPass123" \
      --livekit-key "prodkey" \
      --livekit-secret "Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9"

  # 使用域名作为 Webhook 地址
  sudo ./deploy-livekit-node.sh \
      --master-ip 47.117.96.203 \
      --redis-password "TgoRedis@2025" \
      --livekit-key "prodkey" \
      --livekit-secret "Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9" \
      --tgortc-url "https://api.example.com"

  # 国外服务器一键部署（GitHub）
  curl -fsSL https://raw.githubusercontent.com/TgoRTC/TgoRTCServer/main/scripts/deploy-livekit-node.sh | sudo bash -s -- \
      --master-ip 47.117.96.203 \
      --redis-password "TgoRedis@2025" \
      --livekit-key "prodkey" \
      --livekit-secret "Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9" \
      --tgortc-url "https://api.example.com"

  # 国内服务器一键部署（Gitee + 中国镜像加速）
  curl -fsSL https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy-livekit-node.sh | sudo bash -s -- \
      --cn \
      --master-ip 47.117.96.203 \
      --redis-password "TgoRedis@2025" \
      --livekit-key "prodkey" \
      --livekit-secret "Xj9K2mP5nQ8vR1wT4yU7zA0bC3dE6fG9" \
      --tgortc-url "https://api.example.com"

  # 部署后查看节点信息
  ./deploy-livekit-node.sh info

  # 配置防火墙
  sudo ./deploy-livekit-node.sh firewall

注意:
  1. LiveKit API Key 和 Secret 必须与主服务器配置完全相同
  2. 主服务器需要开放 Redis 端口（默认 6380）给本节点访问
  3. 主服务器需要开放 TgoRTC Server 端口（默认 8080）给本节点访问
  4. 本节点需要开放以下端口:
     - 7880 (TCP): LiveKit HTTP/WebSocket
     - 7881 (TCP): LiveKit RTC TCP
     - 3478 (UDP): TURN UDP
     - 5349 (TCP): TURN TLS
     - 50000-50100 (UDP): WebRTC 媒体端口
  5. 部署完成后，需要将本节点 IP 添加到主服务器的 LIVEKIT_NODES 配置中
EOF
}

# ============================================================================
# 交互式配置
# ============================================================================
interactive_config() {
    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}               LiveKit 集群节点配置向导                          ${NC}"
    echo -e "${CYAN}════════════════════════════════════════════════════════════════${NC}"
    echo ""
    
    # 主服务器 IP
    if [ -z "$MASTER_IP" ]; then
        read -p "请输入主服务器 IP 地址: " MASTER_IP
        if [ -z "$MASTER_IP" ]; then
            log_error "主服务器 IP 不能为空"
            exit 1
        fi
    fi
    
    # Redis 密码
    if [ -z "$REDIS_PASSWORD" ]; then
        read -p "请输入 Redis 密码（与主服务器相同）: " REDIS_PASSWORD
        if [ -z "$REDIS_PASSWORD" ]; then
            log_error "Redis 密码不能为空"
            exit 1
        fi
    fi
    
    # LiveKit API Key
    if [ -z "$LIVEKIT_API_KEY" ]; then
        read -p "请输入 LiveKit API Key（与主服务器相同）: " LIVEKIT_API_KEY
        if [ -z "$LIVEKIT_API_KEY" ]; then
            log_error "LiveKit API Key 不能为空"
            exit 1
        fi
    fi
    
    # LiveKit API Secret
    if [ -z "$LIVEKIT_API_SECRET" ]; then
        read -p "请输入 LiveKit API Secret（与主服务器相同）: " LIVEKIT_API_SECRET
        if [ -z "$LIVEKIT_API_SECRET" ]; then
            log_error "LiveKit API Secret 不能为空"
            exit 1
        fi
    fi
    
    # TgoRTC Server 地址（可选）
    if [ -z "$TGORTC_URL" ]; then
        echo ""
        echo -e "${YELLOW}TgoRTC Server 地址（用于 Webhook 回调）:${NC}"
        echo -e "  如果使用域名，请输入完整地址，例如: https://api.example.com"
        echo -e "  如果使用 IP，直接回车将使用默认值: http://$MASTER_IP:$TGORTC_PORT"
        read -p "请输入 TgoRTC Server 地址（可选，直接回车跳过）: " TGORTC_URL
    fi
    
    echo ""
}

# ============================================================================
# 检查环境
# ============================================================================
check_requirements() {
    log_info "检查系统环境..."
    
    # 检查 Docker
    if ! command -v docker &> /dev/null; then
        log_warn "Docker 未安装"
        if is_interactive; then
            read -p "是否自动安装 Docker? [Y/n] " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Nn]$ ]]; then
                install_docker
            else
                log_error "请先安装 Docker"
                exit 1
            fi
        else
            install_docker
        fi
    else
        log_success "Docker 已安装: $(docker --version | head -1)"
    fi
    
    # 检查 Docker Compose
    log_info "检查 Docker Compose..."
    if docker compose version &> /dev/null; then
        log_success "Docker Compose 已安装: $(docker compose version --short)"
    else
        log_error "Docker Compose 未安装"
        exit 1
    fi
    
    # 检查 Docker 是否运行
    if ! docker_cmd info &> /dev/null; then
        log_warn "Docker 未运行，正在启动..."
        sudo systemctl start docker
    fi
    log_success "Docker 运行中"
    
    # 配置镜像加速（如果需要）
    if [ "$USE_CN_MIRROR" = "true" ]; then
        if ! grep -q "registry-mirrors" /etc/docker/daemon.json 2>/dev/null; then
            configure_docker_mirror
        else
            log_success "Docker 镜像加速器已配置"
        fi
    fi
    
    # 检查端口
    log_info "检查端口占用..."
    local ports_to_check=(7880 7881 3478 5349)
    local port_in_use=false
    
    for port in "${ports_to_check[@]}"; do
        if ss -tuln 2>/dev/null | grep -q ":$port " || netstat -tuln 2>/dev/null | grep -q ":$port "; then
            log_warn "端口 $port 已被占用"
            port_in_use=true
        fi
    done
    
    if [ "$port_in_use" = "true" ]; then
        log_warn "部分端口已被占用，可能会导致冲突"
    else
        log_success "端口检查通过"
    fi
    
    log_success "环境检查完成"
}

# ============================================================================
# 测试连接
# ============================================================================
test_connections() {
    log_info "测试与主服务器的连接..."
    
    # 测试 Redis 连接
    log_info "测试 Redis 连接 ($MASTER_IP:$REDIS_PORT)..."
    if timeout 5 bash -c "echo PING | nc -q1 $MASTER_IP $REDIS_PORT" &>/dev/null || \
       timeout 5 bash -c "</dev/tcp/$MASTER_IP/$REDIS_PORT" 2>/dev/null; then
        log_success "Redis 端口可达"
    else
        log_warn "无法连接到 Redis ($MASTER_IP:$REDIS_PORT)"
        log_warn "请确保主服务器防火墙已开放此端口"
    fi
    
    # 测试 TgoRTC Server 连接
    local tgortc_test_url=""
    if [ -n "$TGORTC_URL" ]; then
        tgortc_test_url="${TGORTC_URL%/}/health"
    else
        tgortc_test_url="http://$MASTER_IP:$TGORTC_PORT/health"
    fi
    log_info "测试 TgoRTC Server 连接 ($tgortc_test_url)..."
    if curl -s --connect-timeout 5 "$tgortc_test_url" &>/dev/null; then
        log_success "TgoRTC Server 可达"
    else
        log_warn "无法连接到 TgoRTC Server ($tgortc_test_url)"
        log_warn "请确保主服务器防火墙已开放相应端口"
    fi
}

# ============================================================================
# 生成配置
# ============================================================================
generate_configs() {
    log_info "生成配置文件..."
    
    # 检测本节点 IP
    if [ -z "$NODE_IP" ]; then
        log_info "检测本节点公网 IP..."
        NODE_IP=$(get_public_ip)
        if [ -z "$NODE_IP" ]; then
            log_error "无法检测公网 IP，请使用 --node-ip 参数指定"
            exit 1
        fi
    fi
    log_success "本节点 IP: $NODE_IP"
    
    # 构建 Webhook URL（用于 .env 和显示）
    local webhook_base_url=""
    if [ -n "$TGORTC_URL" ]; then
        webhook_base_url="${TGORTC_URL%/}"
    else
        webhook_base_url="http://$MASTER_IP:$TGORTC_PORT"
    fi
    
    # 创建 .env 文件
    cat > .env << EOF
# LiveKit 集群节点配置
# 自动生成时间: $(date '+%Y-%m-%d %H:%M:%S')

# 主服务器配置
MASTER_IP=$MASTER_IP
REDIS_PORT=$REDIS_PORT
REDIS_PASSWORD=$REDIS_PASSWORD
TGORTC_PORT=$TGORTC_PORT
TGORTC_URL=$webhook_base_url

# LiveKit 配置
LIVEKIT_API_KEY=$LIVEKIT_API_KEY
LIVEKIT_API_SECRET=$LIVEKIT_API_SECRET

# 本节点配置
NODE_IP=$NODE_IP
EOF
    log_success "创建 .env"
    
    # 创建 docker-compose.yml
    cat > docker-compose.yml << EOF
# LiveKit 集群节点
# 自动生成时间: $(date '+%Y-%m-%d %H:%M:%S')
#
# 此节点连接到主服务器的 Redis 进行集群同步
# 主服务器地址: $MASTER_IP

services:
  livekit:
    image: livekit/livekit-server:latest
    container_name: livekit-node
    restart: always
    command: --config /etc/livekit.yaml
    volumes:
      - ./livekit.yaml:/etc/livekit.yaml:ro
    ports:
      - "7880:7880"       # HTTP/WebSocket
      - "7881:7881"       # RTC TCP
      - "3478:3478/udp"   # TURN UDP
      - "5349:5349"       # TURN TLS
      - "50000-50100:50000-50100/udp"  # WebRTC 媒体端口
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:7880"]
      interval: 30s
      timeout: 10s
      retries: 3
EOF
    log_success "创建 docker-compose.yml"

    # 复用前面已计算的 webhook_base_url
    local webhook_url="${webhook_base_url}/api/v1/webhooks/livekit"
    
    # 创建 livekit.yaml
    cat > livekit.yaml << EOF
# LiveKit 集群节点配置
# 自动生成时间: $(date '+%Y-%m-%d %H:%M:%S')
#
# 主服务器: $MASTER_IP
# 本节点 IP: $NODE_IP

port: 7880

bind_addresses:
  - "0.0.0.0"

rtc:
  port_range_start: 50000
  port_range_end: 50100
  # 使用本节点的公网 IP
  node_ip: $NODE_IP
  tcp_port: 7881

turn:
  enabled: true
  # TURN 域名设置为本节点 IP（如有域名可替换）
  domain: $NODE_IP
  udp_port: 3478

# LiveKit API 密钥
# 格式: key_name: secret
# 必须与主服务器配置相同
keys:
  $LIVEKIT_API_KEY: $LIVEKIT_API_SECRET

# Redis 配置（连接主服务器的 Redis，用于集群同步）
# 这是 LiveKit 集群模式的关键配置
redis:
  address: $MASTER_IP:$REDIS_PORT
  password: $REDIS_PASSWORD
  db: 0

# Webhook 回调配置（通知主服务器的 TgoRTC 服务）
# api_key 必须与 keys 中的 key_name 相同
webhook:
  api_key: $LIVEKIT_API_KEY
  urls:
    - $webhook_url

logging:
  level: info
EOF
    log_success "创建 livekit.yaml"
    
    log_success "配置文件生成完成"
}

# ============================================================================
# 启动服务
# ============================================================================
start_services() {
    log_info "拉取 Docker 镜像..."
    docker_compose_cmd pull 2>&1 | while read line; do
        echo "  $line"
    done
    
    log_info "启动 LiveKit 服务..."
    docker_compose_cmd up -d
    
    log_success "服务启动完成"
}

# ============================================================================
# 健康检查
# ============================================================================
health_check() {
    log_info "等待服务启动..."
    
    local max_wait=60
    local waited=0
    
    while [ $waited -lt $max_wait ]; do
        if curl -s --connect-timeout 2 "http://localhost:7880" &>/dev/null; then
            log_success "LiveKit 服务已就绪"
            return 0
        fi
        
        echo -n "."
        sleep 2
        waited=$((waited + 2))
    done
    
    echo ""
    log_warn "LiveKit 启动超时，请检查日志: sudo docker compose logs"
    return 1
}

# ============================================================================
# 显示结果
# ============================================================================
show_result() {
    local node_ip="$NODE_IP"
    
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║            ✅ LiveKit 集群节点部署完成！                        ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    echo -e "${CYAN}══════════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                    ★ 本节点服务器信息 ★                          ${NC}"
    echo -e "${CYAN}══════════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${YELLOW}【重要】请将以下信息配置到主服务器！${NC}"
    echo ""
    echo -e "  ┌──────────────────────────────────────────────────────────────┐"
    echo -e "  │                                                              │"
    echo -e "  │   本节点 IP 地址:  ${GREEN}$node_ip${NC}                        "
    echo -e "  │   本节点端口:      ${GREEN}7880${NC}                                      "
    echo -e "  │                                                              │"
    echo -e "  │   LiveKit 地址:    ${BLUE}$node_ip:7880${NC}                    "
    echo -e "  │                                                              │"
    echo -e "  └──────────────────────────────────────────────────────────────┘"
    echo ""
    
    # 构建显示用的 Webhook URL
    local display_webhook_url=""
    if [ -n "$TGORTC_URL" ]; then
        display_webhook_url="${TGORTC_URL%/}/api/v1/webhooks/livekit"
    else
        display_webhook_url="http://$MASTER_IP:$TGORTC_PORT/api/v1/webhooks/livekit"
    fi
    
    echo -e "${CYAN}═══════════════════ 集群配置信息 ═══════════════════${NC}"
    echo ""
    echo -e "  主服务器 IP:          ${GREEN}$MASTER_IP${NC}"
    echo -e "  Redis 地址:           ${GREEN}$MASTER_IP:$REDIS_PORT${NC}"
    echo -e "  TgoRTC Webhook:       ${GREEN}$display_webhook_url${NC}"
    echo -e "  LiveKit API Key:      ${GREEN}$LIVEKIT_API_KEY${NC}"
    echo ""
    
    echo -e "${CYAN}═══════════════════ 访问地址 ═══════════════════${NC}"
    echo -e "  LiveKit HTTP:         ${BLUE}http://$node_ip:7880${NC}"
    echo -e "  LiveKit WebSocket:    ${BLUE}ws://$node_ip:7880${NC}"
    echo ""
    
    echo -e "${CYAN}═══════════════════ 配置文件 ═══════════════════${NC}"
    echo -e "  部署目录:             ${BLUE}$(pwd)${NC}"
    echo -e "  环境配置:             ${BLUE}$(pwd)/.env${NC}"
    echo -e "  LiveKit 配置:         ${BLUE}$(pwd)/livekit.yaml${NC}"
    echo ""
    
    echo -e "${CYAN}═══════════════════ 常用命令 ═══════════════════${NC}"
    echo -e "  查看状态:             ${YELLOW}sudo docker compose ps${NC}"
    echo -e "  查看日志:             ${YELLOW}sudo docker compose logs -f${NC}"
    echo -e "  重启服务:             ${YELLOW}sudo docker compose restart${NC}"
    echo -e "  停止服务:             ${YELLOW}sudo docker compose down${NC}"
    echo ""
    
    echo -e "${CYAN}═══════════════ 需要开放的端口 ═══════════════${NC}"
    echo ""
    echo -e "  ${YELLOW}本节点需要开放以下端口（云安全组 + 系统防火墙）:${NC}"
    echo ""
    echo -e "  ┌────────────────────────────────────────────────────────────┐"
    echo -e "  │ 端口          │ 协议   │ 用途                              │"
    echo -e "  ├────────────────────────────────────────────────────────────┤"
    echo -e "  │ 7880          │ TCP    │ LiveKit HTTP/WebSocket API        │"
    echo -e "  │ 7881          │ TCP    │ LiveKit RTC TCP                   │"
    echo -e "  │ 3478          │ UDP    │ TURN UDP                          │"
    echo -e "  │ 5349          │ TCP    │ TURN TLS                          │"
    echo -e "  │ 50000-50100   │ UDP    │ WebRTC 媒体端口                   │"
    echo -e "  └────────────────────────────────────────────────────────────┘"
    echo ""
    
    echo -e "${RED}══════════════════════════════════════════════════════════════════${NC}"
    echo -e "${RED}                    ⚠️  主服务器配置步骤                           ${NC}"
    echo -e "${RED}══════════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${YELLOW}【步骤 1】在主服务器 .env 文件中添加本节点:${NC}"
    echo ""
    echo -e "  ${BLUE}# 如果是第一个节点:${NC}"
    echo -e "  ${GREEN}LIVEKIT_NODES=$node_ip:7880${NC}"
    echo ""
    echo -e "  ${BLUE}# 如果已有其他节点，用逗号分隔:${NC}"
    echo -e "  ${GREEN}LIVEKIT_NODES=其他节点IP:7880,$node_ip:7880${NC}"
    echo ""
    echo -e "  ${YELLOW}【步骤 2】在主服务器执行重新加载 Nginx:${NC}"
    echo ""
    echo -e "  ${GREEN}cd ~/tgortc && ./deploy.sh reload-nginx${NC}"
    echo ""
    echo -e "  ${YELLOW}【步骤 3】验证集群状态:${NC}"
    echo ""
    echo -e "  ${GREEN}# 在主服务器上测试本节点是否可达${NC}"
    echo -e "  ${GREEN}curl http://$node_ip:7880${NC}"
    echo ""
    
    echo -e "${GREEN}════════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${CYAN}💡 提示: 可以执行 './deploy-livekit-node.sh firewall' 自动配置防火墙${NC}"
    echo ""
}

# ============================================================================
# 配置防火墙
# ============================================================================
cmd_firewall() {
    log_info "配置防火墙规则..."
    
    local ports=(
        "7880/tcp"
        "7881/tcp"
        "3478/udp"
        "5349/tcp"
    )
    
    # 检测防火墙类型
    if command -v ufw &> /dev/null && ufw status 2>/dev/null | grep -q "active"; then
        log_info "检测到 UFW 防火墙"
        for port in "${ports[@]}"; do
            sudo ufw allow "$port" comment "LiveKit Node" 2>/dev/null || true
        done
        # UDP 端口范围
        sudo ufw allow 50000:50100/udp comment "LiveKit WebRTC" 2>/dev/null || true
        log_success "UFW 规则已添加"
        
    elif command -v firewall-cmd &> /dev/null && systemctl is-active firewalld &> /dev/null; then
        log_info "检测到 firewalld 防火墙"
        for port in "${ports[@]}"; do
            sudo firewall-cmd --permanent --add-port="$port" 2>/dev/null || true
        done
        # UDP 端口范围
        sudo firewall-cmd --permanent --add-port=50000-50100/udp 2>/dev/null || true
        sudo firewall-cmd --reload
        log_success "firewalld 规则已添加"
        
    else
        log_warn "未检测到活动的防火墙（ufw/firewalld）"
        log_info "请手动配置防火墙或云安全组"
    fi
    
    echo ""
    log_info "请确保云服务器安全组也已开放相应端口"
}

# ============================================================================
# 下载脚本到本地
# ============================================================================
download_script_if_needed() {
    local script_path="$DEPLOY_DIR/deploy-livekit-node.sh"
    
    # 如果脚本不存在于部署目录，下载它
    if [ ! -f "$script_path" ]; then
        log_info "保存部署脚本到本地..."
        
        local script_url=""
        if [ "$USE_CN_MIRROR" = "true" ]; then
            script_url="https://gitee.com/No8blackball/tgo-rtcserver/raw/main/scripts/deploy-livekit-node.sh"
        else
            script_url="https://raw.githubusercontent.com/TgoRTC/TgoRTCServer/main/scripts/deploy-livekit-node.sh"
        fi
        
        if curl -fsSL "$script_url" -o "$script_path" 2>/dev/null; then
            chmod +x "$script_path"
            log_success "脚本已保存: $script_path"
        else
            # 如果下载失败，尝试从当前执行的脚本复制
            if [ -f "$0" ] && [ "$0" != "bash" ]; then
                cp "$0" "$script_path" 2>/dev/null || true
                chmod +x "$script_path" 2>/dev/null || true
            fi
        fi
    fi
}

# ============================================================================
# 主函数
# ============================================================================
main() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║           LiveKit 集群节点 一键部署脚本                         ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # 设置部署目录
    if [ -z "$DEPLOY_DIR" ]; then
        if [ "$(pwd)" = "$HOME" ]; then
            DEPLOY_DIR="$HOME/livekit-node"
        else
            DEPLOY_DIR="$(pwd)"
        fi
    fi
    
    mkdir -p "$DEPLOY_DIR"
    cd "$DEPLOY_DIR"
    log_info "部署目录: $DEPLOY_DIR"
    
    # 下载脚本到本地（用于后续操作）
    download_script_if_needed
    
    # 如果是交互模式且缺少必要参数，进入交互配置
    if [ -z "$MASTER_IP" ] || [ -z "$REDIS_PASSWORD" ] || [ -z "$LIVEKIT_API_KEY" ] || [ -z "$LIVEKIT_API_SECRET" ]; then
        if is_interactive; then
            interactive_config
        else
            log_error "非交互模式下必须提供所有必需参数"
            show_help
            exit 1
        fi
    fi
    
    # 验证必需参数
    if [ -z "$MASTER_IP" ]; then
        log_error "缺少必需参数: --master-ip"
        exit 1
    fi
    if [ -z "$REDIS_PASSWORD" ]; then
        log_error "缺少必需参数: --redis-password"
        exit 1
    fi
    if [ -z "$LIVEKIT_API_KEY" ]; then
        log_error "缺少必需参数: --livekit-key"
        exit 1
    fi
    if [ -z "$LIVEKIT_API_SECRET" ]; then
        log_error "缺少必需参数: --livekit-secret"
        exit 1
    fi
    
    # 构建显示用的 Webhook URL
    local display_tgortc_url=""
    if [ -n "$TGORTC_URL" ]; then
        display_tgortc_url="$TGORTC_URL"
    else
        display_tgortc_url="http://$MASTER_IP:$TGORTC_PORT"
    fi
    
    # 显示配置摘要
    echo ""
    log_info "配置摘要:"
    echo "  • 主服务器 IP:     $MASTER_IP"
    echo "  • Redis 地址:      $MASTER_IP:$REDIS_PORT"
    echo "  • TgoRTC 地址:     $display_tgortc_url"
    echo "  • LiveKit Key:     $LIVEKIT_API_KEY"
    echo ""
    
    # 执行部署步骤
    check_requirements
    test_connections
    generate_configs
    start_services
    health_check || true
    
    # 配置防火墙
    if is_interactive; then
        read -p "是否自动配置防火墙规则? [Y/n] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            cmd_firewall
        fi
    fi
    
    show_result
}

# ============================================================================
# 子命令处理
# ============================================================================
cmd_status() {
    echo ""
    echo -e "${CYAN}═══════════════════ LiveKit 节点状态 ═══════════════════${NC}"
    echo ""
    
    # 加载配置
    if [ -f .env ]; then
        source .env
        echo -e "  节点 IP:      ${GREEN}${NODE_IP:-未知}${NC}"
        echo -e "  主服务器:     ${GREEN}${MASTER_IP:-未知}${NC}"
        echo ""
    fi
    
    docker_compose_cmd ps
}

cmd_logs() {
    docker_compose_cmd logs -f
}

cmd_restart() {
    log_info "重启 LiveKit 服务..."
    docker_compose_cmd restart
    log_success "服务已重启"
}

cmd_stop() {
    log_info "停止 LiveKit 服务..."
    docker_compose_cmd down
    log_success "服务已停止"
}

cmd_update() {
    log_info "更新 LiveKit..."
    docker_compose_cmd pull
    docker_compose_cmd up -d
    log_success "更新完成"
}

cmd_info() {
    # 加载配置
    if [ -f .env ]; then
        source .env
    fi
    
    local node_ip="${NODE_IP:-$(get_public_ip)}"
    
    echo ""
    echo -e "${CYAN}══════════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                    ★ 本节点服务器信息 ★                          ${NC}"
    echo -e "${CYAN}══════════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ┌──────────────────────────────────────────────────────────────┐"
    echo -e "  │                                                              │"
    echo -e "  │   本节点 IP 地址:  ${GREEN}$node_ip${NC}                        "
    echo -e "  │   本节点端口:      ${GREEN}7880${NC}                                      "
    echo -e "  │                                                              │"
    echo -e "  │   LiveKit 地址:    ${BLUE}$node_ip:7880${NC}                    "
    echo -e "  │                                                              │"
    echo -e "  └──────────────────────────────────────────────────────────────┘"
    echo ""
    echo -e "  ${YELLOW}请将以下配置添加到主服务器 .env:${NC}"
    echo ""
    echo -e "  ${GREEN}LIVEKIT_NODES=$node_ip:7880${NC}"
    echo ""
}

# ============================================================================
# 查找部署目录
# ============================================================================
find_deploy_dir() {
    # 优先使用环境变量
    if [ -n "$DEPLOY_DIR" ] && [ -f "$DEPLOY_DIR/docker-compose.yml" ]; then
        echo "$DEPLOY_DIR"
        return 0
    fi
    
    # 检查当前目录
    if [ -f "./docker-compose.yml" ] && [ -f "./livekit.yaml" ]; then
        pwd
        return 0
    fi
    
    # 检查常见目录
    for dir in "$HOME/livekit-node" "$HOME/livekit" "/opt/livekit-node" "/opt/livekit"; do
        if [ -f "$dir/docker-compose.yml" ]; then
            echo "$dir"
            return 0
        fi
    done
    
    # 默认返回当前目录
    pwd
}

# ============================================================================
# 入口
# ============================================================================

# 首先检查是否有子命令（在 parse_args 之前处理，避免被当作未知参数）
case "${1:-}" in
    firewall)
        cd "$(find_deploy_dir)" 2>/dev/null || true
        cmd_firewall
        exit 0
        ;;
    status)
        cd "$(find_deploy_dir)" 2>/dev/null || true
        cmd_status
        exit 0
        ;;
    logs)
        cd "$(find_deploy_dir)" 2>/dev/null || true
        cmd_logs
        exit 0
        ;;
    restart)
        cd "$(find_deploy_dir)" 2>/dev/null || true
        cmd_restart
        exit 0
        ;;
    stop)
        cd "$(find_deploy_dir)" 2>/dev/null || true
        cmd_stop
        exit 0
        ;;
    update)
        cd "$(find_deploy_dir)" 2>/dev/null || true
        cmd_update
        exit 0
        ;;
    info)
        cd "$(find_deploy_dir)" 2>/dev/null || true
        cmd_info
        exit 0
        ;;
esac

# 解析参数（首次部署时使用）
parse_args "$@"

# 运行主函数（首次部署）
main
