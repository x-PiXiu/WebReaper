# WebReaper 云端部署检查清单

> 配套：`deploy/env.production.template`（生产 .env 模板）
> 架构：docker-compose（host 网络）+ 宿主机 nginx + 共享 zhichen-mysql(:13306)/redis(:16379)

---

## 一、部署前（本地准备）

- [ ] `git pull` 最新代码（含 31/32 号全部修复）
- [ ] 前端构建：`cd web && npm run build`（VITE_API_PREFIX=/webreaper 已在 .env.production）
- [ ] 拷贝 `deploy/env.production.template` → 云端根目录 `.env`，填写【填写】占位项（Vidu Key）

## 二、云端基础设施确认

- [ ] MySQL(zhichen-mysql :13306) 运行中，且已建 `webreaper` database（首次部署自动迁移建表，但库本身需存在）
- [ ] Redis(:16379) 运行中，密码与 .env 一致
- [ ] nginx 已装并加载 `deploy/nginx.conf`（证书 certbot 签发 huoke.zhichen.chat）
- [ ] OSS bucket `zhichenai` 可写，`media.zhichen.chat` CDN 域名 HTTPS 可访问
- [ ] 防火墙/安全组：80/443 开放；**8082 不要对公网开放**（nginx 反代即可）

## 三、部署

```bash
# 云端根目录（.env 与 docker-compose.yml 同级）
docker compose build
docker compose up -d
docker compose logs -f --tail=100   # 观察启动
```

启动日志核对（应看到）：
- [ ] `HTTP 服务已启动 {"port": "8082"}`
- [ ] `统一生成已接入 Vidu（真实 API…）`——若显示 mock 模式说明 Vidu Key 未生效
- [ ] `小米MiMo TTS provider 已注册`——缺失说明 MIMO_API_KEY 未生效
- [ ] `媒体存储已启用`（OSS 模式）——若失败检查 OSS 凭证
- [ ] 无 `Data too long` / 迁移报错

## 四、部署后验证（按链路）

| # | 验证项 | 方法 | 预期 |
|---|--------|------|------|
| 1 | 健康检查 | `curl https://huoke.zhichen.chat/webreaper/healthz` | 200 |
| 2 | 前端页面 | 浏览器打开 https://huoke.zhichen.chat | 登录页正常 |
| 3 | **素材上传走 OSS** | 商户端上传一张图 | URL 是 media.zhichen.chat（非 localhost）|
| 4 | **音色注册链**（本地受限项）| 商户端克隆音色（test6669）| 成功 + 口播可用——公网样本 URL 本次应通过 |
| 5 | 口播生成 | 分身+文案+Vidu 预置音色 → 提交 | 1-3 分钟出片，卡片"可用" |
| 6 | 试听播放 | 音色库点试听 | 音频正常播放（media 域名）|
| 7 | 抖音绑定 | 扫码绑定（云端 headless）| 观察 cookie 是否签发；不行则有头模式人工过 |
| 8 | 发布通道 | 提交一条半自动发布 | 任务创建成功 |
| 9 | 机审灰度（可选）| 管理后台开 gen_moderation_enabled | 违规文案进 flagged 队列 |

## 五、已知环境差异（本地 vs 云端）

| 项 | 本地 | 云端 |
|----|------|------|
| 音色注册链 | ❌ 受限（localhost 样本 Vidu 拉不到） | ✅ OSS 公网 URL 可注册 |
| C 路径口播（上传素材） | ❌ 同上 | ✅ |
| Vidu 回调通道 | 纯轮询 20s | 回调秒级 + 轮询兜底 |
| RPA 抖音扫码 | 有头模式人工过验证 | headless——**首次绑定可能触发风控**，失败则临时开 headed |

## 六、回滚

```bash
docker compose down
docker compose up -d --build  # 用上一个 git tag 重新构建
```

数据安全：媒体在 OSS（独立持久化）；DB 在共享 MySQL（不受容器生命周期影响）——回滚不丢数据。
