package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// Publisher 封装推送能力（供 Agent 工具调用）。
// 由 usecase/publish.PublishUseCase 实现此接口，main 装配时注入。
//
// 设计动机（DIP）：crawler 包不依赖 usecase/publish 包（避免 adapter→usecase 横向耦合），
// 由 port 接口隔离；main 装配时把 PublishUseCase 适配为此接口注入。
type Publisher interface {
	// PublishTo 把指定 DataItem 推送到目标系统。
	PublishTo(ctx context.Context, dataItemID, systemName string) (PublishResult, error)
	// ListSystems 列出可用的外部系统。
	ListSystems(ctx context.Context) ([]SystemInfo, error)
}

// PublishResult 推送结果。
type PublishResult struct {
	Success    bool
	ExternalID string
	Error      string
}

// SystemInfo 外部系统摘要（Agent 可读）。
type SystemInfo struct {
	Name        string
	Description string
	ContentType string
	Endpoint    string
}

// PublishTool 把"推送到外部系统"封装为 Agent 工具（CrawlerTool 接口）。
// Agent 可在 ReAct 循环中自主调用，把采集的数据推送到外部系统。
type PublishTool struct {
	publisher Publisher
}

// NewPublishTool 创建推送工具。
func NewPublishTool(publisher Publisher) *PublishTool {
	return &PublishTool{publisher: publisher}
}

func (t *PublishTool) Name() string { return "publish_to_external" }
func (t *PublishTool) Description() string {
	return "把已采集并审核通过的数据项推送到外部系统。" +
		"输入参数：data_item_id（数据项ID，必填）、system_name（目标系统名，必填）。" +
		"用 list_external_systems 工具可查看可用的目标系统。"
}

type publishToolArgs struct {
	DataItemID string `json:"data_item_id"`
	SystemName string `json:"system_name"`
}

func (t *PublishTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args publishToolArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.DataItemID == "" || args.SystemName == "" {
		return entity.DataItem{}, fmt.Errorf("data_item_id 和 system_name 都是必填")
	}

	result, err := t.publisher.PublishTo(ctx, args.DataItemID, args.SystemName)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("推送失败: %w", err)
	}

	status := "成功"
	if !result.Success {
		status = "失败：" + result.Error
	}
	content := fmt.Sprintf("推送结果：%s\n目标系统：%s\n数据项：%s\n外部ID：%s",
		status, args.SystemName, args.DataItemID, result.ExternalID)

	return entity.DataItem{
		ID:        fmt.Sprintf("publish-%d", time.Now().UnixNano()),
		Title:     fmt.Sprintf("推送 %s → %s", args.DataItemID, args.SystemName),
		Content:   content,
		SourceURL: "publish://external",
		Status:    entity.ItemStatusApproved,
		Metadata: map[string]string{
			"tool_type": "publish", "system": args.SystemName,
			"success": fmt.Sprintf("%v", result.Success),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (t *PublishTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "publish_to_external",
		Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"data_item_id": {Type: "string", Description: "要推送的数据项ID（必填）"},
			"system_name":  {Type: "string", Description: "目标外部系统名（必填，可用 list_external_systems 查看）"},
		},
		Required: []string{"data_item_id", "system_name"},
	}
}

// 编译期断言。
var _ port.CrawlerTool = (*PublishTool)(nil)

// ListExternalSystemsTool 把"查询可用外部系统"封装为 Agent 工具。
type ListExternalSystemsTool struct {
	publisher Publisher
}

func NewListExternalSystemsTool(publisher Publisher) *ListExternalSystemsTool {
	return &ListExternalSystemsTool{publisher: publisher}
}

func (t *ListExternalSystemsTool) Name() string { return "list_external_systems" }
func (t *ListExternalSystemsTool) Description() string {
	return "列出所有可用的外部推送系统（名称、描述、接收的数据类型）。" +
		"在调用 publish_to_external 之前先用此工具了解可推送的目标系统。输入参数：无。"
}

type listSystemsArgs struct{}

func (t *ListExternalSystemsTool) Execute(ctx context.Context, _ string) (entity.DataItem, error) {
	systems, err := t.publisher.ListSystems(ctx)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("list systems: %w", err)
	}

	content := fmt.Sprintf("共 %d 个可用外部系统：\n\n", len(systems))
	for i, s := range systems {
		content += fmt.Sprintf("%d. %s（%s）\n   类型: %s\n   端点: %s\n",
			i+1, s.Name, s.Description, s.ContentType, s.Endpoint)
	}
	if content == "" {
		content = "暂无可用外部系统。请先在「外部系统」页面配置。"
	}

	return entity.DataItem{
		ID:        fmt.Sprintf("syslist-%d", time.Now().UnixNano()),
		Title:     "可用外部系统列表",
		Content:   content,
		SourceURL: "list://external-systems",
		Status:    entity.ItemStatusApproved,
		Metadata:  map[string]string{"tool_type": "list_systems", "count": fmt.Sprintf("%d", len(systems))},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (t *ListExternalSystemsTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "list_external_systems",
		Description: t.Description(),
		Properties:  map[string]port.PropSpec{},
		Required:    []string{},
	}
}

var _ port.CrawlerTool = (*ListExternalSystemsTool)(nil)
