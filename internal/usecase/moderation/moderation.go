// Package moderation 实现内容机审用例（32号 P2 第一批：文本审核）。
//
// 设计原则（32号 §六）：
//   - 机器只标记（flagged），处置权永远在管理员——flagged 不下架、不拦发布、用户无感；
//     管理员在 Works Tab 待复核队列中决定 处置（hidden/deleted）或 放行（清记录）。
//   - 全异步（LLM 调用秒级，不阻断业务主流程）；失败静默（审核不可用≠业务不可用）。
//   - 复用 port.AIGenerator（LLM 配置体系/默认模型热切换现成），零新传输依赖。
//   - 总开关 gen_moderation_enabled 默认关（灰度），审核提示词常量保守默认。
//
// 触发点（第一批三处文本）：生成提交（口播文案/克隆试听文案）、GEO 内容生成后、
// 发布前（标题+正文复检）。图片/音频（ASR 转文本）为第二批。
package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// defaultPrompt 保守默认审核提示词（政治/色情/暴力/违法广告/医疗夸大五类）。
// 输出契约：仅一个 JSON 对象 {"flag":bool,"category":"","reason":""}。
const defaultPrompt = `你是内容安全审核员。审核下面的用户内容是否违反平台规范。
违规类目（仅限以下五类）：
1. politics：政治敏感、危害国家安全、煽动性政治内容
2. porn：色情、低俗、性暗示
3. violence：暴力、血腥、恐怖主义、自残教唆
4. illegal_ad：违法广告（赌博、诈骗、代开发票、违禁品售卖）
5. medical_exaggeration：医疗健康夸大宣传（包治愈、绝对化疗效承诺）

判定标准：营销文案的正常商业表述（卖点、价格、优惠）不违规；仅当内容明确落入上述类目才标记。
只输出一个 JSON 对象，不要输出其他文字：{"flag":true/false,"category":"类目英文或空","reason":"30字内判定理由"}`

// Moderator 内容机审（P2 第一批文本 + 第二批阻断档/图片/音频）。
type Moderator struct {
	llm         port.AIGenerator           // LLM（复用默认配置，热切换）
	modRepo     port.WorkModerationRepository
	settingRepo port.SystemSettingRepository // 开关（gen_moderation_enabled / gen_moderation_block）
	vision      port.VisionChat             // 可选（第二批：图片机审——MiMo 多模态）
	transcriber port.SpeechTranscriber      // 可选（第二批：音频 ASR 回审）
}

func NewModerator(llm port.AIGenerator, mr port.WorkModerationRepository, sr port.SystemSettingRepository) *Moderator {
	return &Moderator{llm: llm, modRepo: mr, settingRepo: sr}
}

// SetVision 注入图片机审通道（可选；缺省图片内容不审）。
func (m *Moderator) SetVision(v port.VisionChat) {
	if v != nil {
		m.vision = v
	}
}

// SetTranscriber 注入语音转写（可选；缺省音频内容不审）。
func (m *Moderator) SetTranscriber(t port.SpeechTranscriber) {
	if t != nil {
		m.transcriber = t
	}
}

// BlockEnabled 高危阻断档（第二批）：开启后高危类目（politics/porn/violence）
// 在生成提交时同步判定并直接拒绝（代价：提交时延 +1~3s）。默认关。
func (m *Moderator) BlockEnabled(ctx context.Context) bool {
	if !m.Enabled(ctx) {
		return false
	}
	s, err := m.settingRepo.Get(ctx, entity.SettingKeyGenModerationBlock)
	if err != nil {
		return false
	}
	switch strings.TrimSpace(s.Value) {
	case "true", "1", "on", "yes":
		return true
	default:
		return false
	}
}

// highRiskCategories 高危类目（阻断档拒绝范围——营销夸大类不阻断仅标记）。
var highRiskCategories = map[string]bool{
	"politics": true, "porn": true, "violence": true,
}

// IsHighRisk 高危类目判定（阻断档触发条件）。
func IsHighRisk(v textVerdict) bool {
	return v.Flag && highRiskCategories[v.Category]
}

// CheckTextSync 同步文本判定（阻断档专用——调用方在提交路径，接受秒级时延）。
// 总开关关闭时返回通过（零开销）。
func (m *Moderator) CheckTextSync(ctx context.Context, text string) (textVerdict, error) {
	if !m.Enabled(ctx) {
		return textVerdict{}, nil
	}
	return m.moderate(ctx, text)
}

// ModerateImageAsync 异步图片机审（第二批）：产物图 URL → 多模态五类目判定
// → flagged（不阻断——图片产物审核在后置路径，处置权在管理员）。
func (m *Moderator) ModerateImageAsync(tenantID, workKey, imageURL string) {
	if m == nil || m.vision == nil || workKey == "" || imageURL == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if !m.Enabled(ctx) {
			return
		}
		out, err := m.vision.ChatWithImage(ctx, visionPrompt, imageURL)
		if err != nil {
			log.Printf("[moderation] 图片审核失败（静默）key=%s err=%v", workKey, err)
			return
		}
		v := parseVerdict(out)
		if !v.Flag {
			return
		}
		if uErr := m.modRepo.Upsert(ctx, entity.WorkModeration{
			ID:       fmt.Sprintf("wm-%d", time.Now().UnixNano()),
			WorkKey:  workKey, WorkKind: "image", TenantID: tenantID,
			Action: entity.WorkActionFlagged, Reason: v.Category + "：" + v.Reason,
			Operator: "machine",
		}); uErr != nil {
			log.Printf("[moderation] 图片 flagged 写入失败 key=%s err=%v", workKey, uErr)
		} else {
			log.Printf("[moderation] 图片已标记待复核 key=%s category=%s", workKey, v.Category)
		}
	}()
}

// ModerateAudioAsync 异步音频机审（第二批）：下载音频 → ASR 转写 → 文本五类目
// 判定 → flagged。触发场景：克隆样本（声音滥用/违规口播内容）。
func (m *Moderator) ModerateAudioAsync(tenantID, workKey, audioURL string) {
	if m == nil || m.transcriber == nil || workKey == "" || audioURL == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if !m.Enabled(ctx) {
			return
		}
		text, err := m.transcribeAudio(ctx, audioURL)
		if err != nil || strings.TrimSpace(text) == "" {
			if err != nil {
				log.Printf("[moderation] 音频转写失败（静默）key=%s err=%v", workKey, err)
			}
			return // 无语音/失败不标记（宁漏勿误杀）
		}
		v, err := m.moderate(ctx, text)
		if err != nil {
			log.Printf("[moderation] 音频文本审核失败（静默）key=%s err=%v", workKey, err)
			return
		}
		if !v.Flag {
			return
		}
		if uErr := m.modRepo.Upsert(ctx, entity.WorkModeration{
			ID:       fmt.Sprintf("wm-%d", time.Now().UnixNano()),
			WorkKey:  workKey, WorkKind: "audio", TenantID: tenantID,
			Action: entity.WorkActionFlagged, Reason: v.Category + "：" + v.Reason,
			Operator: "machine",
		}); uErr != nil {
			log.Printf("[moderation] 音频 flagged 写入失败 key=%s err=%v", workKey, uErr)
		} else {
			log.Printf("[moderation] 音频已标记待复核 key=%s category=%s", workKey, v.Category)
		}
	}()
}

// Enabled 机审总开关（默认关——灰度开启）。
func (m *Moderator) Enabled(ctx context.Context) bool {
	if m == nil || m.llm == nil || m.modRepo == nil {
		return false
	}
	if m.settingRepo == nil {
		return false // 未接设置仓储=保守关闭
	}
	s, err := m.settingRepo.Get(ctx, entity.SettingKeyGenModerationEnabled)
	if err != nil || s.Value == "" {
		return false
	}
	switch strings.TrimSpace(s.Value) {
	case "true", "1", "on", "yes":
		return true
	default:
		return false
	}
}

// ModerateTextAsync 异步审核文本（fire-and-forget：flagged 落 machine 记录，失败静默）。
// workKey 传空则不落记录（仅审不记的场景不存在，当前必传）。
func (m *Moderator) ModerateTextAsync(tenantID, workKey, kind, text string) {
	if m == nil || workKey == "" || strings.TrimSpace(text) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if !m.Enabled(ctx) {
			return
		}
		v, err := m.moderate(ctx, text)
		if err != nil {
			log.Printf("[moderation] 审核失败（静默，不阻断业务）key=%s err=%v", workKey, err)
			return
		}
		if !v.Flag {
			return // 通过——不留痕（避免表膨胀；巡查流只看被标记的）
		}
		if uErr := m.modRepo.Upsert(ctx, entity.WorkModeration{
			ID:       fmt.Sprintf("wm-%d", time.Now().UnixNano()),
			WorkKey:  workKey, WorkKind: kind, TenantID: tenantID,
			Action: entity.WorkActionFlagged, Reason: v.Category + "：" + v.Reason,
			Operator: "machine",
		}); uErr != nil {
			log.Printf("[moderation] flagged 记录写入失败 key=%s err=%v", workKey, uErr)
		} else {
			log.Printf("[moderation] 已标记待复核 key=%s category=%s reason=%s", workKey, v.Category, v.Reason)
		}
	}()
}

// textVerdict 审核判定（LLM JSON 输出契约）。
type textVerdict struct {
	Flag     bool   `json:"flag"`
	Category string `json:"category"`
	Reason   string `json:"reason"`
}

// moderate 单轮无状态调用 + 宽松 JSON 提取（LLM 可能带 markdown 围栏）。
func (m *Moderator) moderate(ctx context.Context, text string) (textVerdict, error) {
	truncated := []rune(text)
	if len(truncated) > 6000 { // 审核上限（防超长正文撑爆上下文）
		truncated = truncated[:6000]
	}
	out, err := m.llm.ChatStream(ctx, "", "default", []port.ChatMessage{
		{Role: "user", Content: defaultPrompt + "\n\n---用户内容开始---\n" + string(truncated) + "\n---用户内容结束---"},
	}, nil)
	if err != nil {
		return textVerdict{}, err
	}
	return parseVerdict(out), nil
}

// parseVerdict 宽松解析：截取首个 {..} JSON 对象；解析失败视为通过（宁漏勿误杀）。
func parseVerdict(out string) textVerdict {
	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return textVerdict{}
	}
	var v textVerdict
	if err := json.Unmarshal([]byte(out[start:end+1]), &v); err != nil {
		return textVerdict{} // 输出不规范=通过（保守：不因模型格式问题误标用户）
	}
	return v
}

// visionPrompt 图片审核提示词（五类目同文本版；判定营销图正常商业呈现不违规）。
const visionPrompt = `你是图片内容安全审核员。审核这张图片是否违反平台规范。
违规类目（仅限以下五类）：
1. politics：政治敏感、危害国家安全、煽动性政治内容
2. porn：色情、低俗、性暗示
3. violence：暴力、血腥、恐怖主义
4. illegal_ad：违法广告（赌博、诈骗、违禁品售卖标识）
5. medical_exaggeration：医疗健康夸大宣传（包治愈、绝对化疗效承诺）

判定标准：商品图/门店图/二维码等正常商业营销呈现不违规；仅当图片明确落入上述类目才标记。
只输出一个 JSON 对象，不要输出其他文字：{"flag":true/false,"category":"类目英文或空","reason":"30字内判定理由"}`

// transcribeAudio 下载音频到临时文件 → SpeechTranscriber 转写。
// 下载属审核编排 IO（本站托管地址只有本服务可达——与生成域内联同一问题域，
// 为避免单次下载新增端口而直用 http；SSRF 语义同生成域：私网地址在此是合法目标）。
func (m *Moderator) transcribeAudio(ctx context.Context, audioURL string) (string, error) {
	if !strings.HasPrefix(audioURL, "http://") && !strings.HasPrefix(audioURL, "https://") {
		return "", fmt.Errorf("音频地址无效")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("音频下载 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20)) // ≤20MB（素材上传同限）
	if err != nil {
		return "", err
	}
	ext := ".mp3"
	switch strings.ToLower(path.Ext(strings.Split(audioURL, "?")[0])) {
	case ".wav":
		ext = ".wav"
	case ".m4a":
		ext = ".m4a"
	}
	tmp, err := os.CreateTemp("", "mod-audio-*"+ext)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()
	mime := "audio/mpeg"
	if ext == ".wav" {
		mime = "audio/wav"
	}
	return m.transcriber.Transcribe(ctx, tmp.Name(), mime, int64(len(data)))
}
