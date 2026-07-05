package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// 编译期断言：确保实现 port.CrawlerTool（含全部 4 个方法）。
var _ port.CrawlerTool = (*KnowledgeSearcher)(nil)

// KnowledgeSearcher 是知识检索工具（Agent 可调用，搜索已采集的知识库）。
// 让 Agent 能"回忆"之前采集的信息——这是数据闭环的"检索"环节。
type KnowledgeSearcher struct {
	searcher port.KnowledgeSearcher
}

// NewKnowledgeSearcher 创建知识检索工具。
// searcher 由 usecase/process.ProcessUseCase 实现（main 装配时注入）。
func NewKnowledgeSearcher(searcher port.KnowledgeSearcher) *KnowledgeSearcher {
	return &KnowledgeSearcher{searcher: searcher}
}

func (k *KnowledgeSearcher) Name() string { return "knowledge_search" }

func (k *KnowledgeSearcher) Description() string {
	return "搜索已采集的知识库（之前 Agent 采集并审核通过的数据）。输入参数：query（搜索关键词）、top_k（返回条数，默认5）。" +
		"当用户问的问题可能已经有采集过的知识时，优先用这个工具搜索。"
}

type knowledgeSearchArgs struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

func (k *KnowledgeSearcher) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args knowledgeSearchArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Query == "" {
		return entity.DataItem{}, fmt.Errorf("query is required")
	}
	if args.TopK <= 0 {
		args.TopK = 5
	}

	results, err := k.searcher.SearchKnowledge(ctx, args.Query, args.TopK)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("search knowledge: %w", err)
	}

	// 格式化搜索结果
	resultStr := ""
	for i, r := range results {
		resultStr += fmt.Sprintf("%d. [score=%.2f] %s\n", i+1, r.Score, r.Metadata["title"])
	}
	if resultStr == "" {
		resultStr = "未找到相关知识"
	}

	return entity.DataItem{
		ID:        fmt.Sprintf("knowledge-%d", time.Now().UnixNano()),
		Title:     fmt.Sprintf("知识搜索: %s", args.Query),
		Content:   resultStr,
		SourceURL: "knowledge://search",
		Status:    entity.ItemStatusApproved,
		Metadata:  map[string]string{"tool_type": "knowledge_search", "results_count": fmt.Sprintf("%d", len(results))},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (k *KnowledgeSearcher) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "knowledge_search",
		Description: k.Description(),
		Properties: map[string]port.PropSpec{
			"query": {Type: "string", Description: "搜索关键词（必填）"},
			"top_k": {Type: "integer", Description: "返回条数，默认5"},
		},
		Required: []string{"query"},
	}
}
