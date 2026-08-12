package generation

import (
	"fmt"
	"strings"

	"webreaper/internal/domain/entity"
)

// translateRefs 提示词翻译层（核心：统一所有输入 × 所有模式的 @引用翻译）。
//
// 输入：prompt 中的 @名称 标记 + 结构化引用清单（图片/音频/视频，来自客户端素材库）。
// 输出：按 端点×能力向量 把引用映射为该端点需要的参数格式，同时把 prompt 中的
//       @名称 还原为纯名称文本（去除引用语法，避免干扰上游提示词）。
//
// 翻译规则（端点能力驱动，新增端点只需扩展此表）：
//   - 图引用 image：支持图片的端点并入 images（multiframe → start_image 优先；
//     digital_human → image；subject → images；reference2video 非主体模式 → images）
//   - 音频引用 audio：voice_clone/digital_human → audio_url
//   - 视频引用 video：reference2video（q2-pro VideoSlots>0）→ videos
//   - 类型不匹配（如文生视频引用图片）→ 明确报错，绝不静默丢失
func translateRefs(subType string, cap entity.ModelCapability, params entity.GenerationParams, refs []entity.PromptRef) (entity.GenerationParams, error) {
	if len(refs) == 0 {
		return params, nil
	}
	// ① prompt 中 @名称 → 纯名称（保留语义文本）
	if prompt, ok := params["prompt"].(string); ok && prompt != "" {
		for _, r := range refs {
			if r.Name != "" {
				prompt = strings.ReplaceAll(prompt, "@"+r.Name, r.Name)
			}
		}
		params["prompt"] = prompt
	}

	// ② 按类型聚合引用
	var imgs, audios, videos []string
	for _, r := range refs {
		switch r.Kind {
		case entity.RefKindImage:
			imgs = append(imgs, r.URL)
		case entity.RefKindAudio:
			audios = append(audios, r.URL)
		case entity.RefKindVideo:
			videos = append(videos, r.URL)
		}
	}

	// ③ 按端点翻译
	appendStr := func(key string, vals []string) {
		if len(vals) == 0 {
			return
		}
		if existing, ok := params[key].([]string); ok {
			params[key] = append(existing, vals...)
		} else {
			params[key] = vals
		}
	}
	has := func(key string) bool {
		v, ok := params[key]
		if !ok {
			return false
		}
		switch t := v.(type) {
		case string:
			return t != ""
		case []string:
			return len(t) > 0
		}
		return true
	}

	switch {
	case subType == "digital_human":
		// 数字人：图 → image（未指定时）；音频 → audio_url（未指定时）
		if len(imgs) > 0 && !has("image") && !has("images") {
			params["image"] = imgs[0]
		}
		if len(audios) > 0 && !has("audio_url") {
			params["audio_url"] = audios[0]
		}
		if len(videos) > 0 {
			return params, fmt.Errorf("数字人模式不支持视频引用")
		}
	case subType == "voice_clone":
		// 声音克隆：音频 → audio_url（必填——客户端可只传引用不手动填）
		if len(audios) == 0 {
			return params, fmt.Errorf("声音克隆需要引用音频素材（@音频）")
		}
		if !has("audio_url") {
			params["audio_url"] = audios[0]
		}
		if len(imgs) > 0 || len(videos) > 0 {
			return params, fmt.Errorf("声音克隆仅支持音频引用")
		}
	case subType == "multiframe":
		// 智能多帧：图 → start_image（未指定时）
		if len(imgs) > 0 && !has("start_image") {
			params["start_image"] = imgs[0]
		}
		if len(audios) > 0 || len(videos) > 0 {
			return params, fmt.Errorf("智能多帧仅支持图片引用")
		}
	case subType == "subject":
		// 主体创建：图 → images（未指定时）
		if len(imgs) > 0 && !has("images") {
			params["images"] = imgs
		}
		if len(audios) > 0 || len(videos) > 0 {
			return params, fmt.Errorf("主体创建仅支持图片引用")
		}
	case subType == "reference2video" && cap.VideoSlots > 0:
		// 参考生视频（q2-pro）：视频 → videos；图 → images
		if len(videos) > 0 && !has("videos") {
			params["videos"] = videos
		}
		appendStr("images", imgs)
		if len(audios) > 0 {
			return params, fmt.Errorf("参考生视频不支持音频引用")
		}
	default:
		// 图片能力端点：图引用并入 images
		if cap.ImageSlots != 0 {
			appendStr("images", imgs)
		} else if len(imgs) > 0 {
			return params, fmt.Errorf("该模式不支持图片引用（%s）", subType)
		}
		if len(audios) > 0 {
			return params, fmt.Errorf("该模式不支持音频引用（%s）", subType)
		}
		if len(videos) > 0 {
			return params, fmt.Errorf("该模式不支持视频引用（%s）", subType)
		}
	}
	return params, nil
}
