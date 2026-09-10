# --- Stage 1: Build ---
FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 编译静态二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -o /rag-backend .

# --- Stage 2: Final Image ---
FROM alpine:latest

# 复制编译好的程序
COPY --from=builder /rag-backend /rag-backend

# 暴露API服务器监听的8080端口
EXPOSE 8080

# 设置容器启动命令
ENTRYPOINT ["/rag-backend"]
