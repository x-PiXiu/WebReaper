# WebReaper

基于 Go + 整洁架构的数据采集与智能加工系统。采集 → AI 加工 → 多平台推送的自动化闭环。

> 📐 **架构**：严格遵循整洁架构，依赖方向零违规（grep 验证）
> 🔄 **降级**：双实现（真实+mock），零配置可启动
> 📚 **文档**：完整的[规划文档体系](Docs/WebReaper规划/README.md)

## 核心能力

| 能力 | 实现状态 | 技术栈 |
|---|---|---|
| 通用网页采集 | ✅ 真实 | colly 静态爬虫 + chromedp 动态爬虫 + 搜索/API 爬虫 + 3 装饰器（聚焦/增量/深度）|
| AI 加工（Agent 模型） | ✅ 真实（MiniMax）| trpc-agent-go ReAct Agent + 工具调用（采集/落库/推送工具化）|
| 多平台推送 | ✅ 真实 | 字段映射 + HTTP 推送（raw/mapping 双模式）+ 推送记录审计 |
| 数据持久化 | ✅ 真实（MySQL）| GORM + PO/Entity 分离 + 11 个版本化迁移 |
| HTTP API | ✅ 真实（Gin）| REST + 统一响应信封 + JWT 认证 |
| 异步任务 | ✅ 真实 | 进程内队列 + Worker + 状态机 + 指数退避重试 |
| 可观测性 | ✅ 真实 | port.Tracer 抽象 + OpenTelemetry（stdout/otlp 双 exporter）|
| 前端 UI | ✅ 真实 | React + Vite + AntD（2500+ 行，Nginx 独立部署，前后端分离）|
| 向量检索 | 🟡 降级 | Embedding 真实 + 内存向量库（Milvus 真实 SDK 待接入）|

## 快速开始

```bash
# 零配置启动（降级模式，无需 MySQL/LLM）
go run ./cmd/server

# 完整配置启动
cp configs/.env.example configs/.env  # 填入 MySQL + MiniMax 配置
go run ./cmd/server
```

### 前端 UI（web/，React + Vite + AntD）

采用 **前后端分离** 架构：Go 只服务 API，前端独立构建后由 Nginx 托管。

```bash
# 开发模式（前后端各起一个进程，前端经 vite proxy 调后端，免 CORS）
make dev-server     # 后端 API :8082
make dev-web        # 前端 http://localhost:5173（另一终端）

# 生产部署
make build-web      # 构建前端 → web/dist/，交给 Nginx 托管
make build-server   # 构建后端 → bin/webreaper
```

**Nginx 配置要点**（前端 SPA 由 Nginx 托管，`/api` 反代到 Go，未匹配路径 fallback 到 `index.html` 支持 React Router history 模式）：

```nginx
server {
    listen 80;
    root /var/www/webreaper/dist;        # make build-web 的产物

    # API 反代到 Go 服务
    location /api/ {
        proxy_pass http://127.0.0.1:8082;
        proxy_set_header Host $host;
    }
    # SSE 流式对话（/api/v1/chat）需要关闭缓冲 + 长超时
    location /api/v1/chat {
        proxy_pass http://127.0.0.1:8082;
        proxy_buffering off;
        proxy_read_timeout 300s;
    }

    # SPA fallback：刷新 /agents 等前端路由不 404
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

API 端点（默认 :8082，详见 `internal/adapter/handler/router.go`）：
- `GET  /healthz` 健康检查
- `POST /api/v1/auth/{register,login}` 注册/登录（公开）
- `POST /api/v1/chat` AI 对话（SSE 流式，JWT 保护）
- `POST /api/v1/agents/run` Agent 同步执行
- `POST /api/v1/tasks` / `GET /api/v1/tasks[/:id]` 异步任务投递/查询
- `GET/POST/DELETE /api/v1/agents` Agent 配置管理
- `GET/POST/DELETE /api/v1/llm-configs` LLM 配置管理
- `GET/POST /api/v1/data-items[/:id/{approve,reject}]` 数据项与审核
- `GET/POST/DELETE /api/v1/external-systems` + `POST /publish` 外部系统与推送
- `GET /api/v1/conversations[/:id]` 聊天会话（多轮上下文持久化）
- `GET /api/v1/search` 知识语义搜索
- `GET/PUT /api/v1/crawl-config` 采集配置（速率/robots）

详见 [部署与配置指南](Docs/WebReaper规划/06-部署运维/部署与配置指南.md)。

## 架构概览

```
internal/
├── domain/          实体层（零框架依赖，最稳定）
│   ├── entity/      DataItem/Task/User/AgentConfig/LLMConfig/ExternalSystem 等
│   └── valueobject/ TaskStatus（状态机）
├── usecase/         用例层（只依赖 domain + port）
│   ├── port/        边界接口（Logger/Tracer/Repository/TaskQueue 等依赖倒置核心）
│   ├── task/        异步任务（Enqueue/Worker/Dispatch + AgentHandler）
│   ├── publish/     推送用例（字段映射 + HTTP + 记录）
│   ├── process/     数据闭环（结构化→向量化→检索）
│   ├── dataitem/    数据项 + 审核编排
│   └── auth/conversation/crawlconfig/...
├── adapter/         适配器层（框架细节封装于此）
│   ├── handler/     Gin HTTP Controller（JWT 中间件 + SPA fallback）
│   ├── crawler/     colly 爬虫工具（4 基础 + 3 装饰器 + publish/save 工具）
│   ├── ai/          trpc-agent-go（MiniMax，工具调用 + 流式对话）
│   ├── repository/  GORM 仓储（MySQL/SQLite + 版本化迁移）
│   ├── telemetry/   OpenTelemetry（port.Tracer 实现）
│   ├── vectorstore/ 向量库（Memory 真实 + Milvus 诚实降级）
│   └── mock/        全部接口的降级实现
├── config/          统一配置加载（含 Telemetry）
└── pkg/             错误码
web/                 前端 SPA（React+Vite，独立构建，Nginx 托管）
```

**依赖铁律**：`domain` ← `usecase` ← `adapter` ← `main`，箭头永远向内。
**架构转向（ADR-006）**：单一 Agent + N 工具模型取代 4 独立用例（详见规划文档）。

**部署架构**：前后端分离——Go 纯 API 服务，前端独立构建后由 Nginx 托管（SPA fallback），`/api` 反代到 Go。

整洁架构符合性详见 [架构体检报告](Docs/WebReaper规划/02-架构设计/01-整洁架构符合性分析.md)（评级 A-）。

## 测试

```bash
go test ./...                    # 全量测试（无需外部依赖）
go test -cover ./...             # 覆盖率
```

## 文档

完整的规划文档体系位于 [`Docs/WebReaper规划/`](Docs/WebReaper规划/README.md)，涵盖：
- 📋 [需求分析](Docs/WebReaper规划/01-需求分析/业务需求与用户画像.md)
- 🏗️ [架构设计](Docs/WebReaper规划/02-架构设计/01-整洁架构符合性分析.md)（符合性/模块边界/模式推导/反思）
- 📅 [开发计划](Docs/WebReaper规划/03-开发计划/01-分阶段演进路线图.md)（路线图/优先级决策）
- 🔧 [功能详解](Docs/WebReaper规划/04-功能详解/01-招聘需求采集.md)（4 个用例逐一详解）
- 🎯 [战略规划](Docs/WebReaper规划/05-战略规划/数据采集差异化战略.md)（差异化战略/trpc 集成策略）
- 🚀 [部署运维](Docs/WebReaper规划/06-部署运维/部署与配置指南.md)

## 配置

配置通过 `configs/.env` 加载（参考 `.env.example`）。支持 MySQL / MiniMax LLM / Embedding / Milvus / Redis / JWT / Telemetry 八类配置，缺失项自动降级。详见 [配置指南](Docs/WebReaper规划/06-部署运维/部署与配置指南.md)。

## 与 AgentCore 的关系

WebReaper 是 [AgentCore](../AgentCore/) 的数据侧姊妹项目：WebReaper 生产内容（采集+AI加工），通过推送 API 输出到 AgentCore 的内容接入网关。两者共享相同的整洁架构理念。
