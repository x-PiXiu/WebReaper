// compose_dispatch.go B-Roll compose 类型的统一提交分发（22 号计划）。
// 编排实现在 usecase/videocompose（port.Composer 接口注入）——本文件只做参数适配。
package generation

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// submitCompose compose 类型分发：统一提交参数 → port.ComposeInput → videocompose。
func (uc *GenerationUseCase) submitCompose(ctx context.Context, in UnifiedSubmitInput) (entity.GenerationTask, error) {
	if uc.composer == nil {
		return entity.GenerationTask{}, fmt.Errorf("B-Roll 合成未启用（compose 服务未配置）")
	}
	if in.Params == nil {
		return entity.GenerationTask{}, fmt.Errorf("缺少 compose 参数（source_task_id / segments）")
	}
	srcID, _ := in.Params["source_task_id"].(string)
	if srcID == "" {
		return entity.GenerationTask{}, fmt.Errorf("缺少 source_task_id（源成片任务 ID）")
	}
	var segs []port.ComposeSegment
	if raw, ok := in.Params["segments"].([]any); ok {
		for _, r := range raw {
			if m, ok2 := r.(map[string]any); ok2 {
				idx, _ := m["sentence_index"].(float64)
				url, _ := m["media_url"].(string)
				segs = append(segs, port.ComposeSegment{
					SentenceIndex: int(idx), MediaURL: url,
				})
			}
		}
	}
	if len(segs) == 0 {
		return entity.GenerationTask{}, fmt.Errorf("缺少插入片段（segments 数组）")
	}

	res, err := uc.composer.SubmitCompose(ctx, port.ComposeInput{
		TenantID:     in.TenantID,
		BrandID:      in.BrandID,
		SourceTaskID: srcID,
		Segments:     segs,
	})
	if err != nil {
		return entity.GenerationTask{}, err
	}
	// compose 任务由 videocompose 创建——回读返回给统一提交调用方（保持返回类型一致）
	return uc.repo.FindByID(ctx, in.TenantID, res.TaskID)
}
