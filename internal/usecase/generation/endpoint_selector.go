package generation

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

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
	materials, err := s.getMaterials(ctx, req.TenantID, req.Materials)
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
func (s *EndpointSelectorImpl) getMaterials(ctx context.Context, tenantID string, materialIDs []string) ([]entity.MediaAsset, error) {
	if len(materialIDs) == 0 {
		return nil, nil
	}

	if s.mediaStore == nil {
		return nil, nil
	}

	// 查询素材库（租户隔离——此前传空租户列出全部租户素材再按 ID 过滤，
	// 隔离仅靠 ID 不可猜；现从请求上下文取租户）
	allMaterials, err := s.mediaStore.List(ctx, tenantID, entity.AssetTypeMaterial)
	if err != nil {
		return nil, fmt.Errorf("查询素材失败: %w", err)
	}

	// 过滤出指定素材：ID 直配 或 URL 直传匹配（前端对本站 /media/... 资产免重传，
	// 直接传原 URL——按路径后缀匹配 SourceURL）
	idMap := make(map[string]bool)
	urlMap := make(map[string]bool)
	for _, id := range materialIDs {
		if strings.HasPrefix(id, "/media/") || strings.Contains(id, "/media/") {
			urlMap[filepath.Base(id)] = true
		} else {
			idMap[id] = true
		}
	}

	var materials []entity.MediaAsset
	for _, m := range allMaterials {
		if idMap[m.ID] || (len(urlMap) > 0 && urlMap[filepath.Base(m.SourceURL)]) {
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
	// 如果用户明确指定了类型，直接使用
	if req.Type != "" {
		return s.selectByType(req, stats, hasText)
	}

	// 情况1: 有视频+音频 → 对口型
	if stats.VideoCount > 0 && stats.AudioCount > 0 {
		params := s.buildLipSyncParams(req, stats)
		params["__sub_type"] = "lip_sync"
		return "lip_sync", params, nil
	}

	// 情况2: 有视频+文本 → 对口型（文本驱动）
	if stats.VideoCount > 0 && hasText {
		params := s.buildLipSyncTextParams(req, stats)
		params["__sub_type"] = "lip_sync"
		return "lip_sync", params, nil
	}

	// 情况3: 单张图片+音频 → 数字人口播
	if stats.ImageCount == 1 && stats.AudioCount > 0 {
		params := s.buildDigitalHumanParams(req, stats)
		params["__sub_type"] = "digital_human"
		return "digital_human", params, nil
	}

	// 情况4: 单张图片+文本 → 图生视频
	if stats.ImageCount == 1 && hasText {
		params := s.buildImg2VideoParams(req, stats)
		params["__sub_type"] = "img2video"
		return "img2video", params, nil
	}

	// 情况5: 恰好2张图片 → 首尾帧视频
	if stats.ImageCount == 2 {
		params := s.buildStartEndParams(req, stats)
		params["__sub_type"] = "start_end2video"
		return "start_end2video", params, nil
	}

	// 情况6: 3-7张图片 → 参考生视频
	if stats.ImageCount >= 3 && stats.ImageCount <= 7 {
		params := s.buildReference2VideoParams(req, stats)
		params["__sub_type"] = "reference2video"
		return "reference2video", params, nil
	}

	// 情况7: 只有音频 → 语音合成
	if stats.AudioCount > 0 && stats.ImageCount == 0 && stats.VideoCount == 0 {
		params := s.buildTTSParams(req, stats)
		params["__sub_type"] = "tts"
		return "tts", params, nil
	}

	// 情况8: 只有文本 → 文生视频
	if hasText && stats.ImageCount == 0 && stats.VideoCount == 0 && stats.AudioCount == 0 {
		params := s.buildText2VideoParams(req)
		params["__sub_type"] = "text2video"
		return "text2video", params, nil
	}

	return "", nil, fmt.Errorf("无法根据素材组合确定端点类型")
}

// selectByType 根据用户指定的类型选择端点。
func (s *EndpointSelectorImpl) selectByType(req entity.UnifiedGenerationRequest, stats MaterialStats, hasText bool) (string, entity.GenerationParams, error) {
	switch req.Type {
	case "video":
		// 视频类型：根据素材选择具体端点
		if stats.ImageCount == 1 {
			params := s.buildImg2VideoParams(req, stats)
			params["__sub_type"] = "img2video"
			return "img2video", params, nil
		} else if stats.ImageCount == 2 {
			params := s.buildStartEndParams(req, stats)
			params["__sub_type"] = "start_end2video"
			return "start_end2video", params, nil
		} else if stats.ImageCount >= 3 {
			params := s.buildReference2VideoParams(req, stats)
			params["__sub_type"] = "reference2video"
			return "reference2video", params, nil
		} else {
			params := s.buildText2VideoParams(req)
			params["__sub_type"] = "text2video"
			return "text2video", params, nil
		}
	case "image":
		params := s.buildText2ImageParams(req)
		params["__sub_type"] = "text2image"
		return "text2image", params, nil
	case "audio":
		params := s.buildTTSParams(req, stats)
		params["__sub_type"] = "tts"
		return "tts", params, nil
	case "voice":
		params := s.buildVoiceCloneParams(req, stats)
		params["__sub_type"] = "voice_clone"
		return "voice_clone", params, nil
	default:
		return "", nil, fmt.Errorf("不支持的生成类型: %s", req.Type)
	}
}

// buildText2ImageParams 构建文生图参数。
func (s *EndpointSelectorImpl) buildText2ImageParams(req entity.UnifiedGenerationRequest) entity.GenerationParams {
	return entity.GenerationParams{
		"prompt": req.Text,
	}
}

// buildVoiceCloneParams 构建声音克隆参数。
func (s *EndpointSelectorImpl) buildVoiceCloneParams(req entity.UnifiedGenerationRequest, stats MaterialStats) entity.GenerationParams {
	// 生成唯一的 voice_id（8-256位，字母/数字/横线/下划线，首字符必须为英文字母）
	voiceID := fmt.Sprintf("voice-clone-%d", len(req.Text))
	if len(voiceID) < 8 {
		voiceID = voiceID + "-default"
	}
	
	params := entity.GenerationParams{
		"text":     req.Text,
		"voice_id": voiceID,
	}
	if len(stats.Audios) > 0 {
		params["audio_url"] = stats.Audios[0].SourceURL
	}
	return params
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

// advancedParamKeys 高级参数白名单（merge 的准入集合——adapter 只按已知 key 构造
// 请求体，白名单外字段即使混入也会被 adapter 忽略；白名单让意图显式且防误覆盖核心字段）。
var advancedParamKeys = map[string]bool{
	"seed": true, "style": true, "movement_amplitude": true,
	"audio": true, "audio_type": true, "bgm": true,
	"watermark": true, "off_peak": true, "payload": true,
	"voice_id": true, // 用户自定义音色名（voice_clone 尊重用户输入，覆盖自动生成名）
	"image_settings": true, "timing_prompts": true,
	"speed": true, "volume": true, "ref_photo_url": true, // lip_sync 文本驱动（裸 key 风格）
	"model": true, "resolution": true, "duration": true, "aspect_ratio": true,
}

// advancedParamAllowed 白名单判定（voice_setting_* 前缀族整体放行——TTS 语速/音量/音色/情感）。
func advancedParamAllowed(key string) bool {
	return advancedParamKeys[key] || strings.HasPrefix(key, "voice_setting_")
}

// mergeAdvancedParams 用户高级参数合并（覆盖默认值——applyDefaults 之后调用，
// 用户显式值优先）。兼容层透传的专业模式参数（multiframe 的 image_settings、
// 音效的 timing_prompts、TTS 的 voice_setting_* 等）由此到达 adapter。
func mergeAdvancedParams(params entity.GenerationParams, user map[string]any) {
	for k, v := range user {
		if v == nil || !advancedParamAllowed(k) {
			continue
		}
		params[k] = v
	}
}

// applyDefaults 应用默认值。
func (s *EndpointSelectorImpl) applyDefaults(ctx context.Context, params entity.GenerationParams, req entity.UnifiedGenerationRequest) entity.GenerationParams {
	// 如果用户指定了时长，使用用户指定的
	if req.Duration > 0 {
		params["duration"] = req.Duration
	}

	// 根据端点类型决定是否添加分辨率和比例
	// TTS、voice_clone、text2audio 等音频端点不需要分辨率
	subType, _ := params["__sub_type"].(string)
	if subType != "tts" && subType != "voice_clone" && subType != "text2audio" {
		// 如果用户指定了质量，使用用户指定的
		if req.Quality != "" {
			params["resolution"] = req.Quality
		} else {
			params["resolution"] = "1080p" // 默认1080p（所有Vidu模型均支持）
		}

		// 如果用户指定了比例，使用用户指定的
		if req.AspectRatio != "" {
			params["aspect_ratio"] = req.AspectRatio
		} else {
			params["aspect_ratio"] = "16:9" // 默认16:9
		}
	}

	// 高级参数合并（最后执行——用户显式值覆盖以上默认）
	mergeAdvancedParams(params, req.Params)

	log.Printf("[EndpointSelector][DEBUG] 端点=%s 参数keys=%v 用户高级参数=%d个", subType, paramKeysSorted(params), len(req.Params))
	return params
}

// paramKeysSorted 日志用：参数 key 排序列表（值不打——脱敏）。
func paramKeysSorted(p entity.GenerationParams) []string {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
