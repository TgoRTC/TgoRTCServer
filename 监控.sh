#!/bin/bash

################################################################################
# LiveKit 监控和维护脚本
# 用于监控服务状态、性能和日志
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

################################################################################
# 显示函数
################################################################################

print_header() {
    echo -e "${BLUE}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║         LiveKit 监控和维护工具                            ║"
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
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

################################################################################
# 监控函数
################################################################################

check_docker_status() {
    print_section "Docker 容器状态"
    
    cd "$DEPLOYMENT_DIR"
    
    if ! docker-compose ps > /dev/null 2>&1; then
        print_error "无法连接到 Docker"
        return 1
    fi
    
    docker-compose ps
    
    echo ""
    
    # 检查容器健康状态
    local unhealthy=0
    while IFS= read -r line; do
        if [[ $line == *"unhealthy"* ]]; then
            print_warning "发现不健康的容器: $line"
            ((unhealthy++))
        fi
    done < <(docker-compose ps)
    
    if [ $unhealthy -eq 0 ]; then
        print_success "所有容器状态正常"
    else
        print_warning "发现 $unhealthy 个不健康的容器"
    fi
}

check_resource_usage() {
    print_section "资源使用情况"
    
    cd "$DEPLOYMENT_DIR"
    
    echo "容器资源使用:"
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}"
    
    echo ""
    print_info "系统资源使用:"
    
    # CPU 使用率
    local cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1)
    echo "CPU 使用率: ${cpu_usage}%"
    
    # 内存使用率
    local mem_info=$(free -h | grep Mem)
    echo "内存使用: $mem_info"
    
    # 磁盘使用率
    local disk_info=$(df -h / | tail -1)
    echo "磁盘使用: $disk_info"
}

check_network_status() {
    print_section "网络连接状态"
    
    cd "$DEPLOYMENT_DIR"
    
    # 检查 LiveKit 端口
    print_info "检查 LiveKit 端口..."
    
    local ports=(7880 7881 7882 3478)
    for port in "${ports[@]}"; do
        if nc -z localhost $port 2>/dev/null; then
            print_success "端口 $port 开放"
        else
            print_warning "端口 $port 未开放"
        fi
    done
    
    # 检查 HTTPS
    echo ""
    print_info "检查 HTTPS..."
    if curl -s https://localhost > /dev/null 2>&1; then
        print_success "HTTPS 连接正常"
    else
        print_warning "HTTPS 连接失败"
    fi
    
    # 检查 WebSocket
    echo ""
    print_info "检查 WebSocket..."
    if curl -s http://localhost:7880/ws > /dev/null 2>&1; then
        print_success "WebSocket 连接正常"
    else
        print_warning "WebSocket 连接失败"
    fi
}

check_redis_status() {
    print_section "Redis 状态"
    
    cd "$DEPLOYMENT_DIR"
    
    if ! docker-compose exec -T redis redis-cli ping > /dev/null 2>&1; then
        print_error "Redis 连接失败"
        return 1
    fi
    
    print_success "Redis 连接正常"
    
    echo ""
    print_info "Redis 信息:"
    
    # Redis 版本
    local redis_version=$(docker-compose exec -T redis redis-cli INFO server | grep redis_version | cut -d':' -f2 | tr -d '\r')
    echo "版本: $redis_version"
    
    # Redis 内存使用
    local redis_memory=$(docker-compose exec -T redis redis-cli INFO memory | grep used_memory_human | cut -d':' -f2 | tr -d '\r')
    echo "内存使用: $redis_memory"
    
    # Redis 连接数
    local redis_clients=$(docker-compose exec -T redis redis-cli INFO clients | grep connected_clients | cut -d':' -f2 | tr -d '\r')
    echo "连接数: $redis_clients"
    
    # Redis 键数
    local redis_keys=$(docker-compose exec -T redis redis-cli DBSIZE | grep keys | awk '{print $1}')
    echo "键数: $redis_keys"
}

check_livekit_status() {
    print_section "LiveKit 服务状态"
    
    cd "$DEPLOYMENT_DIR"
    
    # 检查服务是否运行
    if ! curl -s http://localhost:7880/ > /dev/null 2>&1; then
        print_error "LiveKit 服务未响应"
        return 1
    fi
    
    print_success "LiveKit 服务正常运行"
    
    echo ""
    print_info "获取服务统计信息..."
    
    # 获取统计信息
    local stats=$(curl -s http://localhost:7880/stats)
    
    if [ -n "$stats" ]; then
        echo "$stats" | jq '.' 2>/dev/null || echo "$stats"
    else
        print_warning "无法获取统计信息"
    fi
}

view_logs() {
    print_section "查看日志"
    
    cd "$DEPLOYMENT_DIR"
    
    local service="${1:-livekit}"
    local lines="${2:-50}"
    
    print_info "显示 $service 最后 $lines 行日志:"
    echo ""
    
    docker-compose logs --tail "$lines" "$service"
}

view_error_logs() {
    print_section "错误日志"
    
    cd "$DEPLOYMENT_DIR"
    
    print_info "搜索错误日志..."
    echo ""
    
    docker-compose logs | grep -i "error\|failed\|exception" | tail -20
}

show_performance_stats() {
    print_section "性能统计"
    
    cd "$DEPLOYMENT_DIR"
    
    print_info "实时性能监控 (按 Ctrl+C 退出)..."
    echo ""
    
    watch -n 1 'docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}"'
}

show_help() {
    cat << EOF
LiveKit 监控和维护工具

用法: $0 [命令]

命令:
  status              显示完整的服务状态
  docker              检查 Docker 容器状态
  resources           显示资源使用情况
  network             检查网络连接状态
  redis               检查 Redis 状态
  livekit             检查 LiveKit 服务状态
  logs [service]      查看日志 (默认: livekit)
  errors              查看错误日志
  performance         实时性能监控
  help                显示帮助信息

示例:
  # 显示完整状态
  $0 status

  # 查看 LiveKit 日志
  $0 logs livekit

  # 查看错误日志
  $0 errors

  # 实时性能监控
  $0 performance

EOF
}

################################################################################
# 主函数
################################################################################

main() {
    print_header
    
    # 检查部署目录
    if [ ! -d "$DEPLOYMENT_DIR" ]; then
        print_error "部署目录不存在: $DEPLOYMENT_DIR"
        print_info "请先运行部署脚本"
        exit 1
    fi
    
    local command="${1:-status}"
    
    case "$command" in
        status)
            check_docker_status
            echo ""
            check_resource_usage
            echo ""
            check_network_status
            echo ""
            check_redis_status
            echo ""
            check_livekit_status
            ;;
        docker)
            check_docker_status
            ;;
        resources)
            check_resource_usage
            ;;
        network)
            check_network_status
            ;;
        redis)
            check_redis_status
            ;;
        livekit)
            check_livekit_status
            ;;
        logs)
            view_logs "${2:-livekit}" "${3:-50}"
            ;;
        errors)
            view_error_logs
            ;;
        performance)
            show_performance_stats
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
}

# 运行主函数
main "$@"

