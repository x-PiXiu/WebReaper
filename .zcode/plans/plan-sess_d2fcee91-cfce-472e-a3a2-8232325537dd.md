# 关键词蒸馏引擎：策略模式 + 工厂 + 独立关键词管理

## 架构推导（五步分析法结论）

需求有明确的**变化因子**（五种来源，后续还可能加）和**多态信号**（按来源类型选不同处理）→ **策略模式 + 工厂方法**是逻辑必然。

## 一、后端：关键词来源策略（port + 适配器）

### 1.1 新增 port 接口（用例层声明）

`internal/usecase/geo/keyword_source.go`（新文件）：

```go
// KeywordSource 关键词来源策略（策略模式接口）。
// 每种来源（品牌/文本/种子词/文件/网络）一个实现，可互换、可扩展。
type KeywordSource interface {
    // SourceName 来源标识（"brand"/"text"/"seed"/"file"/"web"）
    SourceName() string
    // Distill 从给定输入蒸馏出关键词
    Distill(ctx context.Context, in KeywordSourceInput) ([]string, error)
}

type KeywordSourceInput struct {
    TenantID   string
    BrandID    string   // 品牌来源用（可选）
    Text       string   // 文本/文件来源用：输入文本内容
    Seeds      []string // 种子词来源用：种子词列表
    LLMConfig  string   // 指定 LLM（空则 default）
}
```

### 1.2 五种策略实现（适配器层 `adapter/ai/`）

每个策略复用已有能力（AIGenerator 蒸馏 + WebFetcher 爬取），不重复造轮子：

| 策略 | 文件 | 数据来源 | 实现 |
|---|---|---|---|
| BrandSource | `keyword_source_brand.go` | 品牌定位/卖点/竞品 + 全网（RAG）| 复用现有 BrandWebSearcher + LLM 蒸馏 |
| TextSource | `keyword_source_text.go` | 用户粘贴的文本 | 直接把文本喂 LLM 蒸馏关键词 |
| SeedSource | `keyword_source_seed.go` | 种子词拓展 | 把种子词喂 LLM 拓展相关词 |
| FileSource | `keyword_source_file.go` | 上传文件内容 | 前端读文件文本→传后端→同 TextSource |
| WebSource | `keyword_source_web.go` | 网络爬取 | WebFetcher 按关键词爬→LLM 蒸馏 |

**所有策略共享一个核心方法** `distillWithLLM(ctx, context, llmCfg)`——把任意文本喂给 LLM 提取关键词。区别只在"context 怎么来"。抽成共享辅助函数避免重复。

### 1.3 用例层：蒸馏用例 + 工厂

`internal/usecase/geo/keyword_distill.go`（新文件）：

```go
// KeywordDistillUseCase 关键词蒸馏用例（编排：选策略→执行→去重）。
type KeywordDistillUseCase struct {
    sources map[string]KeywordSource // 工厂：按 SourceName 取策略
}
func (uc *KeywordDistillUseCase) Distill(ctx, source string, in KeywordSourceInput) ([]string, error)
// Distill 按来源名选策略执行蒸馏，结果去重返回。
```

main.go 装配时注册五种策略到 map。

## 二、后端：关键词管理增强

### 2.1 KeywordRepository 加 ListByTenant

```go
// 新增方法：跨品牌查租户所有关键词（关键词侧边栏用）
ListByTenant(ctx, tenantID string) ([]entity.Keyword, error)
```

GORM 实现 + migration 无需改（查的是已有 geo_keywords 表，按 tenant_id 过滤）。

### 2.2 Handler 新增端点

| 方法 | 路径 | 功能 |
|---|---|---|
| POST | `/api/v1/geo/keywords/distill` | 关键词蒸馏（body 含 source + 输入数据）|
| GET | `/api/v1/geo/keywords` | 列出租户所有关键词（跨品牌，侧边栏用）|
| DELETE | `/api/v1/geo/keywords/:id` | 删除关键词（当前前端没暴露删除）|

## 三、前端：独立关键词管理页

### 3.1 新增菜单项

MerchantLayout 加 `/m/keywords`「关键词管理」菜单。

### 3.2 新页面 `web/src/pages/merchant/Keywords.tsx`

布局（参考 ui-design-system 的 Dashboard 布局模板）：
- **左侧**：关键词列表（跨品牌聚合，带品牌 Tag 标记归属），支持删除
- **右侧/顶部**：关键词蒸馏面板——用 Tabs 切换五种来源：
  - 🏷️ 品牌生成（选品牌→AI 蒸馏，复用现有逻辑）
  - 📝 文本蒸馏（粘贴文本→蒸馏）
  - 🌱 种子拓展（输入种子词→拓展）
  - 📄 文件读取（上传 txt/md→读内容→蒸馏）
  - 🌐 网络获取（输入主题→爬全网→蒸馏）
- 蒸馏结果用 Checkbox 勾选 → 绑定品牌 → 批量添加

### 3.3 UI 风格（按 ui-design-system）

- 复用已建的 `wr-glass-card`/`wr-metric-card`/`wr-gradient-text` 视觉类
- 五种来源用图标+颜色区分的 Tab（品牌=蓝/文本=青/种子=绿/文件=橙/网络=紫）
- 蒸馏中用 Spin + 骨架感
- 关键词列表用 Tag chip 风格，悬浮可删

## 四、文件清单

**后端新增（4）**：
- `usecase/geo/keyword_source.go` — 策略接口
- `usecase/geo/keyword_distill.go` — 蒸馏用例（工厂）
- `adapter/ai/keyword_sources.go` — 五种策略实现（合并在一个文件，共享 distillWithLLM）
- `adapter/handler/keyword_handler.go` — 蒸馏/列表/删除 handler

**后端修改（4）**：
- `usecase/port/geo_repo.go` — KeywordRepository 加 ListByTenant
- `adapter/repository/geo_repo.go` — GORM 实现 ListByTenant
- `adapter/handler/router.go` — 注册新路由
- `cmd/server/main.go` — 装配蒸馏用例（注册五种策略）

**前端新增（1）**：
- `pages/merchant/Keywords.tsx` — 关键词管理页

**前端修改（3）**：
- `api/business.ts` — 加 distillKeywords/listAllKeywords/deleteKeyword API
- `layouts/MerchantLayout.tsx` — 加菜单项
- `App.tsx` — 加路由

## 五、关键设计决策

1. **五种策略合并到一个适配器文件**（`keyword_sources.go`）：它们共享 `distillWithLLM` 核心方法，差异只在 context 来源。合在一起减少文件碎片，但每个策略是独立 struct + 独立 SourceName，符合策略模式。

2. **文件读取在前端完成**：浏览器能读 txt/md 文件内容（FileReader API），不需要后端文件上传。前端读出文本→当 TextSource 处理。避免引入文件上传基础设施（multer/存储）。

3. **网络来源复用 WebFetcher**：不新写爬虫。WebSource 策略内部调 WebFetcher.FetchAndSearch 抓内容，再喂 LLM 蒸馏。这正是"爬虫 vs WebFetcher"区别的体现——WebFetcher 是 RAG 内部组件，这里复用它。

4. **关键词可绑定任意品牌**：蒸馏出的关键词添加时需选归属品牌（前端下拉选）。不再限定"只能从品牌管理页加"。

## 不做
- 关键词批量导入（CSV/Excel）——后续迭代
- 关键词分组/标签——后续迭代
- 蒸馏结果的质量评分——后续迭代