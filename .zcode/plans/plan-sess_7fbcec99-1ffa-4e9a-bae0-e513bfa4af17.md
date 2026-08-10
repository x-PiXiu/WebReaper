# 经济系统实现计划（订阅 / 配额 / 计量 / 计费后台）

依据整洁架构四层 + 依赖倒置。所有新概念先在 domain/port 建模，再在 adapter 实现，main 装配。支付网关用 port 接口 + mock 适配器（与 Vidu 同款降级策略——真实对接需商户资质，当前阶段不可能）。

## 架构总览

```
domain/entity     Plan / Subscription / Order / Invoice / QuotaExceededError
usecase/port      SubscriptionRepo / OrderRepo / PlanRepo / QuotaStore / PaymentGateway / UsageQueryer
usecase/billing   BillingUseCase（订阅/订单/配额查询）
usecase/quota     Guard（装饰器：包 usecase 方法，CheckQuota→执行）
adapter/repository plans/subscriptions/orders/invoices GORM
adapter/payment   MockPaymentGateway（开发降级；真实 StripeAdapter 预留位）
adapter/handler   billing_handler（admin + 商户端端点）
cmd/server        装配 + 装饰器包装
```

## 批次 1：订阅实体 + Plan 配置化（domain + repo）

**domain/entity/billing.go**
```go
type Plan struct {
    ID, Name, Level string          // free/pro/team
    PriceCents int                  // 分（避免浮点）
    Quotas map[string]int           // scene→配额（monitor:500, content-gen:50, video:10, chat:-1）；-1=无限
    Features []string               // 功能开关白名单
    Status string                   // active/archived
}

type Subscription struct {
    ID, TenantID, PlanID string
    Status string                    // active/expired/cancelled
    PeriodStart, PeriodEnd time.Time // 计费周期（按月）
    CreatedAt time.Time
}

type Order struct {
    ID, TenantID, PlanID string
    AmountCents int
    Status string                    // pending/paid/refunded/failed
    PaymentGateway, PaymentID string // 第三方流水
    CreatedAt, PaidAt time.Time
}
```

**usecase/port/billing_repo.go** —— SubscriptionRepo / PlanRepo / OrderRepo（Save/Find/UpdateStatus/List）

**adapter/repository/billing_model.go + billing_repo.go** —— GORM PO + 实现
- 迁移 025_billing：plans / subscriptions / orders 三表

**seed**：启动写内置 3 套餐（free/pro/team，配额参考上次经济系统分析）

**admin 端点**：GET/POST/PUT /admin/plans（套餐 CRUD）

## 批次 2：配额检查（装饰器模式）

**usecase/quota/guard.go** —— 核心装饰器
```go
type Guard struct {
    inner     any       // 被包装的 usecase（用 interface{} 反射或显式包装）
    store     port.QuotaStore
    scene     string
}

// QuotaStore 抽象配额存取（Redis/DB）
type QuotaStore interface {
    // Remaining 返回租户某场景剩余配额（-1=无限）
    Remaining(ctx, tenantID, scene string) (int, error)
    // Consume 扣减配额（用量计数已超时返回 false）
    Consume(ctx, tenantID, scene string, n int) error
}
```

**接入方式**：在 main 装配时，给烧 token 的 usecase 套装饰器。两种策略：
- **简单方案**：每个 usecase 加 `SetQuotaGate(gate)`，方法开头 `gate.Check(ctx, "scene")`。改动面小、显式可读。
- **纯净方案**：用 Go 接口组合，把 ContentUseCase 包进 `QuotaGuardedContent` 实现同接口。零侵入但模板代码多。

**采用简单方案**（与现有 SetRuleScorer/SetRAGRetriever 注入风格一致）。QuotaGate 检查点：
- `ContentUseCase.Optimize/Generate`（content-opt/content-gen）
- `MonitorUseCase.Monitor`（monitor）
- `KeywordDistillUseCase.Distill`（keyword-distill）

超限返回 `pkg.ErrQuotaExceeded`（新增），handler 经 `statusForError` 映射 HTTP 402。

## 批次 3：计量挂钩（补 ctx 注入，消费点已有）

**关键洞察**：`trpc_agent.go:335` 的 `RecordUsage` 消费点已存在，缺的是上游 ctx 注入。补三处：
1. **chat**：`chat_handler.go:93` 调 ChatStream 前注入 `WithUsageContext(ctx, tenantID, "chat")`
2. **keyword-distill**：`KeywordDistillUseCase.Distill` 调 LLM 前注入
3. **monitor（自动盯盘）**：`DailyMonitorTask.Execute` 调 MonitorUC 前注入（已有 monitor.go:59 单关键词打点，补批量任务的）

**计费配额扣减**：在 `RecordUsage` 成功后，调 `QuotaStore.Consume` 扣配额（消费点统一扣减，避免散落）。

## 批次 4：完整计费后台

**usecase/billing/billing.go** —— BillingUseCase
```go
type BillingUseCase struct {
    planRepo, subRepo, orderRepo, usageRepo ...
    payment port.PaymentGateway  // 可选：nil=线下模式
}

// 商户端
func (uc) GetMyPlan(ctx, tenantID) (Plan, Subscription, UsageSummary)
func (uc) CreateOrder(ctx, tenantID, planID) (Order, PaymentURL)  // 拉起支付
func (uc) ConfirmPayment(ctx, orderID) error                       // 回调确认
// admin 端
func (uc) AssignPlan(ctx, tenantID, planID) error                  // 手动开通
func (uc) RevenueReport(ctx) (RevenueSummary)                      // 收入报表
```

**usecase/port/payment.go**
```go
type PaymentGateway interface {
    CreatePayment(ctx, order Order) (paymentURL, paymentID string, err error)
    QueryPayment(ctx, paymentID) (status string, err error)
}
```

**adapter/payment/mock.go** —— MockPaymentGateway（模拟支付 URL + 自动确认）
**adapter/payment/stripe.go** —— 预留（未配置 Stripe key 时不注册）

**admin 端点**：
- GET /admin/billing/revenue —— 收入报表（MRR、各套餐租户数、到期预警）
- GET /admin/billing/subscriptions —— 全部订阅
- PUT /admin/billing/subscriptions/:tenant —— 手动开通/变更套餐
- GET /admin/billing/orders —— 订单流水

**商户端端点**：
- GET /billing/my-plan —— 我的套餐 + 用量（配额余量）
- POST /billing/orders —— 创建订单拉起支付

## 批次 5：Stats 扩展 + 前端 + 测试

**StatsUseCase 扩展**：注入 UsageRepository，聚合按租户/场景/月的 token 消耗（收入报表数据源）

**前端**（web/src/pages/admin/Billing.tsx）：
- 套餐管理（CRUD）
- 订阅列表 + 手动开通
- 收入仪表盘（MRR 折线、套餐分布饼图、到期预警表）
- 商户端"我的套餐"页（用量进度条）

**测试**：billing/quota usecase 单测（mock 仓储），配额超限场景，装饰器拦截验证

## 不做的事（诚实边界）
- **真实支付网关对接**：StripeAdapter 只写骨架 + TODO，不接真实 API（需商户资质/密钥/回调签名验证，属另一阶段工程）
- **自动续费**：需要支付代扣协议，超出范围
- **发票/税务**：需要税控系统对接，超出范围
- **前端支付 UI**：商户端只做"创建订单→mock 支付页→确认"演示流程

## 实施顺序与提交粒度
1. 提交①：billing 实体 + repo + 迁移 + seed + Plan CRUD（批次1）
2. 提交②：quota 装饰器 + 三处 usecase 接入 + ErrQuotaExceeded（批次2）
3. 提交③：计量挂钩三处 ctx 注入 + Consume 扣减（批次3）
4. 提交④：BillingUseCase + payment port/mock + admin/商户端 API（批次4）
5. 提交⑤：Stats 扩展 + 前端计费页 + 测试（批次5）

每批次独立编译/测试/提交，保持可回滚。全程遵循现有代码风格（success/fail 信封、SetXxx 注入、mock 仓储模式、迁移版本化）。