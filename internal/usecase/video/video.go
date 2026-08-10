// Package video 实现"视频生成工作台"用例：素材/文本 → 视频 → 配音 → 合成 → 就绪。
//
// 整洁架构：
//   - 依赖 port 接口（VideoProvider/VoiceSynthesizer/VideoComposer），不感知具体视频模型。
//   - 流水线骨架固定（模板方法），各阶段实现可替换（策略）——新增视频模型 = 新适配器。
//   - 任务状态机规则在实体层（CanTransitionTo），用例层只编排。
//
// 异步设计：提交任务立即返回，后台 goroutine 驱动流水线，
// 前端轮询 GET /tasks/:id 查看进度（复用 task 域的模式）。
package video

import (
	"context"
	"fmt"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// VideoUseCase 视频生成工作台用例。
type VideoUseCase struct {
	repo      port.VideoTaskRepository
	jobRepo   port.VideoJobRepository
	provider  port.VideoProvider  // 可为 nil：未配置视频模型时创建即失败（提示配置）
	voice     port.VoiceSynthesizer // 可为 nil：跳过配音阶段
	composer  port.VideoComposer  // 可为 nil：跳过合成阶段（直接发布原视频）
	logger    port.Logger
	pollSec   int // 视频生成轮询间隔（秒），测试可注入
	mu        sync.Mutex
	workers   map[string]bool // taskID → 正在跑
}

// NewVideoUseCase 创建视频用例。
// provider/voice/composer 未配置时传 nil，对应阶段自动跳过或标记失败（见各方法）。
func NewVideoUseCase(
	repo port.VideoTaskRepository,
	jobRepo port.VideoJobRepository,
	provider port.VideoProvider,
	voice port.VoiceSynthesizer,
	composer port.VideoComposer,
	logger port.Logger,
) *VideoUseCase {
	return &VideoUseCase{
		repo:     repo,
		jobRepo:  jobRepo,
		provider: provider,
		voice:    voice,
		composer: composer,
		logger:   logger,
		pollSec:  5,
		workers:  make(map[string]bool),
	}
}

// SubmitInput 提交生成任务的入参。
type SubmitInput struct {
	TenantID    string
	BrandID     string
	Mode        string // text / material
	Prompt      string
	MaterialURL string
}

// Submit 提交视频生成任务（立即返回，流水线后台驱动）。
// 视频模型未配置（provider==nil）时直接失败——前端提示配置 Vidu API Key。
func (uc *VideoUseCase) Submit(ctx context.Context, in SubmitInput) (entity.VideoTask, error) {
	task := entity.VideoTask{
		ID:          fmt.Sprintf("vt-%d", time.Now().UnixNano()),
		TenantID:    in.TenantID,
		BrandID:     in.BrandID,
		Mode:        in.Mode,
		Prompt:      in.Prompt,
		MaterialURL: in.MaterialURL,
		Status:      entity.VideoStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if !task.IsValid() {
		return entity.VideoTask{}, fmt.Errorf("任务参数不完整：mode=%s 需要对应输入", in.Mode)
	}
	if err := uc.repo.Save(ctx, task); err != nil {
		return entity.VideoTask{}, err
	}
	// 后台驱动流水线（复用租户上下文，失败不 panic）
	go uc.pipeline(context.Background(), task.ID)
	return task, nil
}

// Get 查询任务详情（前端轮询）。
func (uc *VideoUseCase) Get(ctx context.Context, tenantID, id string) (entity.VideoTask, error) {
	return uc.repo.FindByID(ctx, tenantID, id)
}

// List 任务列表。
func (uc *VideoUseCase) List(ctx context.Context, tenantID string, limit int) ([]entity.VideoTask, error) {
	if limit <= 0 {
		limit = 30
	}
	return uc.repo.ListByTenant(ctx, tenantID, limit)
}

// pipeline 流水线骨架（模板方法）：生成 → 配音 → 合成 → 就绪。
// 任一阶段失败 → failed（保留 Error）；可重新提交重试。
func (uc *VideoUseCase) pipeline(ctx context.Context, taskID string) {
	uc.mu.Lock()
	if uc.workers[taskID] {
		uc.mu.Unlock()
		return
	}
	uc.workers[taskID] = true
	uc.mu.Unlock()
	defer func() {
		uc.mu.Lock()
		delete(uc.workers, taskID)
		uc.mu.Unlock()
	}()

	task, err := uc.repo.FindByID(ctx, "", taskID)
	if err != nil {
		return
	}
	log := uc.logger.With(port.String("task", taskID), port.String("mode", task.Mode))

	// ① 视频生成
	if err := uc.stepGenerate(ctx, &task, log); err != nil {
		uc.fail(ctx, task, err)
		return
	}
	// ② 配音（未配置则跳过）
	if task.VoiceText != "" && uc.voice != nil {
		if err := uc.stepDub(ctx, &task, log); err != nil {
			uc.fail(ctx, task, err)
			return
		}
	}
	// ③ 合成（未配置则跳过——直接用原视频）
	if uc.composer != nil && task.VoiceURL != "" {
		if err := uc.stepCompose(ctx, &task, log); err != nil {
			uc.fail(ctx, task, err)
			return
		}
	}
	// ④ 就绪
	if task.CanTransitionTo(entity.VideoStatusReady) {
		_ = uc.repo.UpdateStatus(ctx, task.TenantID, task.ID, entity.VideoStatusReady, "")
		log.Info("视频成片就绪", port.String("final", task.FinalURL))
	}
}

// stepGenerate ① 视频生成（异步轮询 provider）。
func (uc *VideoUseCase) stepGenerate(ctx context.Context, task *entity.VideoTask, log port.Logger) error {
	if uc.provider == nil {
		return fmt.Errorf("视频生成模型未配置（VIDU_API_KEY）")
	}
	if !task.CanTransitionTo(entity.VideoStatusGenerating) {
		return nil
	}
	_ = uc.repo.UpdateStatus(ctx, task.TenantID, task.ID, entity.VideoStatusGenerating, "")
	remoteID, err := uc.provider.Submit(ctx, task.Mode, task.Prompt, task.MaterialURL)
	if err != nil {
		return fmt.Errorf("视频生成提交失败：%w", err)
	}
	log.Info("视频生成已提交", port.String("remote", remoteID))
	// 轮询直到完成
	ticker := time.NewTicker(time.Duration(uc.pollSec) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("视频生成中断：%w", ctx.Err())
		case <-ticker.C:
			_, done, videoURL, err := uc.provider.Poll(ctx, remoteID)
			if err != nil {
				return fmt.Errorf("视频生成轮询失败：%w", err)
			}
			if done {
				task.VideoURL = videoURL
				_ = uc.repo.UpdateResult(ctx, task.TenantID, task.ID, map[string]any{"video_url": videoURL})
				log.Info("视频生成完成", port.String("url", videoURL))
				return nil
			}
		}
	}
}

// stepDub ② 配音。
func (uc *VideoUseCase) stepDub(ctx context.Context, task *entity.VideoTask, log port.Logger) error {
	if !task.CanTransitionTo(entity.VideoStatusDubbing) {
		return nil
	}
	_ = uc.repo.UpdateStatus(ctx, task.TenantID, task.ID, entity.VideoStatusDubbing, "")
	audioURL, err := uc.voice.Synthesize(ctx, task.VoiceText, "")
	if err != nil {
		return fmt.Errorf("配音失败：%w", err)
	}
	task.VoiceURL = audioURL
	_ = uc.repo.UpdateResult(ctx, task.TenantID, task.ID, map[string]any{"voice_url": audioURL})
	log.Info("配音完成", port.String("url", audioURL))
	return nil
}

// stepCompose ③ 合成。
func (uc *VideoUseCase) stepCompose(ctx context.Context, task *entity.VideoTask, log port.Logger) error {
	if !task.CanTransitionTo(entity.VideoStatusComposing) {
		return nil
	}
	_ = uc.repo.UpdateStatus(ctx, task.TenantID, task.ID, entity.VideoStatusComposing, "")
	finalURL, err := uc.composer.Compose(ctx, task.VideoURL, task.VoiceURL)
	if err != nil {
		return fmt.Errorf("合成失败：%w", err)
	}
	task.FinalURL = finalURL
	_ = uc.repo.UpdateResult(ctx, task.TenantID, task.ID, map[string]any{"final_url": finalURL})
	log.Info("合成完成", port.String("url", finalURL))
	return nil
}

// fail 任务失败（保留 Error，可重新提交）。
func (uc *VideoUseCase) fail(ctx context.Context, task entity.VideoTask, err error) {
	_ = uc.repo.UpdateStatus(ctx, task.TenantID, task.ID, entity.VideoStatusFailed, err.Error())
	uc.logger.Error("视频任务失败", port.Err(err), port.String("task", task.ID))
}

// Publish ④ 视频发布（抖音/快手等）。
// accountID 为空 = 账号池随机；发布通道由上层（handler）按平台注入。
func (uc *VideoUseCase) Publish(ctx context.Context, tenantID, taskID, platform, accountID string) (entity.VideoJob, error) {
	task, err := uc.repo.FindByID(ctx, tenantID, taskID)
	if err != nil {
		return entity.VideoJob{}, err
	}
	if task.Status != entity.VideoStatusReady {
		return entity.VideoJob{}, fmt.Errorf("任务未就绪（当前：%s），请等待成片生成", task.StatusLabel())
	}
	if task.FinalURL == "" {
		return entity.VideoJob{}, fmt.Errorf("成片地址为空，无法发布")
	}
	job := entity.VideoJob{
		ID:        fmt.Sprintf("vj-%d", time.Now().UnixNano()),
		TenantID:  tenantID,
		TaskID:    taskID,
		AccountID: accountID,
		Platform:  platform,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	if err := uc.jobRepo.Save(ctx, job); err != nil {
		return entity.VideoJob{}, err
	}
	return job, nil
}

// ListJobs 视频发布任务列表。
func (uc *VideoUseCase) ListJobs(ctx context.Context, tenantID string, limit int) ([]entity.VideoJob, error) {
	if limit <= 0 {
		limit = 30
	}
	return uc.jobRepo.ListByTenant(ctx, tenantID, limit)
}
