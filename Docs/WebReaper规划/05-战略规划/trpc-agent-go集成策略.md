# trpc-agent-go 集成策略

> 📌 trpc-agent-go 是完整 Agent 框架（含图工作流/工具调用/Memory/多 Agent 协作），
> WebReaper 当前只用了最基础的"LLM 文本生成"能力。本文档分析如何按需逐步释放框架价值。

## 一、现状诊断：用了多少

trpc-agent-go 能力清单 vs WebReaper 当前使用：

| 框架能力 | WebReaper 是否使用 | 说明 |
|---|---|---|
| LLM 文本生成（chat completion） | ✅ 使用 | 生成面试题/总结知识点 |
| 流式响应（stream events） | ✅ 使用 | 拼接 Delta.Content |
| OpenAI 兼容端点（WithBaseURL） | ✅ 使用 | 接入 MiniMax |
| **工具调用（Tool Calling）** | ❌ 未用 | 框架支持，但当前场景不需要 |
| **图工作流（Graph Workflow）** | ❌ 未用 | 当前用例是线性编排，不需要 DAG |
| **Session/Memory** | ❌ 未用 | 无状态单轮调用 |
| **多 Agent 协作** | ❌ 未用 | 当前单 Agent 够用 |
| **RAG 知识检索** | ❌ 未用 | 向量库未接入 |
| **OpenTelemetry** | ❌ 未用 | 框架内置，但项目未接入 OTel |

**使用率约 30%**。这并非浪费——YAGNI 原则下，当前业务只需要这 30%。

## 二、整洁架构核心判断：框架放哪层

```
            ┌─────────────────────────────────────┐
            │      框架与驱动（Frameworks）        │
            │  trpc-agent-go 在这里 ← 必须最外层   │
            │  ┌───────────────────────────────┐  │
            │  │    接口适配器（Adapters）       │  │
            │  │  adapter/ai/TrpcAgentGenerator │  │
            │  │  ┌─────────────────────────┐  │  │
            │  │  │   用例（Use Cases）       │  │  │
            │  │  │  只认 port.AIGenerator   │  │  │
            │  │  │  ┌───────────────────┐  │  │  │
            │  │  │  │  实体（Entities）   │  │  │  │
            │  │  │  │  不知道 trpc 存在  │  │  │  │
            │  │  │  └───────────────────┘  │  │  │
            │  │  └─────────────────────────┘  │  │
            │  └───────────────────────────────┘  │
            └─────────────────────────────────────┘
```

**铁律**：trpc-agent-go 只出现在 `adapter/ai/`，用例层永远只依赖 `port.AIGenerator` 接口。

## 三、按需释放框架能力的四阶段

### 阶段 1：当前（文本生成）✅ 已完成
- 单轮 chat completion
- MiniMax 通过 WithBaseURL 接入
- Prompt 工程在适配器层

### 阶段 2：启用工具调用（中期）
**触发条件**：需要让 AI 主动查询数据库（如"生成面试题前先查该公司历史题目避免重复"）

```
用例 → AIGenerator.GenerateQuestions(带工具)
         ↓
     trpc-agent-go Agent 调用 LLM
         ↓ LLM 决定调用工具
     Tool: QueryExistingQuestions（由 WebReaper 实现）
         ↓ 返回结果给 LLM
     LLM 生成去重后的面试题
```

**关键**：工具实现归 WebReaper 业务模块，框架只做调度。工具本身也通过 port 接口注入。

### 阶段 3：多 Agent 协作（远期）
**触发条件**：需要"出题 Agent + 审核 Agent"分工

```
出题 Agent → 生成面试题草稿
    ↓
审核 Agent → 校验难度/准确性/去重
    ↓
定稿入库
```

此时 `AIGenerator` 接口扩展为 `AgentOrchestrator`，但仍由 port 定义、adapter 实现。

### 阶段 4：Memory + RAG（远期）
**触发条件**：知识点库需要语义检索增强生成

```
用例 → AIGenerator.GenerateQuestions(带 RAG)
         ↓
     trpc-agent-go 从 Milvus 检索相关知识
         ↓
     LLM 基于检索结果生成更精准的面试题
```

## 四、三个不要做

| 不要做 | 理由 |
|---|---|
| ❌ 不让用例层 import trpc-agent-go | 破坏依赖方向，框架升级时用例受牵连 |
| ❌ 不为"可能用到"的高级特性提前抽象 | YAGNI，等需求来了再加 |
| ❌ 不让 trpc-agent-go 的数据类型泄露到 domain | 用 domain 实体，框架类型只在 adapter 内部 |

## 五、优先级行动清单

| 行动 | 优先级 | 说明 |
|---|---|---|
| 接入 OpenTelemetry（框架已内置）| 🟡 P1 | 让 LLM 调用有 trace，监控 Token 消耗 |
| Prompt 版本管理 | 🟡 P1 | Prompt 迭代不应靠改代码 |
| 工具调用（查重工具）| 🟢 P2 | 让 AI 自主去重 |
| 多 Agent（出题+审核）| ⚪ P3 | 等单 Agent 质量稳定后 |

## 六、总结

> **不是框架有什么用什么，而是业务需要什么让框架实现什么。**

trpc-agent-go 是强大的工具箱，但 WebReaper 只取当前所需（30%）。剩余 70% 的能力留在框架里，等业务真正需要时再按整洁架构的方式（接口隔离+适配器实现）逐步释放。这正是"推迟决策"原则的体现。

---

> 📎 **关联文档**：[数据采集差异化战略](数据采集差异化战略.md) | [04-功能详解/02-面试题生成](../04-功能详解/02-面试题生成.md)
