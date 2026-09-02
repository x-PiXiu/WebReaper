// avatar_chain.go 链式形象视频（25 号阶段二——23 号计划 §2.1②）。
//
// 主体注册成功后自动创建 10s 不说话的形象视频任务（reference2video）：
//   - 主体一致性：subjects[0]={name, server_id}
//   - D1 场景注入：有 scene_image 时 subjects[1]={name:"场景", images:[...]}
//     （参考生视频支持多主体混合——见 Docs/第三方/Vidu/创建视频任务/参考生视频.md）
//   - 场景描述融入 prompt，无则默认形象展示模板
//   - 链式任务带 params.avatar_video=true（作品库过滤用——中间产物不进「我的作品」）
//   - 任务 ID 写回主体任务 params.avatar_task_id（前端 join 任务列表渲染三态/预览）
//   - 28号改进：prompt 支持用户自定义 + settings 默认值 + @name 引用
//
// 决策点（25 号 §2.3.1）：D2 配置 gen_chain_avatar_video 默认开；D3 计入 generation
// 配额（走 Submit 既有闸门）；D4 链式失败不影响主体可用性，可幂等重试；环境主体不链。
package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
)

// defaultAvatarVideoPrompt 兜底默认值（settings 未配置时使用）。
const defaultAvatarVideoPrompt = "形象展示：正面特写，微笑看向镜头，姿态自然大方，缓慢自然的肢体动作"

// maybeChainAvatarVideo 创建主体成功后的链式钩子：配置关闭或环境主体时跳过；
// 链式失败仅日志（D4——主体本身不受影响，前端可走重试端点补建）。
func (uc *GenerationUseCase) maybeChainAvatarVideo(ctx context.Context, subject entity.GenerationTask) {
	var p map[string]any
	_ = json.Unmarshal([]byte(subject.ParamsJSON), &p)
	if k, _ := p["kind"].(string); k == "scene" {
		return // 环境主体（25 号 §6.5）不需要形象视频
	}
	if !uc.generationBoolDefault(ctx, "gen_chain_avatar_video", true) {
		return // D2：运营开关（默认开）
	}
	if _, err := uc.chainAvatarVideo(ctx, subject); err != nil {
		log.Printf("[avatar-chain] 主体 %s 形象视频链式创建失败（不影响主体可用，可重试）: %v", subject.ID, err)
	}
}

// chainAvatarVideo 创建形象视频任务并写回 params.avatar_task_id。
func (uc *GenerationUseCase) chainAvatarVideo(ctx context.Context, subject entity.GenerationTask) (entity.GenerationTask, error) {
	serverID := subject.ProviderTaskID
	if serverID == "" {
		serverID = firstCreationID(subject.CreationsJSON)
	}
	if serverID == "" {
		return entity.GenerationTask{}, fmt.Errorf("主体缺少 server_id，无法生成形象视频")
	}

	var p map[string]any
	_ = json.Unmarshal([]byte(subject.ParamsJSON), &p)
	if p == nil {
		p = map[string]any{}
	}
	name, _ := p["name"].(string)
	if name == "" {
		name = "主体"
	}
	sceneDesc, _ := p["scene_description"].(string)
	sceneImg, _ := p["scene_image"].(string)
	sceneSubjectID, _ := p["scene_subject_id"].(string)

	// 32号 F2：场景主体复用——选择已有环境主体时，取其 server_id 作为第二主体，
	// 并回查其注册任务的场景描述作为画面语义。免重复上传场景图。
	var sceneServerID string
	if sceneSubjectID != "" && uc.subjectAssetRepo != nil {
		if a, err := uc.subjectAssetRepo.FindByID(ctx, sceneSubjectID); err == nil && a.Status == "active" {
			sceneServerID = a.ServerID
			if sceneDesc == "" && a.SourceTaskID != "" && uc.repo != nil {
				if srcTask, sErr := uc.repo.FindByID(ctx, a.TenantID, a.SourceTaskID); sErr == nil {
					var sp map[string]any
					_ = json.Unmarshal([]byte(srcTask.ParamsJSON), &sp)
					if sd, _ := sp["scene_description"].(string); strings.TrimSpace(sd) != "" {
						sceneDesc = sd
					}
				}
			}
		}
	}

	// 28号改进：prompt 支持用户自定义 + settings 默认值
	prompt := strings.TrimSpace(sceneDesc)
	if prompt == "" {
		// 从 system_settings 读取默认 prompt（运营可调）
		prompt = uc.settingString(ctx, entity.SettingKeyGenDefaultAvatarPrompt, defaultAvatarVideoPrompt)
	}

	// 28号改进：添加 @name 引用（Vidu 参考生视频需要在 prompt 中引用主体）
	prompt = fmt.Sprintf("@%s %s", name, prompt)

	subjects := []any{map[string]any{"name": name, "server_id": serverID}}
	if sceneImg != "" {
		// D1：场景图作为附加图片主体（非主体模式降级——保持主体一致性）
		subjects = append(subjects, map[string]any{"name": "场景", "images": []string{sceneImg}})
	}
	if sceneServerID != "" {
		// 32号 F2：场景主体作为第二主体（组合出镜——分身 × 复用环境）
		subjects = append(subjects, map[string]any{"name": "场景", "server_id": sceneServerID})
	}

	chain, err := uc.Submit(ctx, SubmitInput{
		TenantID: subject.TenantID, BrandID: subject.BrandID,
		SubType: "reference2video",
		Params: entity.GenerationParams{
			"prompt":       prompt,
			"duration":     10,
			"aspect_ratio": "9:16",
			"subjects":     subjects,
			"avatar_video": true,
			// 31号 §4.1：形象视频是内部链画面素材（分身预览/lip_sync 输入）——
			// 显式静默（Q3 音画同出默认开，带声画面二次驱动对口型效果劣化）
			"audio": false,
		},
	})
	if err != nil {
		return chain, err
	}

	// 写回 avatar_task_id（params JSON 读改写后整任务落库）
	p["avatar_task_id"] = chain.ID
	pj, _ := json.Marshal(p)
	subject.ParamsJSON = string(pj)
	subject.UpdatedAt = time.Now()
	if err := uc.repo.Save(ctx, subject); err != nil {
		log.Printf("[avatar-chain] 主体 %s 写回 avatar_task_id 失败: %v", subject.ID, err)
	}
	return chain, nil
}

// RetryAvatarVideo 重试/补建形象视频（幂等：已有未终态链式任务直接返回该任务）。
// 对应 POST /generation/tasks/:id/avatar-video——仅作用于 sub_type=subject 的已注册主体。
func (uc *GenerationUseCase) RetryAvatarVideo(ctx context.Context, tenantID, taskID string) (entity.GenerationTask, error) {
	subject, err := uc.repo.FindByID(ctx, tenantID, taskID)
	if err != nil {
		return entity.GenerationTask{}, fmt.Errorf("主体任务不存在")
	}
	if subject.SubType != "subject" {
		return entity.GenerationTask{}, fmt.Errorf("仅数字分身主体可生成形象视频")
	}
	if subject.State != entity.TaskStateSuccess {
		return entity.GenerationTask{}, fmt.Errorf("主体尚未注册成功，无法生成形象视频")
	}

	var p map[string]any
	_ = json.Unmarshal([]byte(subject.ParamsJSON), &p)
	if prev, _ := p["avatar_task_id"].(string); prev != "" {
		if prevTask, err := uc.repo.FindByID(ctx, tenantID, prev); err == nil {
			switch prevTask.State {
			case entity.TaskStateCreated, entity.TaskStateQueueing, entity.TaskStateProcessing:
				return prevTask, nil // 幂等：未终态不重复创建
			}
		}
	}
	return uc.chainAvatarVideo(ctx, subject)
}

// firstCreationID 解析 creations JSON 的首个 id（主体任务的 server_id 兜底路径）。
func firstCreationID(creationsJSON string) string {
	if creationsJSON == "" {
		return ""
	}
	var arr []entity.CreationItem
	if err := json.Unmarshal([]byte(creationsJSON), &arr); err != nil || len(arr) == 0 {
		return ""
	}
	return arr[0].ID
}
