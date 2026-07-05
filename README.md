# WebReaper

基于 Go + 整洁架构的数据采集与智能加工系统。采集 → AI 加工 → 多平台推送的自动化闭环。

> 📐 **架构**：严格遵循整洁架构，依赖方向零违规（grep 验证）
> 🔄 **降级**：双实现（真实+mock），零配置可启动
> 📚 **文档**：完整的[规划文档体系](Docs/WebReaper规划/README.md)

## 核心能力

| 能力 | 实现状态 | 技术栈 |
|---|---|---|
| 招聘需求采集 | ✅ 真实（colly 通用爬虫）| colly + CSS 选择器配置化 |
| AI 生成面试题 | ✅ 真实（MiniMax）| trpc-agent-go + Prompt 工程 |
| AI 总结知识点 | ✅ 真实（MiniMax）| 复用 AIGenerator 接口 |
| 多平台推送 | ✅ 接口完整，真实待接线 | 策略+注册表，HTTP+X-API-Key |
| 数据持久化 | ✅ 真实（MySQL agentcore）| GORM + PO/Entity 分离 |
| HTTP API | ✅ 真实（Gin）| REST + 统一响应信封 + 错误码映射 |

## 快速开始

```bash
# 零配置启动（降级模式，无需 MySQL/LLM）
go run ./cmd/server

# 完整配置启动
cp configs/.env.example configs/.env  # 填入 MySQL + MiniMax 配置
go run ./cmd/server
```

API 端点（默认 :8082）：
- `GET  /healthz` 健康检查
- `POST /api/v1/collect` 采集招聘需求
- `POST /api/v1/questions/generate` 生成面试题
- `POST /api/v1/knowledge/summarize` 总结知识点
- `POST /api/v1/publish` 推送内容

详见 [部署与配置指南](Docs/WebReaper规划/06-部署运维/部署与配置指南.md)。

## 架构概览

```
internal/
├── domain/          实体层（零框架依赖，最稳定）
│   ├── entity/      JobPost/Question/Knowledge/Source/Task
│   └── valueobject/ Fingerprint/TaskStatus/PublishResult
├── usecase/         用例层（只依赖 domain + port）
│   ├── port/        7 个边界接口（依赖倒置核心）
│   ├── collect/     采集用例
│   ├── generate/    生成面试题用例
│   ├── summarize/   总结知识点用例
│   └── publish/     推送用例
├── adapter/         适配器层（框架细节封装于此）
│   ├── handler/     Gin HTTP Controller（谦卑对象）
│   ├── spider/      colly 爬虫
│   ├── ai/          trpc-agent-go（MiniMax）
│   ├── publish/     HTTP 推送平台
│   ├── repository/  GORM 仓储（MySQL/SQLite）
│   └── mock/        全部接口的降级实现
├── config/          统一配置加载
└── pkg/             错误码
```

**依赖铁律**：`domain` ← `usecase` ← `adapter` ← `main`，箭头永远向内。

整洁架构符合性详见 [架构体检报告](Docs/WebReaper规划/02-架构设计/01-整洁架构符合性分析.md)（评级 A-）。

## 测试

```bash
go test ./...                    # 全量测试（56 个，无需外部依赖）
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

配置通过 `configs/.env` 加载（参考 `.env.example`）。支持 MySQL / MiniMax LLM / Embedding / Milvus / Redis / JWT 七类配置，缺失项自动降级。详见 [配置指南](Docs/WebReaper规划/06-部署运维/部署与配置指南.md)。

## 与 AgentCore 的关系

WebReaper 是 [AgentCore](../AgentCore/) 的数据侧姊妹项目：WebReaper 生产内容（采集+AI加工），通过推送 API 输出到 AgentCore 的内容接入网关。两者共享相同的整洁架构理念。
