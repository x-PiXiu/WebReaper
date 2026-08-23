package generation

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// EndpointSelectorImpl 端点选择器实现（整洁架构·Usecase层）。
//
// 设计动机：
//   - 根据素材类型自动选择端点（subType）
//   - 将统一格式参数转换为端点特定参数
//   - 填充默认值（duration、resolution等）
//
// 端点选择规则（基于Vidu端点参数限制）：
//   - 只有文本 → text2video
//   - 1张图片 → img2video
//   - 2张图片 → start_end2video
//   - 3-7张图片 → reference2video
//   - 1张图片+音频 → digital_human
//   - 1张图片+文本 → img2video
//   - 1个视频+音频 → lip_sync
//   - 1个视频+文本 → lip_sync
//   - 只有音频 → tts
type EndpointSelectorImpl struct {
	mediaStore   port.MediaAssetStore
	templateRepo port.TemplateRepository
}

var _ port.EndpointSelector = (*EndpointSelectorImpl)(nil)

func NewEndpointSelector(mediaStore port.MediaAssetStore, templateRepo port.TemplateRepository) *EndpointSelectorImpl {
	return &EndpointSelectorImpl{
		mediaStore:   mediaStore,
		templateRepo: templateRepo,
	}
}

// MaterialStats 素材统计。
type MaterialStats struct {
	ImageCount int
	VideoCount int
	AudioCount int
	Images     []entity.MediaAsset
	Videos     []entity.MediaAsset
	Audios     []entity.MediaAsset
}

// Select 根据素材自动选择端点。
func (s *EndpointSelectorImpl) Select(ctx context.Context, req entity.UnifiedGenerationRequest) (entity.EndpointSelectResult, error) {
	// 1. 获取素材信息
	materials, err := s.getMaterials(ctx, req.Materials)
	if err != nil {
		return entity.EndpointSelectResult{}, err
	}

	// 2. 统计素材类型
	stats := s.analyzeMaterials(materials)
	hasText := req.Text != ""

	// 3. 根据素材组合选择端点
	subType, params, err := s.selectEndpoint(req, stats, hasText)
	if err != nil {
		return entity.EndpointSelectResult{}, err
	}

	// 4. 应用默认值（从模板或配置）
	params = s.applyDefaults(ctx, params, req)

	return entity.EndpointSelectResult{
		SubType: subType,
		Params:  params,
	}, nil
}

// getMaterials 获取素材信息。
func (s *EndpointSelectorImpl) getMaterials(ctx context.Context, materialIDs []string) ([]entity.MediaAsset, error) {
	if len(materialIDs) == 0 {
		return nil, nil
	}

	if s.mediaStore == nil {
		return nil, nil
	}

	// 查询素材库（使用List方法，按租户查询后过滤）
	// 注意：这里需要修改为支持按ID查询的方法，暂时使用List
	allMaterials, err := s.mediaStore.List(ctx, "", entity.AssetTypeMaterial)
	if err != nil {
		return nil, fmt.Errorf("查询素材失败: %w", err)
	}

	// 过滤出指定ID的素材
	idMap := make(map[string]bool)
	for _, id := range materialIDs {
		idMap[id] = true
	}

	var materials []entity.MediaAsset
	for _, m := range allMaterials {
		if idMap[m.ID] {
			materials = append(materials, m)
		}
	}

	return materials, nil
}

// analyzeMaterials 分析素材类型。
func (s *EndpointSelectorImpl) analyzeMaterials(materials []entity.MediaAsset) MaterialStats {
	stats := MaterialStats{}
	for _, m := range materials {
		switch m.Type {
		case entity.MaterialTypeImage:
			stats.ImageCount++
			stats.Images = append(stats.Images, m)
		case entity.MaterialTypeVideo:
			stats.VideoCount++
			stats.Videos = append(stats.Videos, m)
		case entity.MaterialTypeAudio:
			stats.AudioCount++
			stats.Audios = append(stats.Audios, m)
		}
	}
	return stats
}

// selectEndpoint 根据素材组合选择端点。
func (s *EndpointSelectorImpl) selectEndpoint(req entity.UnifiedGenerationRequest, stats MaterialStats, hasText bool) (string, entity.GenerationParams, error) {
	// 情况1: 有视频+音频 → 对口型
	if stats.VideoCount > 0 && stats.AudioCount > 0 {
		return "lip_sync", s.buildLipSyncParams(req, stats), nil
	}

	// 情况2: 有视频+文本 → 对口型（文本驱动）
	if stats.VideoCount > 0 && hasText {
		return "lip_sync", s.buildLipSyncTextParams(req, stats), nil
	}

	// 情况3: 单张图片+音频 → 数字人口播
	if stats.ImageCount == 1 && stats.AudioCount > 0 {
		return "digital_human", s.buildDigitalHumanParams(req, stats), nil
	}

	// 情况4: 单张图片+文本 → 图生视频
	if stats.ImageCount == 1 && hasText {
		return "img2video", s.buildImg2VideoParams(req, stats), nil
	}

	// 情况5: 恰好2张图片 → 首尾帧视频
	if stats.ImageCount == 2 {
		return "start_end2video", s.buildStartEndParams(req, stats), nil
	}

	// 情况6: 3-7张图片 → 参考生视频
	if stats.ImageCount >= 3 && stats.ImageCount <= 7 {
		return "reference2video", s.buildReference2VideoParams(req, stats), nil
	}

	// 情况7: 只有音频 → 语音合成
	if stats.AudioCount > 0 && stats.ImageCount == 0 && stats.VideoCount == 0 {
		return "tts", s.buildTTSParams(req, stats), nil
	}

	// 情况8: 只有文本 → 文生视频
	if hasText && stats.ImageCount == 0 && stats.VideoCount == 0 && stats.AudioCount == 0 {
		return "text2video", s.buildText2VideoParams(req), nil
	}

	return "", nil, fmt.Errorf("无法根据素材组合确定端点类型")
}

// buildText2VideoParams 构建文生视频参数。
func (s *EndpointSelectorImpl) buildText2VideoParams(req entity.UnifiedGenerationRequest) entity.GenerationParams {
	return entity.GenerationParams{
		"prompt": req.Text,
	}
}

// buildImg2VideoParams 构建图生视频参数。
func (s *EndpointSelectorImpl) buildImg2VideoParams(req entity.UnifiedGenerationRequest, stats MaterialStats) entity.GenerationParams {
	return entity.GenerationParams{
		"images": []string{stats.Images[0].SourceURL},
		"prompt": req.Text,
	}
}

// buildStartEndParams 构建首尾帧参数。
func (s *EndpointSelectorImpl) buildStartEndParams(req entity.UnifiedGenerationRequest, stats MaterialStats) entity.GenerationParams {
	return entity.GenerationParams{
		"images": []string{stats.Images[0].SourceURL, stats.Images[1].SourceURL},
		"prompt": req.Text,
	}
}

// buildReference2VideoParams 构建参考生视频参数。
func (s *EndpointSelectorImpl) buildReference2VideoParams(req entity.UnifiedGenerationRequest, stats MaterialStats) entity.GenerationParams {
	var imageURLs []string
	for _, img := range stats.Images {
		imageURLs = append(imageURLs, img.SourceURL)
	}
	return entity.GenerationParams{
		"images": imageURLs,
		"prompt": req.Text,
	}
}

// buildLipSyncParams 构建对口型参数（音频驱动）。
func (s *EndpointSelectorImpl) buildLipSyncParams(req entity.UnifiedGenerationRequest, stats MaterialStats) entity.GenerationParams {
	return entity.GenerationParams{
		"video_url": stats.Videos[0].SourceURL,
		"audio_url": stats.Audios[0].SourceURL,
	}
}

// buildLipSyncTextParams 构建对口型参数（文本驱动）。
func (s *EndpointSelectorImpl) buildLipSyncTextParams(req entity.UnifiedGenerationRequest, stats MaterialStats) entity.GenerationParams {
	return entity.GenerationParams{
		"video_url": stats.Videos[0].SourceURL,
		"text":      req.Text,
		"voice_id":  "default",
	}
}

// buildDigitalHumanParams 构建数字人口播参数。
func (s *EndpointSelectorImpl) buildDigitalHumanParams(req entity.UnifiedGenerationRequest, stats MaterialStats) entity.GenerationParams {
	return entity.GenerationParams{
		"image":     stats.Images[0].SourceURL,
		"audio_url": stats.Audios[0].SourceURL,
	}
}

// buildTTSParams 构建语音合成参数。
func (s *EndpointSelectorImpl) buildTTSParams(req entity.UnifiedGenerationRequest, stats MaterialStats) entity.GenerationParams {
	return entity.GenerationParams{
		"text":                  req.Text,
		"voice_setting_voice_id": "default",
	}
}

// applyDefaults 应用默认值。
func (s *EndpointSelectorImpl) applyDefaults(ctx context.Context, params entity.GenerationParams, req entity.UnifiedGenerationRequest) entity.GenerationParams {
	// 如果用户指定了时长，使用用户指定的
	if req.Duration > 0 {
		params["duration"] = req.Duration
	}

	// 如果用户指定了质量，使用用户指定的
	if req.Quality != "" {
		params["resolution"] = req.Quality
	} else {
		params["resolution"] = "720p" // 默认720p
	}

	// 如果用户指定了比例，使用用户指定的
	if req.AspectRatio != "" {
		params["aspect_ratio"] = req.AspectRatio
	} else {
		params["aspect_ratio"] = "16:9" // 默认16:9
	}

	return params
}
