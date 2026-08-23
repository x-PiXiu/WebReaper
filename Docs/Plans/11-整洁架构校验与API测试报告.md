# 11 - 整洁架构校验与 API 测试报告

> 日期：2026-08-23 | 作者：ZCode Agent | 基于 architect-handbook 整洁架构分析

## 一、整洁架构合规性分析

### 1.1 分层结构总览

```
cmd/server/main.go          ← 组合根（唯一允许 import 所有层的地方）
  └─ internal/
       ├─ domain/entity/     ← 实体层（最内层，零外部依赖）
       ├─ usecase/            ← 用例层（依赖 domain + port）
       │    ├─ port/          ← 端口接口（依赖倒置的边界）
       │    └─ generation/    ← 生成用例实现
       └─ adapter/            ← 适配器层（依赖 usecase/port + domain）
            ├─ handler/       ← HTTP 处理器
            ├─ ttsmimo/       ← MiMo TTS 适配器
            ├─ repository/    ← 数据库仓储
            └─ ...
```

### 1.2 依赖方向校验

| 层 | 导入的包 | 是否合规 | 说明 |
|---|---|---|---|
| **domain/entity** | 仅 stdlib | ✅ 合规 | 最内层，零外部依赖 |
| **usecase/port** | stdlib + domain/entity | ✅ 合规 | 接口定义只依赖实体 |
| **usecase/generation** | stdlib + domain + port | ✅ 合规 | 用例实现只依赖接口 |
| **adapter/ttsmimo** | stdlib + domain + port | ✅ 合规 | 适配器向内依赖 |
| **adapter/handler** | gin + domain + **usecase/generation** | ⚠️ 轻微违规 | handler 直接依赖具体用例类型 |
| **cmd/server** | 所有层 | ✅ 合规 | 组合根允许 import 一切 |

### 1.3 SOLID 原则合规性

#### S — 单一职责原则 ✅

| 组件 | 职责 | 评价 |
|---|---|---|
| `GenerationUseCase` | 生成任务全生命周期管理 | 职责聚焦，~958 行略大但内聚 |
| `EndpointSelectorImpl` | 根据素材自动选择端点 | 单一职责 |
| `MiMoAsGenerationProvider` | 适配 MiMo TTS 为 GenerationProvider | 桥接适配，职责清晰 |
| `MiMoTTSProvider` | MiMo TTS HTTP 通信 | 纯技术适配 |

#### O — 开闭原则 ✅（轻微问题）

- **新增厂商**：只需新增 adapter 实现 + main.go 注册一行 → 用例层零改动 ✅
- **新增端点**：需修改 `subTypeToCapID` 映射表 → 用例层需改动 ⚠️
  - 建议：将映射表移到 `EndpointRegistry` 或配置驱动

#### L — 里氏替换原则 ✅

- `MiMoAsGenerationProvider` 实现 `port.GenerationProvider`，可无缝替换 Vidu
- 同步端点通过 `SubmitResult.State=success` 信号终态，无需轮询

#### I — 接口隔离原则 ✅

- `GenerationProvider` 核心接口（6 方法）
- 可选能力通过窄接口 + 类型断言发现：`SyncSubmitter`、`CallbackEndpoint`、`ModelAutoSelector`
- 避免了"胖接口"问题

#### D — 依赖倒置原则 ✅（handler 层轻微违规）

- 所有 port 接口定义在 usecase 层，adapter 层实现
- handler 直接 import `usecase/generation` 具体类型（非 port 接口）
- **影响**：编译期耦合，不影响运行时行为，但阻碍多态替换

### 1.4 关键设计模式分析

| 模式 | 应用位置 | 评价 |
|---|---|---|
| **策略模式** | `GenerationProvider`（多厂商）、`EndpointAdapter`（多端点） | ✅ 教科书级实现 |
| **适配器模式** | `MiMoAsGenerationProvider`（TTS → GenerationProvider） | ✅ 组合优于继承 |
| **注册表模式** | `EndpointRegistry`（端点策略注册表） | ✅ 开闭原则支撑 |
| **仓库模式** | `GenerationTaskRepository` | ✅ 标准实现 |
| **依赖注入** | main.go 组合根 + Set* 方法注入可选依赖 | ✅ 实用主义 |

## 二、API 回归测试结果

### 2.1 测试矩阵

| # | 测试场景 | 端点 | 预期结果 | 实际结果 | 状态 |
|---|---|---|---|---|---|
| 1 | TTS（MiMo） | POST /generation/submit | provider=xiaomi-mimo, state=success | ✅ 符合预期 | **PASS** |
| 2 | 声音克隆（缺参数） | POST /generation/submit | 返回参数校验错误 | ✅ "audio_url 必填" | **PASS** |
| 3 | 任务列表 | GET /generation/tasks | 返回任务列表 | ✅ code=0, count=5 | **PASS** |
| 4 | 文生视频（Vidu） | POST /generation/submit | 提交到 Vidu | ⚠️ 分辨率校验失败 | **PRE-EXISTING** |
| 5 | 模板列表 | GET /generation/templates | 返回模板列表 | ⚠️ 404 | **PRE-EXISTING** |

### 2.2 回归分析

**无回归引入**。所有测试失败均为预存问题：

1. **分辨率 720p 校验失败**：Vidu 端点 `text2video` 的有效分辨率已从 `720p` 变更为 `1080p`，需要更新 `generation_specs` 表或默认值。这是 Vidu 上游变更，与本次修改无关。

2. **模板列表 404**：`TemplateUseCase` 未在 `main.go` 中装配（`router.SetTemplate()` 未调用），路由未注册。这是功能未完成的状态，与本次修改无关。

### 2.3 MiMo TTS 完整链路验证

```
客户端 → POST /api/v1/generation/submit {"type":"audio","text":"你好世界"}
  → handler.HandleUnifiedSubmit（解析 JSON）
  → generation.UnifiedSubmit（构建 UnifiedGenerationRequest）
  → EndpointSelector.Select（type=audio → subType=tts）
  → generation.Submit（getProvider("tts") → capResolver → xiaomi-mimo）
  → ttsAdapter.BuildRequest（构建 Vidu 格式 body）
  → MiMoAsGenerationProvider.Submit（提取 text/voiceID → 调用 MiMo TTS）
  → MiMoTTSProvider.Synthesize（HTTP POST → mimo-v2.5-tts）
  → 返回 base64 音频 → 同步终态 success
```

**链路验证通过**，中文文本在 Unicode 转义下正确传输，音频时长合理（2.88秒/15字）。

## 三、客户端分析：统一生成 API 下的职责划分

### 3.1 服务端已统一的能力

| 能力 | 服务端实现 | 客户端是否需要关心 |
|---|---|---|
| 端点选择 | `EndpointSelector` 根据素材类型自动选择 | ❌ 不需要 |
| 厂商路由 | `getProvider` + `CapabilityResolver` 动态选择 | ❌ 不需要 |
| 模型选择 | `ModelAutoSelector` / `getDefaultModel` 自动选择 | ❌ 不需要 |
| 参数组装 | `EndpointAdapter.BuildRequest` 端点特定组装 | ❌ 不需要 |
| 默认值填充 | `applyDefaults`（时长/分辨率/比例） | ❌ 不需要 |
| 任务管理 | 提交/轮询/回调/重试/清理 | ❌ 不需要 |
| 产物转存 | 24h URL → 永久存储 | ❌ 不需要 |

### 3.2 客户端只需要关注的内容

```
┌─────────────────────────────────────────────┐
│              客户端职责（仅此而已）              │
├─────────────────────────────────────────────┤
│  1. 布局与样式（UI/UX）                       │
│     - 页面布局、组件样式、响应式设计            │
│     - 动画效果、交互反馈                       │
│                                              │
│  2. 用户交互流程                              │
│     - 素材上传/选择                           │
│     - 文本输入                                │
│     - 模板选择                                │
│     - 提交按钮                                │
│                                              │
│  3. 数据展示                                  │
│     - 任务状态展示（loading/success/failed）   │
│     - 产物预览/下载                           │
│     - 任务列表                                │
│                                              │
│  4. API 调用（仅 3 个端点）                   │
│     - POST /generation/submit（统一提交）      │
│     - GET  /generation/tasks（任务列表）       │
│     - GET  /generation/templates（模板列表）   │
└─────────────────────────────────────────────┘
```

### 3.3 客户端简化要点

**之前（未统一）**：客户端需要知道每个端点的参数格式、模型名称、厂商差异
**现在（已统一）**：客户端只需要 `text` + `materials` + `type`，其余全部自动

```typescript
// 客户端只需要调这一个接口
const result = await submitGeneration({
  brand_id: currentBrand,
  text: "用户输入的文本",
  materials: ["asset-id-1", "asset-id-2"],  // 可选
  type: "audio",  // 可选：video/image/audio/voice
});

// 轮询任务状态
const task = await getGenerationTask(result.id);
if (task.state === 'success') {
  // 展示产物
}
```

**结论：客户端开发者只需关注 UI 布局、交互体验和数据展示，业务逻辑完全由服务端承载。**

## 四、预存问题清单

| # | 问题 | 严重度 | 修复建议 |
|---|---|---|---|
| 1 | `subTypeToCapID` 硬编码在用例层，新增端点需改用例 | 低 | 移到 EndpointRegistry 或配置驱动 |
| 2 | handler 依赖具体 usecase 类型（DIP 轻微违规） | 低 | 引入 GenerationService port 接口 |
| 3 | EndpointSelector 全量加载素材再过滤（性能） | 低 | 添加 FindByIDs 方法到 MediaAssetStore |
| 4 | TemplateUseCase 未装配，模板 API 404 | 中 | main.go 中装配 TemplateUseCase |
| 5 | Vidu text2video 分辨率 720p 校验失败 | 中 | 更新 generation_specs 默认分辨率 |
| 6 | MiMo provider API Key 不支持运行时热更新 | 低 | 实现 ConfigurableProvider 接口 |

## 五、下一步行动计划

### P0 — 立即修复（影响功能可用性）

1. **装配 TemplateUseCase**：在 main.go 中初始化模板仓储和用例，注入 router
2. **修复 Vidu 分辨率默认值**：更新 `generation_specs` 或 `applyDefaults` 中的默认分辨率

### P1 — 架构优化（提升可维护性）

3. **引入 GenerationService port 接口**：在 port 包定义接口，handler 依赖接口而非具体类型
4. **subTypeToCapID 配置化**：将端点→能力映射移到 EndpointRegistry 或 DB 驱动
5. **EndpointSelector 优化**：添加 FindByIDs 到 MediaAssetStore，避免全量加载

### P2 — 功能完善（提升用户体验）

6. **客户端统一提交页面**：基于 UnifiedSubmit API 构建傻瓜式生成页面
7. **管理后台模板管理**：完成 Template CRUD 页面
8. **MiMo API Key 热更新**：实现 ConfigurableProvider 接口
9. **多语言 TTS 测试**：验证不同语言/方言的 TTS 效果

### P3 — 长期演进

10. **新厂商接入流程文档化**：标准化接入指南（实现 GenerationProvider + 注册）
11. **监控与告警**：生成任务成功率/延迟监控
12. **产物 CDN 加速**：音频/视频产物分发优化
