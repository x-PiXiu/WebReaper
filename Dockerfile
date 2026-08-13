# Stage 1: Go 编译（纯静态，CGO_ENABLED=0）
# 注意：builder 镜像版本必须 ≥ go.mod 声明的 go 版本（当前 go 1.26.1）
FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o webreaper ./cmd/server

# Stage 2: 运行时（Chromium headless + Go 二进制）
FROM debian:bookworm-slim
# Chromium（RPA 发布用）+ 中文字体（网页渲染/截图）+ CA 证书
RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    fonts-noto-cjk \
    fonts-noto-color-emoji \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && ln -sf /usr/bin/chromium /usr/bin/google-chrome

WORKDIR /app
COPY --from=builder /build/webreaper .
RUN mkdir -p data/media

ENV SERVER_PORT=8082 \
    APP_ENV=production \
    QR_LOGIN_HEADED=false

EXPOSE 8082
CMD ["./webreaper"]
