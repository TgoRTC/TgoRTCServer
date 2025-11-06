# 多阶段构建：编译阶段
FROM golang:1.23-alpine AS builder

# 使用阿里云镜像源加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 安装依赖
RUN apk add --no-cache git make

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 注意：docs/ 目录中的 swagger 文件是手动维护的完整版本
# 不需要运行 swag init，直接使用已有的文件即可

# 编译应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o tgo-rtc-server \
    main.go

# ============================================================================
# 运行阶段
# ============================================================================
FROM alpine:latest

# 安装运行时依赖（包含 wget/curl 供健康检查使用）
RUN apk add --no-cache ca-certificates tzdata wget curl

# 创建非 root 用户
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/tgo-rtc-server .
COPY --from=builder /app/docs ./docs
COPY --from=builder /app/migrations ./migrations

# 设置权限
RUN chown -R appuser:appuser /app

# 切换用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# 启动应用
CMD ["./tgo-rtc-server"]

