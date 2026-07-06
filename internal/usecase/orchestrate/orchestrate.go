// Package orchestrate 实现"框架内容编排"用例。
//
// 职责：接收主题 → 委托 ContentOrchestrator 生成结构化内容 → 落库（待审核，不推送）。
//
// 整洁架构定位（应用级业务编排）：
//   - 本用例只依赖 port.ContentOrchestrator 接口和 port.DataItemRepository 接口，
//     不知道具体是 graphagent 还是别的实现。
//   - "编排如何实现"（图/单 Agent）是 adapter 层的事，本用例只关心业务流程：
//     生成 → 落库为待审核状态 → 返回 ID 供前端追踪。
package orchestrate

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// OrchestratorUseCase 框架内容编排用例。
type OrchestratorUseCase struct {
	orchestrator port.ContentOrchestrator
	itemRepo     port.DataItemRepository
	logger       port.Logger
}

func NewOrchestratorUseCase(orchestrator port.ContentOrchestrator, itemRepo port.DataItemRepository, logger port.Logger) *OrchestratorUseCase {
	if logger == nil {
		logger = port.NopLogger{}
	}
	return &OrchestratorUseCase{
		orchestrator: orchestrator,
		itemRepo:     itemRepo,
		logger:       logger.With(port.String("component", "orchestrate")),
	}
}

// OrchestrateInput 用例输入（HTTP 层 DTO 可直接映射）。
type OrchestrateInput struct {
	Topic       string // 主题，如 "trpc-agent-go 框架"
	ContentType string // "interview_questions" / "knowledge_summary"
}

// OrchestrateOutput 用例输出。
type OrchestrateOutput struct {
	ItemIDs []string // 落库后的 DataItem ID 列表（供前端追踪/审核）
	Count   int      // 生成条数
}

// Execute 执行编排：生成内容 → 逐条落库（待审核状态，不推送）。
//
// onProgress 透传给编排器，供调用方（如 SSE 端点）实时上报进度。
func (uc *OrchestratorUseCase) Execute(ctx context.Context, in OrchestrateInput, onProgress func(msg string)) (OrchestrateOutput, error) {
	if in.Topic == "" {
		return OrchestrateOutput{}, fmt.Errorf("topic is required")
	}
	if uc.orchestrator == nil {
		return OrchestrateOutput{}, fmt.Errorf("编排器未配置（ContentOrchestrator 为 nil）")
	}

	uc.logger.Info("开始编排",
		port.String("topic", in.Topic),
		port.String("content_type", in.ContentType))

	// 1. 委托编排器生成结构化内容
	items, err := uc.orchestrator.Orchestrate(ctx, port.OrchestrateInput{
		Topic:       in.Topic,
		ContentType: in.ContentType,
	}, onProgress)
	if err != nil {
		return OrchestrateOutput{}, fmt.Errorf("orchestrate: %w", err)
	}

	// 2. 逐条落库为 DataItem（待审核状态，不推送——用户后续手动审核推送）
	now := time.Now()
	ids := make([]string, 0, len(items))
	for _, it := range items {
		item := entity.DataItem{
			ID:        fmt.Sprintf("orch-%d-%d", now.UnixNano(), len(ids)),
			Title:     it.Title,
			Content:   it.Content,
			Tags:      it.Tags,
			SourceURL: "", // 编排生成的内容无外部来源 URL
			RawContent: it.Content,
			Status:    entity.ItemStatusPendingReview, // 待审核
			Metadata: map[string]string{
				"source":       "orchestrate",
				"module":       it.Module,
				"content_type": in.ContentType,
				"topic":        in.Topic,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := uc.itemRepo.Save(ctx, item); err != nil {
			// 单条落库失败不阻断整体（记录日志，继续其余条目）
			uc.logger.Warn("单条内容落库失败",
				port.String("title", it.Title),
				port.Err(err))
			continue
		}
		ids = append(ids, item.ID)
	}

	uc.logger.Info("编排完成并落库", port.Int("count", len(ids)))
	return OrchestrateOutput{ItemIDs: ids, Count: len(ids)}, nil
}
