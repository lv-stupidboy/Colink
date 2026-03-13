# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git

# 复制go.mod和go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /isdp ./cmd/server

# 运行阶段
FROM alpine:3.19

WORKDIR /app

# 安装ca证书（用于HTTPS）
RUN apk --no-cache add ca-certificates tzdata

# 从构建阶段复制二进制文件
COPY --from=builder /isdp /app/isdp

# 复制配置文件
COPY configs/ /app/configs/

# 创建非root用户
RUN addgroup -g 1000 isdp && \
    adduser -u 1000 -G isdp -s /bin/sh -D isdp

USER isdp

EXPOSE 8080

ENTRYPOINT ["/app/isdp"]
CMD ["--config", "/app/configs/config.yaml"]