.PHONY: help build run test clean deploy up update stop logs

# 镜像配置
REGISTRY := crpi-4ja8peh93d2yb8c8.cn-shanghai.personal.cr.aliyuncs.com
NAMESPACE := slun
IMAGE_NAME := tgortc
TAG := latest
FULL_IMAGE := $(REGISTRY)/$(NAMESPACE)/$(IMAGE_NAME):$(TAG)

# 颜色定义
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m

.DEFAULT_GOAL := help

# ============================================================================
# 帮助
# ============================================================================

help: ## 显示帮助信息
	@echo "$(BLUE)TgoRTC Server - 构建部署工具$(NC)"
	@echo ""
	@echo "$(YELLOW)开发命令:$(NC)"
	@echo "  make build    构建本地二进制"
	@echo "  make run      本地运行"
	@echo "  make test     运行测试"
	@echo "  make fmt      格式化代码"
	@echo "  make clean    清理构建文件"
	@echo ""
	@echo "$(YELLOW)部署命令:$(NC)"
	@echo "  make deploy   构建并推送镜像（一键部署）"
	@echo "  make up       启动服务"
	@echo "  make update   更新服务（拉取最新镜像并重启）"
	@echo "  make stop     停止服务"
	@echo "  make logs     查看日志"
	@echo ""

# ============================================================================
# 开发命令
# ============================================================================

build: ## 构建本地二进制
	@echo "$(BLUE)构建应用...$(NC)"
	go build -o tgo-rtc-server main.go
	@echo "$(GREEN)✓ 构建完成$(NC)"

run: ## 本地运行
	@echo "$(BLUE)运行应用...$(NC)"
	go run main.go

test: ## 运行测试
	@echo "$(BLUE)运行测试...$(NC)"
	go test -v ./...

fmt: ## 格式化代码
	@echo "$(BLUE)格式化代码...$(NC)"
	go fmt ./...
	@echo "$(GREEN)✓ 格式化完成$(NC)"

clean: ## 清理构建文件
	@echo "$(BLUE)清理构建文件...$(NC)"
	rm -f tgo-rtc-server
	go clean
	@echo "$(GREEN)✓ 清理完成$(NC)"

# ============================================================================
# 部署命令
# ============================================================================

deploy: ## 构建并推送镜像（一键部署）
	@echo "$(BLUE)构建 Docker 镜像...$(NC)"
	docker build -t $(FULL_IMAGE) . --platform linux/amd64
	@echo "$(GREEN)✓ 镜像构建完成: $(FULL_IMAGE)$(NC)"
	@echo "$(BLUE)推送 Docker 镜像...$(NC)"
	docker push $(FULL_IMAGE)
	@echo "$(GREEN)✓ 部署完成: $(FULL_IMAGE)$(NC)"

up: ## 启动服务
	@echo "$(BLUE)启动服务...$(NC)"
	DOCKER_IMAGE=$(FULL_IMAGE) docker compose up -d
	@echo "$(GREEN)✓ 服务已启动$(NC)"

update: ## 更新服务（拉取最新镜像并重启）
	@echo "$(BLUE)更新服务...$(NC)"
	DOCKER_IMAGE=$(FULL_IMAGE) docker compose pull tgo-rtc-server
	DOCKER_IMAGE=$(FULL_IMAGE) docker compose up -d tgo-rtc-server
	@echo "$(GREEN)✓ 服务已更新$(NC)"

stop: ## 停止服务
	@echo "$(BLUE)停止服务...$(NC)"
	DOCKER_IMAGE=$(FULL_IMAGE) docker compose down
	@echo "$(GREEN)✓ 服务已停止$(NC)"

logs: ## 查看日志
	DOCKER_IMAGE=$(FULL_IMAGE) docker compose logs -f
