// Package agent 提供 Agent 执行器实现（适配器层）。
//
// 本文件：GraphContentOrchestrator —— 用 trpc-agent-go 的 graphagent 实现
// "框架内容生产"的图编排（探查→生成→校验→条件回退循环）。
//
// 整洁架构定位：graphagent 的所有框架类型（StateGraph/AddNode/AddConditionalEdges）
// 只出现在本文件，usecase 层通过 port.ContentOrchestrator 接口调用，零框架感知。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/graphagent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"

	"webreaper/internal/usecase/port"
)

// GraphContentOrchestrator 是 port.ContentOrchestrator 的 graphagent 实现。
//
// 用图编排（而非单 Explorer）的原因：
//   - 需求要求"校验完整性，不完整则补生成，循环直到完成"。
//   - 单 Explorer 的停止权在 LLM（主观判断），不可靠。
//   - 图编排用条件边做确定性校验（代码比对清单），停止权在代码，可靠可测。
//
// 流程图：scout → generator → validator ─┬─(missing空/失败超限)→ End
//                                        └─(missing非空)→ 回 generator
type GraphContentOrchestrator struct {
	ai      port.AIGenerator     // LLM 调用（scout/generator 内部用）
	tools   []string             // scout 允许的爬虫工具名（爬取文档用）
	logger  port.Logger
	maxFail int                  // 同批 missing 连续补不上来多少次强制结束（安全阀）
}

// 编译期断言：实现 port.ContentOrchestrator。
var _ port.ContentOrchestrator = (*GraphContentOrchestrator)(nil)

// NewGraphContentOrchestrator 创建图编排器。
// tools 是 scout 探查时可用的爬虫工具名（用于爬取框架文档）。
func NewGraphContentOrchestrator(ai port.AIGenerator, tools []string, logger port.Logger) *GraphContentOrchestrator {
	if logger == nil {
		logger = port.NopLogger{}
	}
	if len(tools) == 0 {
		// 默认用 static + search 爬虫探查文档
		tools = []string{"static_crawler", "search_crawler"}
	}
	return &GraphContentOrchestrator{
		ai: ai, tools: tools, logger: logger,
		maxFail: 3,
	}
}

// Orchestrate 执行图编排，返回生成的结构化条目。
func (o *GraphContentOrchestrator) Orchestrate(ctx context.Context, in port.OrchestrateInput, onProgress func(msg string)) ([]port.OrchestrateItem, error) {
	if in.Topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	progress := func(msg string) {
		if onProgress != nil {
			onProgress(msg)
		}
		o.logger.Info("编排进度", port.String("topic", in.Topic), port.String("msg", msg))
	}

	// 用共享 runState 累积结果：节点函数闭包捕获它，图跑完后直接读。
	// 不依赖框架透出 final state（不同版本事件携带 state 的方式不稳定）。
	rs := &runState{}

	// 构建并编译流程图
	g, err := o.buildGraph(in, progress, rs)
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}

	// 包装成 GraphAgent 并执行（WithExecutorOptions 设 maxSteps 安全阀）
	graphAgent, err := graphagent.New("content-orchestrator", g,
		graphagent.WithDescription(fmt.Sprintf("为 %s 生成%s", in.Topic, contentTypeLabel(in.ContentType))),
		graphagent.WithInitialState(graph.State{
			"topic":        in.Topic,
			"content_type": in.ContentType,
			"checklist":    []string{},
			"items":        []any{},
			"missing":      []string{},
			"fail_count":   0,
		}),
		graphagent.WithExecutorOptions(graph.WithMaxSteps(50)),
	)
	if err != nil {
		return nil, fmt.Errorf("create graph agent: %w", err)
	}

	inv := agent.NewInvocation(agent.WithInvocationMessage(model.NewUserMessage(
		fmt.Sprintf("为主题 %s 生成%s，确保覆盖完整", in.Topic, contentTypeLabel(in.ContentType)),
	)))
	events, err := graphAgent.Run(ctx, inv)
	if err != nil {
		return nil, fmt.Errorf("graph run: %w", err)
	}

	// 消费完事件即可（结果已在 rs.items 里）
	for evt := range events {
		if evt.IsError() && evt.Error != nil {
			return rs.items, fmt.Errorf("graph event error: %v", evt.Error)
		}
	}

	progress(fmt.Sprintf("编排完成，共生成 %d 条", len(rs.items)))
	return rs.items, nil
}

// runState 是图编排的运行期共享状态（节点闭包捕获，累积最终结果）。
// 不放进 graph.State（那是框架内部流转用的），而是 orchestrator 本地持有。
type runState struct {
	items        []port.OrchestrateItem // 累积所有生成的条目
	checklist    []string               // scout 提取的模块清单
	qualityGaps  []string               // LLM 质量评审指出的缺口（供 generator 精确补）
	qualityPass  bool                   // LLM 质量评审是否通过
}

// buildGraph 构建并编译流程图。
//
// 分层校验设计（代码快筛 + LLM 质量评审）：
//
//	scout → generator → coverage_validator ─┬─(缺模块)─→ 回 generator（代码判定，免费）
//	       ↑                                 │
//	       │                                 └─(全覆盖)─→ quality_validator (LLM，烧 token)
//	       │                                                  │
//	       └──────(不达标，带 gaps 反馈)──────────────────────┤
//	                        (达标或 failCount 超限)──────────→ End
//
// 两层校验分工：
//   - coverage_validator（纯函数）：快筛模块广度覆盖，缺模块→回 generator，确定性免费。
//   - quality_validator（LLM）：评内容贴切度+需求覆盖度，全覆盖了才跑，省 token。
//     不达标→回 generator 并带 gaps 反馈，精确补（非盲目重生成）。
func (o *GraphContentOrchestrator) buildGraph(in port.OrchestrateInput, progress func(string), rs *runState) (*graph.Graph, error) {
	schema := graph.NewStateSchema().
		AddField("topic", graph.StateField{Type: reflect.TypeOf(""), Reducer: graph.DefaultReducer}).
		AddField("content_type", graph.StateField{Type: reflect.TypeOf(""), Reducer: graph.DefaultReducer}).
		AddField("checklist", graph.StateField{Type: reflect.TypeOf([]string{}), Reducer: graph.StringSliceReducer}).
		AddField("items", graph.StateField{Type: reflect.TypeOf([]any{}), Reducer: graph.AppendReducer}).
		AddField("missing", graph.StateField{Type: reflect.TypeOf([]string{}), Reducer: graph.StringSliceReducer}).
		AddField("fail_count", graph.StateField{Type: reflect.TypeOf(0), Reducer: graph.DefaultReducer}).
		AddField("quality_pass", graph.StateField{Type: reflect.TypeOf(false), Reducer: graph.DefaultReducer}).
		AddField("quality_gaps", graph.StateField{Type: reflect.TypeOf([]string{}), Reducer: graph.StringSliceReducer})

	sg := graph.NewStateGraph(schema)

	// scout 节点：爬取框架文档 + LLM 提取模块清单
	sg.AddNode("scout", func(ctx context.Context, state graph.State) (any, error) {
		topic, _ := state["topic"].(string)
		progress(fmt.Sprintf("正在探查 %s 的模块结构...", topic))

		// 用带工具的 LLM 爬取文档并提取模块清单
		task := fmt.Sprintf(`分析框架 "%s" 的结构。先爬取其官方文档/README，然后列出该框架所有核心模块（如 agent、tool、model、graph 等）。
返回 JSON 数组，每项是一个模块名（字符串），如 ["agent","tool","model"]。只返回 JSON，不要其他文字。`, topic)
		var sb strings.Builder
		err := o.ai.RunWithTools(ctx, "", "", task,
			"你是框架结构分析助手。用爬虫工具获取真实文档后，提取模块清单。只返回 JSON 数组。",
			o.tools,
			func(e port.ToolEvent) {
				if e.Type == "text-delta" {
					sb.WriteString(e.Text)
				}
			},
		)
		if err != nil {
			return nil, fmt.Errorf("scout run: %w", err)
		}

		checklist := extractStringList(sb.String())
		if len(checklist) == 0 {
			// LLM 没提取出清单，用通用清单兜底
			checklist = []string{"core", "config", "usage"}
		}
		rs.checklist = checklist // 存本地（validator 也能从 state 读，但本地更可靠）
		progress(fmt.Sprintf("识别到 %d 个模块：%s", len(checklist), strings.Join(checklist, ", ")))
		return graph.State{
			"checklist": checklist,
			"missing":   checklist, // 初始全部 missing，触发首轮生成
		}, nil
	})

	// generator 节点：针对 missing 模块生成题目，若有质量反馈则精确补缺口
	sg.AddNode("generator", func(ctx context.Context, state graph.State) (any, error) {
		topic, _ := state["topic"].(string)
		contentType, _ := state["content_type"].(string)
		missing, _ := state["missing"].([]string)
		// 质量评审反馈（来自上一轮 quality_validator 不达标时）
		qualityGaps, _ := state["quality_gaps"].([]string)

		// 确定本轮要生成什么：优先补质量缺口，否则按 missing 模块生成
		focus := missing
		focusLabel := fmt.Sprintf("%d 个缺失模块", len(missing))
		if len(qualityGaps) > 0 {
			// 质量不达标：把 LLM 指出的缺口也纳入 focus，精确补
			focus = append(focus, qualityGaps...)
			focusLabel = fmt.Sprintf("%d 个模块 + %d 个质量缺口", len(missing), len(qualityGaps))
		}
		if len(focus) == 0 {
			return graph.State{}, nil
		}
		progress(fmt.Sprintf("正在为 %s 生成%s...", focusLabel, contentTypeLabel(contentType)))

		// 构建 prompt：基础模块 + 质量反馈（若有）
		prompt := fmt.Sprintf(`为主题 "%s" 生成%s，针对以下重点，每个生成 1-2 道，含题目和参考答案：
重点：%s

返回 JSON 数组，每项格式：{"title":"题目标题","content":"题目+答案","module":"模块名","tags":["标签"]}
只返回 JSON 数组，不要其他文字。`,
			topic, contentTypeLabel(contentType), strings.Join(focus, ", "))
		if len(qualityGaps) > 0 {
			prompt = fmt.Sprintf(`为主题 "%s" 生成%s。上一轮质量评审发现以下缺口，请重点补全：
缺口：%s

要求每道题的内容必须真实对应其所标模块。返回 JSON 数组，每项格式：{"title":"题目标题","content":"题目+答案","module":"模块名","tags":["标签"]}
只返回 JSON 数组。`,
				topic, contentTypeLabel(contentType), strings.Join(qualityGaps, "; "))
		}
		resp, err := o.ai.ChatStream(ctx, "", "", []port.ChatMessage{
			{Role: "system", Content: "你是技术面试题生成专家。只返回 JSON 数组。"},
			{Role: "user", Content: prompt},
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("generator llm: %w", err)
		}

		newItems := decodeItemsJSON(resp)
		progress(fmt.Sprintf("生成了 %d 条，准备校验完整性", len(newItems)))
		// 累加到本地 rs.items（这是最终返回的权威结果）
		rs.items = append(rs.items, newItems...)
		// 清空本轮消费的质量反馈（避免重复补），同时写 state.items 供 coverage_validator 比对
		rs.qualityGaps = nil
		anySlice := make([]any, 0, len(newItems))
		for _, it := range newItems {
			anySlice = append(anySlice, it)
		}
		return graph.State{
			"items":        anySlice,
			"quality_gaps": []string{}, // 清空，下轮重新评
		}, nil
	})

	// coverage_validator 节点：纯函数，比对 checklist vs items，算模块覆盖（第一层快筛，免费）
	sg.AddNode("coverage_validator", func(ctx context.Context, state graph.State) (any, error) {
		checklist := rs.checklist
		failCount, _ := state["fail_count"].(int)

		missing, covered := computeMissing(checklist, rs.items)
		progress(fmt.Sprintf("覆盖校验：已覆盖 %d/%d 模块", len(covered), len(checklist)))

		// fail_count：若 missing 数量没减少（这轮补生成无效），计数+1
		newFail := failCount
		prevMissing, _ := state["missing"].([]string)
		if len(prevMissing) > 0 && len(missing) >= len(prevMissing) {
			newFail = failCount + 1
		}
		return graph.State{
			"missing":    missing,
			"fail_count": newFail,
		}, nil
	})

	// quality_validator 节点：LLM 评审内容贴切度 + 需求覆盖度（第二层，烧 token）
	//
	// 只在 coverage_validator 判定模块全覆盖后才执行——避免对明显残缺的内容白烧 token。
	// 输出结构化：{pass:bool, gaps:[具体缺什么]}，gaps 供 generator 精确补。
	sg.AddNode("quality_validator", func(ctx context.Context, state graph.State) (any, error) {
		topic, _ := state["topic"].(string)
		contentType, _ := state["content_type"].(string)
		failCount, _ := state["fail_count"].(int)

		progress("质量评审：检查内容贴切度与需求覆盖度...")
		// 把已生成的题目摘要发给 LLM 评审
		itemsSummary := itemsToSummary(rs.items)
		prompt := fmt.Sprintf(`你是面试题质量评审专家。请评审以下为 "%s" 生成的%s 是否满足要求：
1. 每道题的内容是否真的对应其所标模块（防跑题）
2. 整体是否覆盖了 "%s" 的核心知识点
3. 是否有明显遗漏的关键内容

题目清单：
%s

返回 JSON：{"pass": true/false, "gaps": ["具体缺什么1", "缺什么2"]}
pass=true 表示质量达标；pass=false 时 gaps 列出具体缺口（供补全）。
只返回 JSON。`,
			topic, contentTypeLabel(contentType), topic, itemsSummary)
		resp, err := o.ai.ChatStream(ctx, "", "", []port.ChatMessage{
			{Role: "system", Content: "你是质量评审专家。只返回 JSON。"},
			{Role: "user", Content: prompt},
		}, nil)
		if err != nil {
			// LLM 评审失败不阻断（降级为通过，让流程继续——已有内容仍有价值）
			progress(fmt.Sprintf("质量评审 LLM 调用失败，降级通过: %v", err))
			return graph.State{"quality_pass": true, "quality_gaps": []string{}}, nil
		}

		pass, gaps := parseQualityResult(resp)
		rs.qualityPass = pass
		rs.qualityGaps = gaps
		if pass {
			progress("质量评审通过：内容贴切且覆盖需求")
		} else {
			progress(fmt.Sprintf("质量评审未通过，发现 %d 个缺口，将补生成", len(gaps)))
			// 质量不达标计入 fail_count（避免 LLM 反复挑刺死循环）
			return graph.State{
				"quality_pass": false,
				"quality_gaps": gaps,
				"fail_count":   failCount + 1,
			}, nil
		}
		return graph.State{"quality_pass": true, "quality_gaps": []string{}}, nil
	})

	// 边：scout → generator → coverage_validator
	sg.AddEdge("scout", "generator")
	sg.AddEdge("generator", "coverage_validator")

	// 条件边①：coverage_validator → 缺模块回 generator / 全覆盖进质量评审
	sg.AddConditionalEdges("coverage_validator", func(ctx context.Context, state graph.State) (graph.ConditionResult, error) {
		missing, _ := computeMissing(rs.checklist, rs.items)
		failCount, _ := state["fail_count"].(int)
		if len(missing) == 0 {
			// 模块全覆盖 → 进入质量评审（不直接结束）
			return graph.ConditionResult{NextNodes: []string{"quality_validator"}}, nil
		}
		if failCount >= o.maxFail {
			progress(fmt.Sprintf("连续 %d 轮未能补全模块，强制结束", failCount))
			return graph.ConditionResult{NextNodes: []string{graph.End}}, nil
		}
		return graph.ConditionResult{NextNodes: []string{"generator"}}, nil
	}, map[string]string{
		graph.End:           graph.End,
		"quality_validator": "quality_validator",
		"generator":         "generator",
	})

	// 条件边②：quality_validator → 达标结束 / 不达标回 generator（带 gaps） / 超限结束
	sg.AddConditionalEdges("quality_validator", func(ctx context.Context, state graph.State) (graph.ConditionResult, error) {
		failCount, _ := state["fail_count"].(int)
		if rs.qualityPass {
			progress("编排完成：模块覆盖且质量达标")
			return graph.ConditionResult{NextNodes: []string{graph.End}}, nil
		}
		if failCount >= o.maxFail {
			progress(fmt.Sprintf("质量评审连续 %d 轮未达标，强制结束（已生成内容仍返回）", failCount))
			return graph.ConditionResult{NextNodes: []string{graph.End}}, nil
		}
		// 不达标 → 回 generator，gaps 已在 state.quality_gaps / rs.qualityGaps 里
		return graph.ConditionResult{NextNodes: []string{"generator"}}, nil
	}, map[string]string{
		graph.End:   graph.End,
		"generator": "generator",
	})

	sg.SetEntryPoint("scout")
	// 不对 graph.End 设 SetFinishPoint——End 是特殊终点标识，不是普通节点。
	// validator 的条件边已负责路由到 End，流程会自然终止。

	compiled, err := sg.Compile()
	if err != nil {
		return nil, fmt.Errorf("compile graph: %w", err)
	}
	return compiled, nil
}

// computeMissing 比对清单与已生成条目，返回（缺失模块, 已覆盖模块）。
//
// 这是确定性清单校验的核心——纯函数，不调 LLM，可单测。
// 判定规则：某模块在 items 的 Module 或 Tags 中出现，视为已覆盖。
func computeMissing(checklist []string, items []port.OrchestrateItem) (missing, covered []string) {
	coveredSet := make(map[string]bool)
	for _, mod := range checklist {
		for _, it := range items {
			if strings.EqualFold(it.Module, mod) || containsTag(it.Tags, mod) {
				coveredSet[mod] = true
				break
			}
		}
	}
	for _, mod := range checklist {
		if coveredSet[mod] {
			covered = append(covered, mod)
		} else {
			missing = append(missing, mod)
		}
	}
	return missing, covered
}

// containsTag 判断 tags 中是否包含指定（大小写不敏感）。
func containsTag(tags []string, target string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, target) {
			return true
		}
	}
	return false
}

// extractStringList 从 LLM 文本中提取 JSON 字符串数组（["a","b"]）。
// 容错：LLM 可能包裹 markdown 或多余文字。
func extractStringList(s string) []string {
	jsonStr := extractJSONArray(s)
	var list []string
	if err := json.Unmarshal([]byte(jsonStr), &list); err != nil {
		return nil
	}
	// 去重 + 去空
	seen := make(map[string]bool)
	result := make([]string, 0, len(list))
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// decodeItemsJSON 从 LLM 文本解析 OrchestrateItem 数组。
func decodeItemsJSON(s string) []port.OrchestrateItem {
	jsonStr := extractJSONArray(s)
	var items []port.OrchestrateItem
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return []port.OrchestrateItem{}
	}
	return items
}

// extractJSONArray 从可能含 markdown/多余文字的文本中提取首个 JSON 数组 [...]。
func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	if start < 0 {
		return "[]"
	}
	// 从末尾找最后一个 ]
	end := strings.LastIndex(s, "]")
	if end < 0 || end <= start {
		return "[]"
	}
	return s[start : end+1]
}

// contentTypeLabel 内容类型的中文标签（用于 prompt 和进度展示）。
func contentTypeLabel(t string) string {
	switch t {
	case "interview_questions":
		return "面试题"
	case "knowledge_summary":
		return "知识点总结"
	default:
		if t == "" {
			return "内容"
		}
		return t
	}
}

// itemsToSummary 把题目列表摘要成文本（供质量评审 LLM 看）。
// 截断每条内容，避免 prompt 过长烧 token。
func itemsToSummary(items []port.OrchestrateItem) string {
	var sb strings.Builder
	for i, it := range items {
		sb.WriteString(fmt.Sprintf("%d. [模块:%s] %s\n   内容摘要: %s\n",
			i+1, it.Module, it.Title, truncateStr(it.Content, 150)))
	}
	if sb.Len() == 0 {
		return "（无题目）"
	}
	return sb.String()
}

// truncateStr 截断字符串（节点 helper 用，避免与 graphagent 内部冲突）。
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// qualityResult 是 LLM 质量评审的结构化输出。
type qualityResult struct {
	Pass bool     `json:"pass"`
	Gaps []string `json:"gaps"`
}

// parseQualityResult 解析 LLM 质量评审输出，返回 (是否通过, 缺口列表)。
//
// 容错：LLM 可能包裹 markdown 或多余文字，先提取 JSON 对象再解析。
// 解析失败时保守返回 pass=true（降级通过，不阻断流程——已有内容仍有价值）。
func parseQualityResult(s string) (bool, []string) {
	jsonStr := extractJSONObject(s)
	var qr qualityResult
	if err := json.Unmarshal([]byte(jsonStr), &qr); err != nil {
		// 解析失败，保守判通过（避免因 LLM 输出格式问题卡死流程）
		return true, nil
	}
	if qr.Gaps == nil {
		qr.Gaps = []string{}
	}
	return qr.Pass, qr.Gaps
}

// extractJSONObject 从可能含 markdown/多余文字的文本中提取首个 JSON 对象 {...}。
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return "{}"
	}
	// 用括号配平找匹配的 }（处理嵌套）
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:] // 未配平，返回到最后
}
