# WebReaper 云端部署指南

> 架构：前端宿主机 nginx + 后端容器（host 网络）+ 共享已有 MySQL

## 前提条件

服务器已有：
- Docker + docker-compose
- MySQL 容器（zhichen-mysql，端口 13306，root/zhichen2026）
- Nginx

## 一、MySQL 建库（共享已有实例）

```bash
# 在已有 MySQL 里创建 webreaper database
docker exec -it zhichen-mysql mysql -uroot -pzhichen2026 -e \
  "CREATE DATABASE IF NOT EXISTS webreaper CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
```

## 二、上传代码 + 配置

```bash
# 方式1：git clone（推荐）
git clone https://github.com/x-PiXiu/WebReaper.git
cd WebReaper

# 方式2：上传代码到服务器（scp/rsync）
```

## 三、配置环境变量

```bash
cp .env.example .env
vim .env  # 填入实际的 API Key、域名、密钥
```

必填项：
- `PUBLIC_BASE_URL`：你的域名（如 `https://webreaper.example.com`）
- `LLM_API_KEY`：LLM 服务密钥
- `JWT_SECRET` / `PUBLISH_COOKIE_SECRET`：生成随机密钥（`openssl rand -hex 32`）

## 四、构建并启动容器

```bash
docker-compose up -d --build
# 查看日志
docker-compose logs -f webreaper
```

启动后自动执行 migrations（建表），监听 :8082。

## 五、前端构建 + nginx 部署

```bash
# 在本地（或服务器）构建前端
cd web
npm install
npm run build  # 产物在 dist/

# 拷贝到 nginx 目录
mkdir -p /var/www/webreaper
cp -r dist/ /var/www/webreaper/

# 配置 nginx
cp deploy/nginx.conf /etc/nginx/sites-available/webreaper
ln -s /etc/nginx/sites-available/webreaper /etc/nginx/sites-enabled/
vim /etc/nginx/sites-available/webreaper  # 改 server_name 为你的域名
nginx -t && nginx -s reload
```

## 六、HTTPS（可选，推荐）

```bash
# Let's Encrypt 自动配置 SSL
certbot --nginx -d webreaper.example.com
```

## 验证

```bash
# 健康检查
curl http://localhost:8082/healthz

# 前端访问
curl http://localhost/  # 应返回 index.html

# API 访问（通过 nginx 反代）
curl http://localhost/healthz
```

## 常见问题

| 问题 | 解决 |
|------|------|
| 容器启动失败：MySQL 连接拒绝 | 确认 zhichen-mysql 在跑 + 13306 端口可达 |
| chromedp 报错 | 容器内 Chromium 需要 `QR_LOGIN_HEADED=false`（headless + no-sandbox，已内置） |
| 扫码登录二维码看不到 | headless 模式下二维码通过截图返回前端（需适配，当前仅全自动发布正常） |
| 前端 404 | 确认 dist/ 路径 + nginx root 配置 |
| API 502 | 确认容器在跑 + localhost:8082 可达 |

## 更新部署

```bash
git pull                    # 拉最新代码
docker-compose up -d --build  # 重建容器
cd web && npm run build && cp -r dist/ /var/www/webreaper/  # 重建前端
```
