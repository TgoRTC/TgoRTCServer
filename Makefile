.PHONY: help build run stop logs clean deploy deploy-prod init-https backup restore verify

# 颜色定义
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m

# 默认目标
.DEFAULT_GOAL := help

# ============================================================================
# 帮助
# ============================================================================

help: ## 显示帮助信息
	@echo "$(BLUE)TgoRTC Server - Docker Compose 部署工具$(NC)"
	@echo ""
	@echo "$(YELLOW)开发环境命令:$(NC)"
	@echo "  make build              构建应用"
	@echo "  make run                运行应用"
	@echo "  make stop               停止应用"
	@echo "  make logs               查看日志"
	@echo "  make clean              清理构建文件"
	@echo ""
	@echo "$(YELLOW)部署命令:$(NC)"
	@echo "  make deploy             部署开发环境（Docker Compose）"
	@echo "  make deploy-prod        部署生产环境（包含业务服务）"
	@echo "  make init-https         初始化 HTTPS 证书"
	@echo ""
	@echo "$(YELLOW)数据管理:$(NC)"
	@echo "  make backup             备份数据"
	@echo "  make restore            恢复数据"
	@echo ""
	@echo "$(YELLOW)维护命令:$(NC)"
	@echo "  make verify             验证部署"
	@echo "  make restart            重启服务"
	@echo "  make ps                 查看容器状态"
	@echo "  make e2e-local          本地端到端测试（自动启动/验证接口）"
	@echo ""

# ============================================================================
# 开发环境
# ============================================================================

build: ## 构建应用
	@echo "$(BLUE)构建应用...$(NC)"
	go build -o tgo-rtc-server main.go
	@echo "$(GREEN)✓ 构建完成$(NC)"

run: ## 运行应用
	@echo "$(BLUE)运行应用...$(NC)"
	go run main.go

stop: ## 停止应用
	@echo "$(BLUE)停止应用...$(NC)"
	pkill -f tgo-rtc-server || true
	@echo "$(GREEN)✓ 应用已停止$(NC)"

logs: ## 查看日志
	@echo "$(BLUE)查看日志...$(NC)"
	tail -f logs/*.log 2>/dev/null || echo "日志文件不存在"

clean: ## 清理构建文件
	@echo "$(BLUE)清理构建文件...$(NC)"
	rm -f tgo-rtc-server
	rm -rf dist/ bin/
	go clean
	@echo "$(GREEN)✓ 清理完成$(NC)"

# ============================================================================
# Docker Compose 部署
# ============================================================================

deploy: ## 部署开发环境（Docker Compose）
	@echo "$(BLUE)部署开发环境...$(NC)"
	@if [ ! -f .env ]; then \
		echo "$(YELLOW)⚠ .env 文件不存在，复制 .env.example...$(NC)"; \
		cp .env.example .env; \
		echo "$(YELLOW)请编辑 .env 文件后重新运行此命令$(NC)"; \
		exit 1; \
	fi
	./部署.sh deploy
	@echo "$(GREEN)✓ 部署完成$(NC)"

deploy-prod: ## 部署生产环境（包含业务服务）
	@echo "$(BLUE)部署生产环境...$(NC)"
	@if [ ! -f .env ]; then \
		echo "$(YELLOW)⚠ .env 文件不存在，复制 .env.example...$(NC)"; \
		cp .env.example .env; \
		echo "$(YELLOW)请编辑 .env 文件后重新运行此命令$(NC)"; \
		exit 1; \
	fi
	docker-compose -f docker-compose.prod.yml up -d
	@echo "$(GREEN)✓ 生产环境部署完成$(NC)"
	@echo "$(BLUE)访问地址: https://$$(grep DOMAIN .env | cut -d= -f2)$(NC)"

init-https: ## 初始化 HTTPS 证书
	@echo "$(BLUE)初始化 HTTPS 证书...$(NC)"
	./部署.sh init-https
	@echo "$(GREEN)✓ HTTPS 证书初始化完成$(NC)"

# ============================================================================
# 数据管理
# ============================================================================

backup: ## 备份数据
	@echo "$(BLUE)备份数据...$(NC)"
	./部署.sh backup
	@echo "$(GREEN)✓ 数据备份完成$(NC)"

restore: ## 恢复数据
	@echo "$(BLUE)恢复数据...$(NC)"
	@read -p "请输入备份目录路径: " backup_path; \
	./部署.sh restore $$backup_path
	@echo "$(GREEN)✓ 数据恢复完成$(NC)"

# ============================================================================
# 维护命令
# ============================================================================

verify: ## 验证部署
	@echo "$(BLUE)验证部署...$(NC)"
	./部署.sh verify
	@echo "$(GREEN)✓ 验证完成$(NC)"

restart: ## 重启服务
	@echo "$(BLUE)重启服务...$(NC)"
	./部署.sh restart
	@echo "$(GREEN)✓ 服务已重启$(NC)"

ps: ## 查看容器状态
	@echo "$(BLUE)容器状态:$(NC)"
	docker-compose -f livekit-deployment/docker-compose.yml ps

docker-logs: ## 查看 Docker 日志
	@echo "$(BLUE)Docker 日志:$(NC)"
	docker-compose -f livekit-deployment/docker-compose.yml logs -f

docker-stop: ## 停止 Docker 容器
	@echo "$(BLUE)停止 Docker 容器...$(NC)"
	docker-compose -f livekit-deployment/docker-compose.yml down
	@echo "$(GREEN)✓ 容器已停止$(NC)"

docker-clean: ## 清理 Docker 容器和卷
	@echo "$(RED)⚠ 警告：此操作将删除所有容器和数据！$(NC)"
	@read -p "确认删除？(y/N): " confirm; \
	if [ "$$confirm" = "y" ]; then \
		docker-compose -f livekit-deployment/docker-compose.yml down -v; \
		echo "$(GREEN)✓ 清理完成$(NC)"; \
	else \
		echo "$(YELLOW)已取消$(NC)"; \
	fi

# ============================================================================
# 开发工具
# ============================================================================

fmt: ## 格式化代码
	@echo "$(BLUE)格式化代码...$(NC)"
	go fmt ./...
	@echo "$(GREEN)✓ 格式化完成$(NC)"

lint: ## 代码检查
	@echo "$(BLUE)代码检查...$(NC)"
	golangci-lint run ./...
	@echo "$(GREEN)✓ 检查完成$(NC)"

test: ## 运行测试
	@echo "$(BLUE)运行测试...$(NC)"
	go test -v ./...
	@echo "$(GREEN)✓ 测试完成$(NC)"

swagger: ## 生成 Swagger 文档
	@echo "$(BLUE)生成 Swagger 文档...$(NC)"
	swag init
	@echo "$(GREEN)✓ Swagger 文档已生成$(NC)"

# ============================================================================
# 快速命令
# ============================================================================

quick-start: ## 快速启动（开发环境）
	@echo "$(BLUE)快速启动...$(NC)"
	@if [ ! -f .env ]; then cp .env.example .env; fi
	./部署.sh deploy
	./部署.sh init-https
	@echo "$(GREEN)✓ 快速启动完成$(NC)"
	@echo "$(BLUE)下一步：运行 'go run main.go' 启动业务服务$(NC)"

quick-stop: ## 快速停止
	@echo "$(BLUE)快速停止...$(NC)"
	./部署.sh stop
	@echo "$(GREEN)✓ 已停止$(NC)"

# ============================================================================
# 信息命令
# ============================================================================

info: ## 显示部署信息
	@echo "$(BLUE)部署信息:$(NC)"
	@echo "域名: $$(grep DOMAIN .env | cut -d= -f2)"
	@echo "Redis 主机: $$(grep REDIS_HOST .env | cut -d= -f2)"
	@echo "LiveKit 节点: $$(grep LIVEKIT_NODES .env | cut -d= -f2 || echo '内置')"
	@echo ""
	@echo "$(BLUE)容器状态:$(NC)"
	@docker-compose -f livekit-deployment/docker-compose.yml ps 2>/dev/null || echo "未部署"

version: ## 显示版本信息
	@echo "$(BLUE)版本信息:$(NC)"
	@echo "Go: $$(go version | awk '{print $$3}')"
	@echo "Docker: $$(docker --version | awk '{print $$3}')"
	@echo "Docker Compose: $$(docker-compose --version | awk '{print $$3}')"

# ============================================================================
# 端到端测试
# ============================================================================
.PHONY: e2e-local

e2e-local: ## 本地端到端测试（自动启动/验证接口）
		@echo "$(BLUE)执行本地 E2E 测试...$(NC)"
		bash scripts/e2e_local.sh || true
		@echo "$(GREEN)✓ E2E 脚本已执行，查看 test-output 目录获取详细结果$(NC)"

