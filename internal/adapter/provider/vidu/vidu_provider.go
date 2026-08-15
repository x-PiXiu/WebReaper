// Package vidu 提供 Vidu 生成服务商的协议层实现（port.GenerationProvider）。
//
// 设计（Docs/Plans/03 计划文档 §2.2）：本包只表达"任务协议"——
// 提交/轮询/取消/回调验签/积分查询/错误翻译，与具体端点无关（端点路径由
// viduendpoint 策略提供）。端点差异在 viduendpoint 包，模型差异在能力向量。
// 换服务商 = 新 provider 包 + main 装配一行。
package vidu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

const baseURL = "https://api.vidu.cn"

// ViduProvider 是 port.GenerationProvider 的 Vidu 实现。
type ViduProvider struct {
	mu     sync.RWMutex // 保护 apiKey（管理后台热更新）
	apiKey string
	client *http.Client
	// R2 修复多实例 Key 漂移：管理后台改 Key 只更新收到请求的那个实例的内存——
	// 其他实例持旧 Key 直到重启。keySource 存在时 apiKeyNow 优先从 DB 读取
	//（30s TTL 缓存），UpdateAPIKey 仅作为同实例即时生效的旁路。
	keySource    func(ctx context.Context) (string, error)
	keyCache     string        // 最近从 keySource 读到的 Key
	keyCacheAt   time.Time     // 上次读取时间（TTL 内不重读）
	keyCacheTTL  time.Duration // 默认 30s
}

// NewViduProvider 创建 Vidu 协议层适配器。
func NewViduProvider(apiKey string) *ViduProvider {
	return &ViduProvider{
		apiKey:      apiKey,
		client:      &http.Client{Timeout: 60 * time.Second},
		keyCacheTTL: 30 * time.Second,
	}
}

// SetKeySource 注入 Key 数据源（R2：管理后台改 Key 后各实例 ≤30s 对齐——
// main 装配时传入 providerconfig 仓储查询闭包）。
func (p *ViduProvider) SetKeySource(fn func(ctx context.Context) (string, error)) {
	p.keySource = fn
}

// UpdateAPIKey 运行时更新 API Key（同实例即时生效）。
// 实现 port.ConfigurableProvider——main 装配后由 admin handler 调用。
// 多实例对齐靠 keySource TTL 刷新（其他实例 ≤30s 从 DB 拉到新值）。
func (p *ViduProvider) UpdateAPIKey(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.apiKey = key
	// 同步更新缓存，避免 keySource 下轮读到旧值
	p.keyCache = key
	p.keyCacheAt = time.Now()
}

// apiKeyNow 并发安全的 Key 读取。优先走 keySource（DB+TTL——多实例最终一致），
// 降级到内存值（未注入 keySource 或 DB 读取失败时兜底）。
func (p *ViduProvider) apiKeyNow() string {
	// 快路径：keySource 存在且缓存未过期 → 直接用（无锁）
	if p.keySource != nil && time.Since(p.keyCacheAt) < p.keyCacheTTL {
		p.mu.RLock()
		k := p.keyCache
		p.mu.RUnlock()
		if k != "" {
			return k
		}
	}
	p.mu.RLock()
	localKey := p.apiKey
	p.mu.RUnlock()
	if localKey != "" {
		return localKey
	}
	// 慢路径：走 keySource 从 DB 读（带 TTL 写回缓存）
	if p.keySource != nil {
		k, err := p.keySource(context.Background())
		if err == nil && k != "" {
			p.mu.Lock()
			p.keyCache = k
			p.keyCacheAt = time.Now()
			p.mu.Unlock()
			return k
		}
	}
	return localKey
}

func (p *ViduProvider) Name() string { return "vidu" }

var _ port.ConfigurableProvider = (*ViduProvider)(nil)

// Submit 提交生成任务（endpoint 由端点策略提供，body 为组装后的请求体）。
func (p *ViduProvider) Submit(ctx context.Context, endpoint string, body map[string]any) (string, int, error) {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+p.apiKeyNow())

	resp, err := p.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("Vidu 创建任务请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("Vidu 创建任务失败 HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var parsed struct {
		TaskID  string `json:"task_id"`
		Credits int    `json:"credits"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0, fmt.Errorf("解析 Vidu 创建响应失败: %w", err)
	}
	if parsed.TaskID == "" {
		return "", 0, fmt.Errorf("Vidu 响应缺少 task_id: %s", truncate(string(raw), 300))
	}
	return parsed.TaskID, parsed.Credits, nil
}

// Poll 轮询任务状态（GET /ent/v2/tasks/{id}/creations）。
func (p *ViduProvider) Poll(ctx context.Context, taskID string) (port.GenerationStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL+"/ent/v2/tasks/"+taskID+"/creations", nil)
	if err != nil {
		return port.GenerationStatus{}, err
	}
	req.Header.Set("Authorization", "Token "+p.apiKeyNow())

	resp, err := p.client.Do(req)
	if err != nil {
		return port.GenerationStatus{}, fmt.Errorf("Vidu 查询任务失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	if resp.StatusCode != http.StatusOK {
		return port.GenerationStatus{}, fmt.Errorf("Vidu 查询任务 HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var parsed struct {
		State     string `json:"state"`
		ErrCode   string `json:"err_code"`
		Creations []struct {
			ID             string `json:"id"`
			URL            string `json:"url"`
			CoverURL       string `json:"cover_url"`
			WatermarkedURL string `json:"watermarked_url"`
		} `json:"creations"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return port.GenerationStatus{}, fmt.Errorf("解析 Vidu 查询响应失败: %w", err)
	}
	st := port.GenerationStatus{State: parsed.State, ErrCode: parsed.ErrCode}
	for _, c := range parsed.Creations {
		st.Creations = append(st.Creations, entity.CreationItem{
			ID: c.ID, URL: c.URL, CoverURL: c.CoverURL, WatermarkedURL: c.WatermarkedURL,
		})
	}
	return st, nil
}

// Cancel 取消任务。
func (p *ViduProvider) Cancel(ctx context.Context, taskID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/ent/v2/tasks/"+taskID+"/cancel", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+p.apiKeyNow())
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("Vidu 取消任务失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("Vidu 取消任务 HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return nil
}

// QueryCredits 查询剩余积分（GET /ent/v2/credits）。
func (p *ViduProvider) QueryCredits(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/ent/v2/credits", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Token "+p.apiKeyNow())
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Vidu 查询积分失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Vidu 查询积分 HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Credits int `json:"credits"`
	}
	_ = json.Unmarshal(raw, &parsed)
	return parsed.Credits, nil
}

// VerifyCallback 回调验签（HMAC-SHA256 复合签名字符串；验签实现见 callback.go）。
func (p *ViduProvider) VerifyCallback(ctx context.Context, header http.Header, body []byte, requestURI string) error {
	return verifyCallbackSignature(p.apiKeyNow(), header, body, requestURI)
}

// TranslateError 错误码 → 产品级消息（可读 + 语义化）。
func (p *ViduProvider) TranslateError(code string) string {
	if code == "" {
		return ""
	}
	if msg, ok := errorTranslations[code]; ok {
		return msg
	}
	return "生成失败（" + code + "）"
}

// errorTranslations 错误码翻译表（数据源：Docs/第三方/Vidu/任务管理/错误码.md）。
var errorTranslations = map[string]string{
	"CreditInsufficient":        "积分不足，请充值后重试",
	"TaskPromptPolicyViolation": "提示词触发安全审核，请调整描述",
	"AuditSubmitIllegal":        "输入未通过安全审核，请调整内容",
	"CreationPolicyViolation":   "生成物触发风控，请调整描述",
	"QuotaExceeded":             "生成服务并发已满，请稍后重试",
	"TooManyRequests":           "请求过于频繁，请稍后重试",
	"SystemThrottling":          "生成服务繁忙，请稍后重试",
	"InternalServiceFailure":    "生成服务异常，请稍后重试或联系客服",
	"ModelUnavailable":          "所选模型不可用，请更换模型",
	"ImageCheckFaceFailed":      "图片人脸检测失败，请换一张清晰照片",
	"ImageCheckBodyJointsFailed": "图片人体检测失败，请换一张完整照片",
	"ImageObjectsUndetected":    "图片人体或人脸有遮挡，请换一张清晰照片",
	"MultiFaceDetected":         "检测到多张人脸，请上传单人照片",
	"NoFaceDetected":            "检测不到人脸，请上传含清晰人像的照片",
	"FaceDetectNotPass":         "人脸检测不通过，请换一张照片",
	"VideoFormatInvalid":        "视频格式不支持（需 mp4/avi/mov）",
	"ImageFormatInvalid":        "图片格式不支持（需 png/jpeg/jpg/webp）",
	"ImageSizeInvalid":          "图片尺寸过大或过小",
	"VideoDownloadFailure":      "视频素材下载失败，请检查链接有效性",
	"ImageDownloadFailure":      "图片素材下载失败，请检查链接有效性",
	"TaskNotFound":              "任务不存在或已过期",
	"UserCancelled":             "任务已被取消",
	"Unauthorized":              "API Key 无效或已过期",
	"Forbidden":                 "无权访问该资源",
	"BadRequest":                "请求参数不合法",
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
