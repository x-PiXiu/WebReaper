# 多 Agent 编排设计（WebReaper）

> 状态：**设计阶段，未实现**。本文档指导未来实现，遵循"先简单后复杂、推迟决策"原则。
> 参考：架构师手册 § 整洁架构「边界是有成本的，按需选择厚度」。

## 一、为什么现在不实现

当前是单 Agent（explorer）+ 全局工具集，已能满足"采集→总结"的核心场景。
多 Agent 编排的复杂度高（DAG、条件分支、状态共享），在需求未明确前实现，
是典型的过度设计（违反 YAGNI）。

**触发实现的信号**（出现任一即启动）：
- 用户明确需要"先采集→再分析→最后生成报告"这类多步骤、多角色协作
- 单 Agent 的 ReAct 循环无法满足（如需要两个不同 LLM 协作）
- 同一任务需要并行采集多个站点后汇总

## 二、架构推导（五步分析法）

| 维度 | 分析 |
|---|---|
| 核心要求 | 把多个 Agent 按拓扑（顺序/并行/分支）组织成一条流水线 |
| 变化因子 | 拓扑结构会变、Agent 角色会新增、编排引擎可能换 |
| 多态信号 | 每个 Agent 是一个可执行节点，输入/输出可衔接 |
| 隔离壁 | 编排逻辑不该焊死在 usecase，未来可能换框架 |

**推导结论**：**先用现有任务队列做"伪编排"（任务链），不引入编排框架**。
- 一个"编排"= 一串有序的异步任务，前一个的 output 作为后一个的 input
- 复用现有的 task queue + dispatch + AgentRunner
- 等真正需要 DAG（条件分支、循环、并行汇合）时再引入编排引擎

## 三、分阶段实施路径

### 阶段 1：任务链（TaskChain）—— 最小可用
**目标**：支持"A 完成后自动触发 B"，B 的输入 = A 的输出。

**实现**：
```
领域层：Task 加 NextTaskID 字段（链表）
用例层：worker 完成任务后，若有 NextTaskID 且当前成功，自动投递下一个任务
配置：Agent 配置页可设置"完成后触发 Agent X"
```

**优点**：零新框架，纯增量；复用现有 worker/queue。
**局限**：只支持线性链，不支持并行/分支。

### 阶段 2：编排模板（Pipeline）—— 声明式 DAG
**目标**：用户在 UI 定义"采集员→分析师→审核员"的 DAG。

**实现**：
```
领域层：Pipeline{ID, Nodes[]PipelineNode, Edges[]}（节点=Agent引用，边=数据流向）
用例层：PipelineExecutor 按 DAG 调度（拓扑排序 + 节点间数据传递）
端口层：保留 AgentRunner 接口不变，Pipeline 编排多个 AgentRunner
```

**关键决策点**（届时再定）：
- 自研轻量调度器 vs 引入现成框架（如不用 LangGraph，太重）
- 数据在节点间如何传递（内存 context vs 持久化中间结果）
- 失败策略（一个节点失败，整条 pipeline 怎么办）

### 阶段 3：可视化编排（未来）
UI 拖拽式 DAG 编辑器（类似 n8n/Dify）。这一步工程量大，需求不紧迫不做。

## 四、与现有架构的关系

```
现有：单 Agent ReAct
  Chat → AIGenerator.RunWithTools → explorer(LLM+工具) → 单轮回复

阶段1：任务链
  EnqueueUseCase → Task(Agent1) → [worker完成] → Task(Agent2) → ...
  （前一个的 output JSON 作为后一个的 input）

阶段2：Pipeline
  PipelineExecutor → 拓扑排序 → 并行/串行调度多个 AgentRunner
```

**依赖方向不变**：Pipeline/PipelineExecutor 在 usecase 层，
依赖 port.AgentRunner（已存在），不依赖 adapter 具体实现。

## 五、接口预留（本轮不做，仅记录方向）

```go
// 未来 usecase/pipeline/pipeline.go（现在不创建）
type Pipeline struct {
    Nodes []PipelineNode   // 每个 node 引用一个 AgentConfig
    Edges []PipelineEdge   // 数据流向
}
type PipelineExecutor struct { /* 按 DAG 调度 */ }
```

**当前 AgentConfig 已具备编排前提**：有 SystemPrompt（定义角色）、LLMConfigName（不同 Agent 用不同模型）、Tools（能力边界）。未来编排只需在上层组合，无需改 AgentConfig 本身。

## 六、建议的下一步

1. **先收集真实编排需求**：观察用户是否真的需要多 Agent，还是单 Agent + 好的提示词就够
2. **若要推进**，从阶段 1（任务链）开始，工作量小、价值可验证
3. **不要过早引入编排框架**：当前的 trpc-agent-go explorer 已经是一个"内编排"（LLM 自主决定调工具），很多场景不需要外编排

---

> Robert C. Martin：好架构让你能"推迟决策"。多 Agent 编排正是这种决策——
> 现在的架构已经为它留好了路（AgentRunner 接口、任务队列、Agent 配置），
> 但不急于实现，等需求和信息均衡时再做。
