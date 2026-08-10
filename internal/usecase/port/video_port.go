package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ---- 视频生成域接口（用例层声明，适配器实现）----
// 视频模型会变（Vidu/Sora/可灵）——适配器层隔离，业务不感知。
// 生成/配音为异步任务：提交后返回 taskID，轮询状态查询进度。

// VideoProvider 视频生成提供方（策略：Vidu / Sora / 可灵...）。
type VideoProvider interface {
	// Name 提供方标识（vidu / sora / kling...）。
	Name() string
	// Submit 提交视频生成任务（文生视频/图生视频）。返回异步任务 ID。
	Submit(ctx context.Context, mode, prompt, materialURL string) (taskID string, err error)
	// Poll 查询生成进度（0~1）。done=true 时 URL 有值。
	Poll(ctx context.Context, taskID string) (progress float64, done bool, videoURL string, err error)
}

// VoiceSynthesizer 配音合成器（TTS）。
type VoiceSynthesizer interface {
	// Synthesize 文本 → 音频文件 URL。
	Synthesize(ctx context.Context, text, voiceID string) (audioURL string, err error)
}

// VideoComposer 音视频合成器（ffmpeg）。
type VideoComposer interface {
	// Compose 视频 + 配音 → 成片 URL。
	Compose(ctx context.Context, videoURL, voiceURL string) (finalURL string, err error)
}

// VideoTaskRepository 视频任务仓储（多租户）。
type VideoTaskRepository interface {
	Save(ctx context.Context, t entity.VideoTask) error
	FindByID(ctx context.Context, tenantID, id string) (entity.VideoTask, error)
	ListByTenant(ctx context.Context, tenantID string, limit int) ([]entity.VideoTask, error)
	UpdateStatus(ctx context.Context, tenantID, id string, status entity.VideoTaskStatus, errMsg string) error
	// UpdateResult 更新流水线产物（视频/配音/成片 URL）。
	UpdateResult(ctx context.Context, tenantID, id string, result map[string]any) error
	// Count 统计任务总数（平台总览用，admin 看全局）。
	Count(ctx context.Context) (int, error)
}

// VideoJobRepository 视频发布任务仓储（多租户）。
type VideoJobRepository interface {
	Save(ctx context.Context, j entity.VideoJob) error
	FindByID(ctx context.Context, tenantID, id string) (entity.VideoJob, error)
	ListByTenant(ctx context.Context, tenantID string, limit int) ([]entity.VideoJob, error)
	UpdateStatus(ctx context.Context, tenantID, id, status, externalURL, errMsg string) error
}

// GeoResolver IP 地理位置解析器（策略：免费 API / 本地 MMDB 库）。
// 用于"登录地址"能力——登录时记录归属地，发布内容时可选附带。
type GeoResolver interface {
	// Resolve 解析 IP 归属地。失败返回错误（调用方降级：仅记 IP 原文）。
	Resolve(ctx context.Context, ip string) (GeoLocation, error)
}

// GeoLocation IP 归属地。
type GeoLocation struct {
	Country  string
	Province string
	City     string
	Lat      float64
	Lng      float64
}
