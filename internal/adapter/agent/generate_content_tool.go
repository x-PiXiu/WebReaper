package agent

import (
	"context"
	"fmt"

	"webreaper/internal/usecase/generation"
)

// GenerateContentTool 生成内容工具（智能体专用）。
//
// 设计动机：
//   - 智能体需要调用生成服务，生成视频/图片/音频
//   - 封装UnifiedSubmit方法，提供统一的生成接口
//   - 用户不需要选择端点/模型，系统自动选择
type GenerateContentTool struct {
	genUC *generation.GenerationUseCase
}

func NewGenerateContentTool(genUC *generation.GenerationUseCase) *GenerateContentTool {
	return &GenerateContentTool{genUC: genUC}
}

func (t *GenerateContentTool) Name() string {
	return "generate_content"
}

func (t *GenerateContentTool) Description() string {
	return `生成内容（视频/图片/音频）。

参数：
- tenant_id（必填）：租户ID
- brand_id（必填）：品牌ID
- text（必填）：文本描述
- materials（可选）：素材ID列表
- template（可选）：模板ID
- duration（可选）：时长（秒）
- quality（可选）：质量（默认720p）

返回：
- task_id：任务ID
- status：任务状态
- sub_type：自动选择的端点类型

使用场景：
- 生成品牌宣传视频
- 生成产品介绍视频
- 生成数字人口播视频
- 生成语音合成

注意事项：
- 系统会根据素材自动选择端点
- 用户不需要选择模型
- 生成需要消耗用户资源（积分）`
}

// TaskResult 任务结果。
type TaskResult struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	SubType string `json:"sub_type"`
}

// Generate 生成内容。
func (t *GenerateContentTool) Generate(ctx context.Context, input generation.UnifiedSubmitInput) (TaskResult, error) {
	if t.genUC == nil {
		return TaskResult{}, fmt.Errorf("生成服务未配置")
	}

	// 调用统一提交方法
	task, err := t.genUC.UnifiedSubmit(ctx, input)
	if err != nil {
		return TaskResult{}, fmt.Errorf("生成失败: %w", err)
	}

	return TaskResult{
		TaskID:  task.ID,
		Status:  task.State,
		SubType: task.SubType,
	}, nil
}
