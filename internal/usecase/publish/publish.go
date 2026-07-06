// Package publish 实现"向外部系统推送数据"用例。
//
// 核心流程：DataItem → 按 FieldMapping 提取字段 → HTTP POST(带 Headers) → 记录 PublishRecord。
//
// 设计动机（整洁架构）：
//   - 字段映射是数据驱动的（JSON 配置），不硬编码任何外部系统。
//   - HTTP 推送是 IO，通过 net/http 适配器执行，usecase 只编排。
//   - 推送结果记录到 publish_records，支持去重和审计。
package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// PublishUseCase 推送用例。
type PublishUseCase struct {
	sysRepo    port.ExternalSystemRepository
	recRepo    port.PublishRecordRepository
	itemRepo   port.DataItemRepository
	httpClient *http.Client
	logger     port.Logger
	tracer     port.Tracer
	maxRetries int // HTTP 推送失败重试次数（默认 3），仅对可重试错误生效
}

func NewPublishUseCase(
	sysRepo port.ExternalSystemRepository,
	recRepo port.PublishRecordRepository,
	itemRepo port.DataItemRepository,
	logger port.Logger,
) *PublishUseCase {
	if logger == nil {
		logger = port.NopLogger{}
	}
	return &PublishUseCase{
		sysRepo: sysRepo, recRepo: recRepo, itemRepo: itemRepo, logger: logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		tracer:    port.NewNopTracer(),
		maxRetries: 3,
	}
}

// SetTracer 注入链路追踪器（可选，默认 NopTracer 不采集 trace）。
// 采用 setter 而非构造参数，避免改动现有调用方签名。
func (uc *PublishUseCase) SetTracer(t port.Tracer) {
	if t != nil {
		uc.tracer = t
	}
}

// SetMaxRetries 设置 HTTP 推送重试次数（0 表示不重试）。
// 仅对可重试错误（网络错误、5xx、429）生效；4xx 等客户端错误不重试。
func (uc *PublishUseCase) SetMaxRetries(n int) {
	uc.maxRetries = n
}

// RecRepo 暴露推送记录仓储（供 handler 查询记录用）。
func (uc *PublishUseCase) RecRepo() port.PublishRecordRepository { return uc.recRepo }

// PublishInput 推送输入。
type PublishInput struct {
	DataItemID string // 被推送的 DataItem ID
	SystemName string // 目标系统名
}

// PublishOutput 推送输出。
type PublishOutput struct {
	Success    bool
	ExternalID string // 外部系统返回的 ID
	ErrorMsg   string
}

// Publish 执行推送：加载配置 → 去重检查 → 字段映射 → HTTP 发送 → 记录结果。
func (uc *PublishUseCase) Publish(ctx context.Context, in PublishInput) (PublishOutput, error) {
	ctx, span := uc.tracer.StartSpan(ctx, "publish.execute")
	defer span.End()
	span.SetAttribute("data_item_id", in.DataItemID)
	span.SetAttribute("system", in.SystemName)

	log := uc.logger.With(port.String("component", "publish"))

	// 1. 去重检查：已成功推送过则跳过
	if _, err := uc.recRepo.FindDedup(ctx, in.DataItemID, in.SystemName); err == nil {
		log.Info("跳过已推送的内容", port.String("item_id", in.DataItemID), port.String("system", in.SystemName))
		return PublishOutput{Success: true}, nil
	}

	// 2. 加载外部系统配置
	sys, err := uc.sysRepo.FindByName(ctx, in.SystemName)
	if err != nil {
		return PublishOutput{}, fmt.Errorf("外部系统 %q 不存在: %w", in.SystemName, err)
	}
	if !sys.Enabled {
		return PublishOutput{}, fmt.Errorf("外部系统 %q 已禁用", in.SystemName)
	}

	// 3. 加载 DataItem
	item, err := uc.itemRepo.FindByID(ctx, in.DataItemID)
	if err != nil {
		return PublishOutput{}, fmt.Errorf("数据项 %q 不存在: %w", in.DataItemID, err)
	}

	// 4. 按 Mode 决定 payload 构造方式
	var payload map[string]any
	mode := sys.Mode
	if mode == "" {
		mode = entity.PublishModeRaw
	}
	if mode == entity.PublishModeMapping {
		// mapping 模式：按 FieldMapping 把 DataItem 转成目标系统期望的 JSON
		payload, err = uc.applyMapping(item, sys.FieldMapping)
		if err != nil {
			return PublishOutput{}, fmt.Errorf("字段映射失败: %w", err)
		}
	} else {
		// raw 模式：把 DataItem 的原始 JSON 作为目标系统的请求体直接转发。
		// 优先用 RawContent（完整原始 JSON），因为 Content 可能被 field_mapping 覆盖为单字段值。
		// 无需字段映射，LLM 在提示词里已按目标格式生成。
		rawJSON := item.RawContent
		if rawJSON == "" {
			rawJSON = item.Content
		}
		payload, err = uc.parseRawPayload(rawJSON)
		if err != nil {
			return PublishOutput{}, fmt.Errorf("raw 模式解析失败（数据不是合法 JSON）: %w", err)
		}
	}

	// 5. HTTP 推送
	method := sys.Method
	if method == "" {
		method = "POST"
	}
	extID, pushErr := uc.doHTTPWithRetry(ctx, sys.Endpoint, method, sys.Headers, payload)

	// 6. 记录推送结果
	now := time.Now()
	rec := entity.PublishRecord{
		ID: fmt.Sprintf("pub-%d", now.UnixNano()),
		ContentID: in.DataItemID, ContentType: "data_item",
		SystemName: in.SystemName, ResultAt: now, CreatedAt: now,
	}
	out := PublishOutput{}
	if pushErr != nil {
		rec.Success = false
		rec.ErrorMsg = pushErr.Error()
		out.ErrorMsg = pushErr.Error()
		span.RecordError(pushErr)
		span.SetAttribute("publish.success", false)
		log.Warn("推送失败", port.String("system", in.SystemName), port.Err(pushErr))
	} else {
		rec.Success = true
		rec.ExternalID = extID
		out.Success = true
		out.ExternalID = extID
		span.SetAttribute("publish.success", true)
		log.Info("推送成功", port.String("system", in.SystemName), port.String("ext_id", extID))
	}
	_ = uc.recRepo.Save(ctx, rec)

	return out, nil
}

// applyMapping 按 FieldMapping 把 DataItem 字段映射为目标系统的 payload。
//
// FieldMapping 格式：{"本系统字段":"目标字段"}，本系统可用字段：
//   title / content / summary / source_url / tags(逗号拼接)
//
// 例：{"title":"title","content":"stem","summary":"answer_good"}
// DataItem{Title:"x",Content:"y",Summary:"z"} → {"title":"x","stem":"y","answer_good":"z"}
func (uc *PublishUseCase) applyMapping(item entity.DataItem, mappingJSON string) (map[string]any, error) {
	var mapping map[string]string
	if err := json.Unmarshal([]byte(mappingJSON), &mapping); err != nil {
		return nil, fmt.Errorf("解析 field_mapping: %w", err)
	}
	// 本系统字段值池
	pool := map[string]string{
		"title":      item.Title,
		"content":    item.Content,
		"summary":    item.Summary,
		"source_url": item.SourceURL,
	}
	if len(item.Tags) > 0 {
		tags := ""
		for i, t := range item.Tags {
			if i > 0 { tags += "," }
			tags += t
		}
		pool["tags"] = tags
	}
	// metadata 也放进池（如 "metadata.difficulty" 可映射）
	for k, v := range item.Metadata {
		pool["metadata."+k] = v
	}

	result := make(map[string]any, len(mapping))
	for srcField, dstField := range mapping {
		if val, ok := pool[srcField]; ok {
			result[dstField] = val
		}
	}
	return result, nil
}

// parseRawPayload 把 DataItem.Content 作为目标系统的请求体 JSON 直接解析。
//
// raw 模式的设计动机：当 Agent 提示词里已要求 LLM 按目标系统的请求体格式生成 JSON，
// 那 DataItem.Content 已经是完整的目标请求体，无需做字段映射，直接解析后转发即可。
//
// 容错：Content 可能被 markdown 代码块包裹（```json...```），用 extractJSONObject 提取。
func (uc *PublishUseCase) parseRawPayload(content string) (map[string]any, error) {
	jsonStr := extractJSONString(content)
	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		return nil, fmt.Errorf("content 不是合法 JSON 对象: %w", err)
	}
	return payload, nil
}

// extractJSONString 从可能含 markdown 包裹的文本中提取第一个 JSON 对象字符串。
func extractJSONString(s string) string {
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			start = i
			break
		}
	}
	if start < 0 {
		return s
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

// doHTTP 执行单次 HTTP 推送，返回外部系统响应里的 ID（尽力提取）。
// 同时透出 status code 和 Retry-After，供重试逻辑判断。
//
// 保持纯：单次调用，不含重试逻辑（重试在 doHTTPWithRetry 里）。
func (uc *PublishUseCase) doHTTP(ctx context.Context, endpoint, method, headersJSON string, payload map[string]any) (extID string, statusCode int, retryAfter string, err error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", 0, "", fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, "", fmt.Errorf("build request: %w", err)
	}
	// 应用自定义请求头
	if headersJSON != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersJSON), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		// 网络错误（DNS/连接/超时）：status=0，可重试
		return "", 0, "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 400 {
		// 透出 Retry-After 头（429/503 常见），供重试逻辑尊重服务端建议
		return "", resp.StatusCode, resp.Header.Get("Retry-After"),
			fmt.Errorf("外部系统返回 %d: %s", resp.StatusCode, string(respBody))
	}

	// 尽力从响应里提取 ID（常见格式 {id:"xxx"} 或 {data:{id:"xxx"}}）
	extID = extractID(respBody)
	return extID, resp.StatusCode, "", nil
}

// isRetryableError 判断错误是否值得重试。
//
// 重试策略（区分技术性临时故障 vs 业务性永久错误）：
//   - 网络错误（status==0，DNS/连接/超时）：重试
//   - HTTP 5xx（502/503/504，服务端临时故障）：重试
//   - HTTP 429（限流）：重试（尊重 Retry-After）
//   - HTTP 4xx（400/401/403/404，客户端错误）：不重试（重试无意义）
func isRetryableError(statusCode int) bool {
	if statusCode == 0 {
		return true // 网络错误
	}
	if statusCode >= 500 {
		return true // 服务端错误
	}
	if statusCode == 429 {
		return true // 限流
	}
	return false
}

// doHTTPWithRetry 执行带重试的 HTTP 推送。
//
// 指数退避：1s → 2s → 4s（与 worker 一致）。
// 429 时若有 Retry-After 头，用其值覆盖退避（尊重服务端限流建议）。
// 抽成方法便于单测（mock httpClient 或 httptest.Server）。
func (uc *PublishUseCase) doHTTPWithRetry(ctx context.Context, endpoint, method, headersJSON string, payload map[string]any) (string, error) {
	maxRetries := uc.maxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		extID, statusCode, retryAfter, err := uc.doHTTP(ctx, endpoint, method, headersJSON, payload)
		if err == nil {
			return extID, nil // 成功
		}
		lastErr = err
		if !isRetryableError(statusCode) {
			return "", err // 不可重试错误（4xx 等），立即返回
		}
		if attempt == maxRetries {
			break // 重试用尽
		}
		// 计算退避：默认指数退避，429/503 优先用 Retry-After
		backoff := time.Duration(1<<attempt) * time.Second // 1s, 2s, 4s
		if retryAfter != "" {
			if secs, parseErr := parseRetryAfter(retryAfter); parseErr == nil && secs >= 0 {
				backoff = time.Duration(secs) * time.Second
			}
		}
		uc.logger.Warn("推送将重试",
			port.Int("attempt", attempt+1),
			port.Int("max", maxRetries+1),
			port.String("backoff", backoff.String()),
			port.Int("status", statusCode),
			port.Err(err))
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return "", ctx.Err() // 被取消，放弃
		}
	}
	return "", lastErr
}

// parseRetryAfter 解析 Retry-After 头（秒数形式；忽略 HTTP-date 形式）。
func parseRetryAfter(v string) (int, error) {
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %q", v)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// extractID 从推送响应里尽力提取 ID（兼容几种常见 JSON 格式）。
func extractID(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	// 直接 id
	if id, ok := m["id"].(string); ok && id != "" {
		return id
	}
	// data.id
	if data, ok := m["data"].(map[string]any); ok {
		if id, ok := data["id"].(string); ok && id != "" {
			return id
		}
	}
	// task_id
	if id, ok := m["task_id"].(string); ok && id != "" {
		return id
	}
	return ""
}
