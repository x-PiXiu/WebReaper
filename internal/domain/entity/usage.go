package entity

import "time"

// UsageRecord LLM 用量记录（经济系统计量基础）。
//
// 设计动机（为计费预留——用量是横切关注点，所有 LLM 调用都应可计量）：
//   - 按租户计量：谁用了多少 token，账本清晰（多租户隔离铁律的延伸）
//   - Scene 标识调用场景（chat/monitor/content-gen/content-opt/...），
//     便于区分"用户消耗"与"平台后台消耗"（如每日自动监测）
//   - 落库后可支撑：套餐额度、用量报表、账单、限流告警
type UsageRecord struct {
	ID               string
	TenantID         string // 空 = 平台后台消耗（定时任务等）
	UserID           string
	Scene            string // chat / monitor / content-gen / content-opt / video / orchestrate ...
	LLMConfigName    string // 用了哪个 LLM 配置（default / deepseek / doubao...）
	Model            string // 实际模型名
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	LLMCalls         int // 一次记录聚合的调用次数
	CreatedAt        time.Time
}
