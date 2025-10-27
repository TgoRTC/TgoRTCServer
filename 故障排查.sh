#!/bin/bash

################################################################################
# LiveKit 故障排查脚本
# 用于诊断和解决常见问题
################################################################################

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 脚本配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOYMENT_DIR="${SCRIPT_DIR}/livekit-deployment"
REPORT_FILE="${DEPLOYMENT_DIR}/troubleshoot-report-$(date +%Y%m%d-%H%M%S).txt"

################################################################################
# 显示函数
################################################################################

print_header() {
    echo -e "${BLUE}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║         LiveKit 故障排查工具                              ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

print_section() {
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
    echo "[SUCCESS] $1" >> "$REPORT_FILE"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
    echo "[ERROR] $1" >> "$REPORT_FILE"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
    echo "[WARNING] $1" >> "$REPORT_FILE"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
    echo "[INFO] $1" >> "$REPORT_FILE"
}

################################################################################
# 诊断函数
################################################################################

check_system_requirements() {
    print_section "系统要求检查"

    # 检查 OS
    print_info "操作系统: $(uname -s)"

    # 检查 CPU
    local cpu_count=$(nproc 2>/dev/null || echo "unknown")
    print_info "CPU 核心数: $cpu_count"

    # 检查内存
    local total_mem=$(free -h | grep Mem | awk '{print $2}')
    print_info "总内存: $total_mem"

    # 检查磁盘
    local disk_usage=$(df -h / | tail -1 | awk '{print $5}')
    print_info "磁盘使用率: $disk_usage"

    if [ "${disk_usage%\%}" -gt 80 ]; then
        print_warning "磁盘使用率过高"
    else
        print_success "磁盘空间充足"
    fi
}

check_docker_installation() {
    print_section "Docker 安装检查"

    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装"
        return 1
    fi

    print_success "Docker 已安装"

    local docker_version=$(docker --version)
    print_info "Docker 版本: $docker_version"

    # 检查 Docker 守护进程
    if docker ps > /dev/null 2>&1; then
        print_success "Docker 守护进程运行正常"
    else
        print_error "Docker 守护进程未运行"
        return 1
    fi

    # 检查 Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        print_error "Docker Compose 未安装"
        return 1
    fi

    print_success "Docker Compose 已安装"

    local compose_version=$(docker-compose --version)
    print_info "Docker Compose 版本: $compose_version"
}

check_network_connectivity() {
    print_section "网络连接检查"

    # 检查 DNS
    print_info "检查 DNS 解析..."
    if ping -c 1 8.8.8.8 > /dev/null 2>&1; then
        print_success "网络连接正常"
    else
        print_error "网络连接失败"
        return 1
    fi

    # 检查 DNS 解析
    if nslookup google.com > /dev/null 2>&1; then
        print_success "DNS 解析正常"
    else
        print_warning "DNS 解析可能有问题"
    fi
}

check_ports() {
    print_section "端口检查"

    local ports=(80 443 7880 7881 7882 3478 6379)

    for port in "${ports[@]}"; do
        if nc -z localhost $port 2>/dev/null; then
            print_success "端口 $port 开放"
        else
            print_warning "端口 $port 未开放"
        fi
    done
}

check_firewall() {
    print_section "防火墙检查"

    if command -v firewall-cmd &> /dev/null; then
        print_info "检测到 firewalld..."

        local active_zones=$(firewall-cmd --get-active-zones 2>/dev/null || echo "none")
        print_info "活跃区域: $active_zones"

        # 检查必要的端口
        local required_ports=(80 443 7880 3478)
        for port in "${required_ports[@]}"; do
            if firewall-cmd --query-port=$port/tcp > /dev/null 2>&1; then
                print_success "端口 $port 已开放"
            else
                print_warning "端口 $port 未开放，建议添加规则"
            fi
        done
    elif command -v ufw &> /dev/null; then
        print_info "检测到 ufw..."
        ufw status
    else
        print_info "未检测到防火墙管理工具"
    fi
}

check_containers() {
    print_section "容器状态检查"

    if [ ! -d "$DEPLOYMENT_DIR" ]; then
        print_error "部署目录不存在: $DEPLOYMENT_DIR"
        return 1
    fi

    cd "$DEPLOYMENT_DIR"

    if ! docker-compose ps > /dev/null 2>&1; then
        print_error "无法连接到 Docker Compose"
        return 1
    fi

    print_info "容器状态:"
    docker-compose ps

    # 检查容器健康状态
    local unhealthy_count=0
    while IFS= read -r line; do
        if [[ $line == *"unhealthy"* ]]; then
            print_warning "不健康的容器: $line"
            ((unhealthy_count++))
        fi
    done < <(docker-compose ps)

    if [ $unhealthy_count -eq 0 ]; then
        print_success "所有容器状态正常"
    else
        print_error "发现 $unhealthy_count 个不健康的容器"
    fi
}

check_redis_connectivity() {
    print_section "Redis 连接检查"

    cd "$DEPLOYMENT_DIR"

    if ! docker-compose exec -T redis redis-cli ping > /dev/null 2>&1; then
        print_error "Redis 连接失败"
        return 1
    fi

    print_success "Redis 连接正常"

    # 检查 Redis 内存
    local redis_memory=$(docker-compose exec -T redis redis-cli INFO memory | grep used_memory_human | cut -d':' -f2 | tr -d '\r')
    print_info "Redis 内存使用: $redis_memory"

    # 检查 Redis 键数
    local redis_keys=$(docker-compose exec -T redis redis-cli DBSIZE | grep keys | awk '{print $1}')
    print_info "Redis 键数: $redis_keys"
}

check_livekit_connectivity() {
    print_section "LiveKit 连接检查"

    if ! curl -s http://localhost:7880/ > /dev/null 2>&1; then
        print_error "LiveKit 服务未响应"
        return 1
    fi

    print_success "LiveKit 服务正常运行"

    # 检查 WebSocket
    if curl -s http://localhost:7880/ws > /dev/null 2>&1; then
        print_success "WebSocket 连接正常"
    else
        print_warning "WebSocket 连接可能有问题"
    fi
}

check_tls_certificate() {
    print_section "TLS 证书检查"

    cd "$DEPLOYMENT_DIR"

    # 检查 Caddy 日志中的证书信息
    local cert_info=$(docker-compose logs caddy | grep -i "certificate" | tail -5)

    if [ -n "$cert_info" ]; then
        print_info "证书信息:"
        echo "$cert_info"
    else
        print_warning "未找到证书信息"
    fi
}

check_disk_space() {
    print_section "磁盘空间检查"

    cd "$DEPLOYMENT_DIR"

    # 检查 Redis 数据大小
    local redis_size=$(du -sh volumes/redis 2>/dev/null || echo "unknown")
    print_info "Redis 数据大小: $redis_size"

    # 检查 Caddy 数据大小
    local caddy_size=$(du -sh volumes/caddy 2>/dev/null || echo "unknown")
    print_info "Caddy 数据大小: $caddy_size"

    # 检查总磁盘使用
    local total_size=$(du -sh . 2>/dev/null || echo "unknown")
    print_info "部署目录总大小: $total_size"
}

check_logs_for_errors() {
    print_section "日志错误检查"

    cd "$DEPLOYMENT_DIR"

    print_info "检查 LiveKit 错误日志..."
    local livekit_errors=$(docker-compose logs livekit 2>/dev/null | grep -i "error\|failed\|exception" | wc -l)
    print_info "LiveKit 错误数: $livekit_errors"

    if [ $livekit_errors -gt 0 ]; then
        print_warning "发现错误，显示最后 5 条:"
        docker-compose logs livekit 2>/dev/null | grep -i "error\|failed\|exception" | tail -5
    fi

    print_info "检查 Redis 错误日志..."
    local redis_errors=$(docker-compose logs redis 2>/dev/null | grep -i "error\|failed" | wc -l)
    print_info "Redis 错误数: $redis_errors"

    print_info "检查 Caddy 错误日志..."
    local caddy_errors=$(docker-compose logs caddy 2>/dev/null | grep -i "error" | wc -l)
    print_info "Caddy 错误数: $caddy_errors"
}

generate_diagnostic_report() {
    print_section "生成诊断报告"

    print_info "诊断报告已保存到: $REPORT_FILE"

    # 添加系统信息
    {
        echo "=========================================="
        echo "LiveKit 诊断报告"
        echo "生成时间: $(date)"
        echo "=========================================="
        echo ""
        echo "系统信息:"
        echo "操作系统: $(uname -s)"
        echo "内核版本: $(uname -r)"
        echo "CPU 核心数: $(nproc)"
        echo "总内存: $(free -h | grep Mem | awk '{print $2}')"
        echo ""
        echo "Docker 信息:"
        docker --version
        docker-compose --version
        echo ""
        echo "容器状态:"
        cd "$DEPLOYMENT_DIR"
        docker-compose ps
        echo ""
    } >> "$REPORT_FILE"

    print_success "诊断报告已生成"
}

show_recommendations() {
    print_section "建议和解决方案"

    echo ""
    echo "常见问题解决方案:"
    echo ""
    echo "1. TLS 证书获取失败"
    echo "   - 检查 DNS 是否正确指向服务器"
    echo "   - 检查防火墙是否允许 80 和 443 端口"
    echo "   - 查看 Caddy 日志: ./monitor-livekit.sh logs caddy"
    echo ""
    echo "2. Redis 连接失败"
    echo "   - 检查 Redis 容器是否运行: docker-compose ps redis"
    echo "   - 查看 Redis 日志: ./monitor-livekit.sh logs redis"
    echo "   - 检查 Redis 配置文件"
    echo ""
    echo "3. LiveKit 服务无响应"
    echo "   - 检查容器状态: ./monitor-livekit.sh docker"
    echo "   - 查看 LiveKit 日志: ./monitor-livekit.sh logs livekit"
    echo "   - 检查资源使用: ./monitor-livekit.sh resources"
    echo ""
    echo "4. 高延迟或连接问题"
    echo "   - 检查网络配置"
    echo "   - 增加 UDP 缓冲区"
    echo "   - 检查防火墙 UDP 规则"
    echo ""
    echo "5. 磁盘空间不足"
    echo "   - 清理旧日志"
    echo "   - 清理 Docker 镜像: docker system prune"
    echo "   - 扩展磁盘容量"
    echo ""
}

show_help() {
    cat << EOF
LiveKit 故障排查工具

用法: $0 [命令]

命令:
  full                执行完整诊断
  system              检查系统要求
  docker              检查 Docker 安装
  network             检查网络连接
  ports               检查端口状态
  firewall            检查防火墙
  containers          检查容器状态
  redis               检查 Redis 连接
  livekit             检查 LiveKit 连接
  tls                 检查 TLS 证书
  disk                检查磁盘空间
  logs                检查日志错误
  report              生成诊断报告
  help                显示帮助信息

示例:
  # 执行完整诊断
  $0 full

  # 检查特定组件
  $0 docker
  $0 redis
  $0 livekit

EOF
}

################################################################################
# 主函数
################################################################################

main() {
    print_header

    # 初始化报告文件
    mkdir -p "$(dirname "$REPORT_FILE")"
    touch "$REPORT_FILE"

    local command="${1:-full}"

    case "$command" in
        full)
            check_system_requirements
            check_docker_installation
            check_network_connectivity
            check_ports
            check_firewall
            check_containers
            check_redis_connectivity
            check_livekit_connectivity
            check_tls_certificate
            check_disk_space
            check_logs_for_errors
            generate_diagnostic_report
            show_recommendations
            ;;
        system)
            check_system_requirements
            ;;
        docker)
            check_docker_installation
            ;;
        network)
            check_network_connectivity
            ;;
        ports)
            check_ports
            ;;
        firewall)
            check_firewall
            ;;
        containers)
            check_containers
            ;;
        redis)
            check_redis_connectivity
            ;;
        livekit)
            check_livekit_connectivity
            ;;
        tls)
            check_tls_certificate
            ;;
        disk)
            check_disk_space
            ;;
        logs)
            check_logs_for_errors
            ;;
        report)
            generate_diagnostic_report
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知命令: $command"
            show_help
            exit 1
            ;;
    esac

    echo ""
    print_info "诊断完成，报告已保存到: $REPORT_FILE"
}

# 运行主函数
main "$@"


