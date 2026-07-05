package entity

import "time"

// ItemStatus 数据项的审核状态。
type ItemStatus string

const (
	ItemStatusPendingReview ItemStatus = "pending_review" // 待人工/Agent审核
	ItemStatusApproved      ItemStatus = "approved"       // 审核通过（可向量化/检索）
	ItemStatusRejected      ItemStatus = "rejected"       // 审核拒绝
)

// DataItem 是通用数据项——一条采集结果。
//
// 这是平台的核心数据模型，替代了之前特定于招聘的 JobPost/Knowledge/InterviewQuestion。
// DataItem 不预设内容类型，title/content/summary/tags 全由 Agent 的 LLM
// 根据其系统提示词动态生成。平台不知道"面试题"或"招聘需求"是什么——
// 它只知道"这是 Agent 采集并加工的一条数据"。
//
// 分类通过 tags 实现（LLM 自动打标签），用户用标签筛选。
type DataItem struct {
	ID           string
	CollectionID string            // 所属采集集合
	Title        string            // LLM 提取的标题
	Content      string            // 清洗后的正文
	Summary      string            // LLM 生成的摘要
	Tags         []string          // LLM 生成的标签（分类维度）
	SourceURL    string            // 来源 URL
	RawContent   string            // 原始数据（HTML/JSON，保留备查）
	Status       ItemStatus        // 审核状态
	Metadata     map[string]string // Agent 扩展字段（不同 Agent 可放不同东西）
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsValid 领域规则：有效的数据项必须有来源URL和原始内容。
// Title/Content 可以为空（爬虫刚采集尚未LLM结构化的原始数据也应当落库）。
func (d DataItem) IsValid() bool {
	return d.SourceURL != "" && (d.Content != "" || d.RawContent != "")
}

// IsPendingReview 判断是否待审核。
func (d DataItem) IsPendingReview() bool {
	return d.Status == ItemStatusPendingReview
}
