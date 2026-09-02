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

# Stage 2: 运行时（Chromium headless + Python sidecar + yt-dlp + ffmpeg + Go 二进制）
FROM debian:bookworm-slim
# 国内网络：apt 换阿里云镜像（deb.debian.org 境外直连慢/超时）
RUN sed -i -e 's|deb.debian.org|mirrors.aliyun.com|g' \
           -e 's|security.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources
# Chromium（RPA 发布用）+ 中文字体（网页渲染/截图）+ CA 证书 + dbus（Chromium 系统总线）
# + python3/pip（抖音解析 Python sidecar——抖音 WAF 按 TLS 指纹分流，Go 直连只拿壳页）
# + ffmpeg（文案提取抽音轨 16k mp3——缺失时降级 ≤25MB 整文件直传 ASR）
# + wget（compose healthcheck 探活）
RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    dbus \
    fonts-noto-cjk \
    fonts-noto-color-emoji \
    ca-certificates \
    python3 \
    python3-pip \
    ffmpeg \
    wget \
    tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && ln -sf /usr/bin/chromium /usr/bin/google-chrome \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone
# Python 依赖：requests（抖音 sidecar）+ yt-dlp（长尾平台通用解析——YouTube/微博/西瓜等 1800+ 站点）
#（debian bookworm 的 PEP 668 管控环境需 --break-system-packages；pip 走阿里云镜像）
RUN pip3 install --break-system-packages --no-cache-dir \
    -i https://mirrors.aliyun.com/pypi/simple/ \
    requests yt-dlp

WORKDIR /app
COPY --from=builder /build/webreaper .
RUN mkdir -p data/media

# 时区：北京时间——定时发布/调度器/日志时间戳全部依赖（默认 UTC 差 8 小时）
ENV TZ=Asia/Shanghai \
    SERVER_PORT=8082 \
    APP_ENV=production \
    QR_LOGIN_HEADED=false

EXPOSE 8082
CMD ["./webreaper"]
