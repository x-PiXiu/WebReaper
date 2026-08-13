# Stage 1: Go 编译（纯静态，CGO_ENABLED=0）
# 注意：builder 镜像版本必须 ≥ go.mod 声明的 go 版本（当前 go 1.26.1）
FROM golang:1.26-alpine AS builder
# 国内网络：Go 模块走七牛云代理（proxy.golang.org 境外直连超时）
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o webreaper ./cmd/server

# Stage 2: 运行时（Chromium headless + Go 二进制）
FROM debian:bookworm-slim
# 国内网络：apt 换阿里云镜像（deb.debian.org 境外直连慢/超时）
RUN sed -i -e 's|deb.debian.org|mirrors.aliyun.com|g' \
           -e 's|security.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources
# Chromium（RPA 发布用）+ 中文字体（网页渲染/截图）+ CA 证书 + dbus（Chromium 系统总线）
RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    dbus \
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
