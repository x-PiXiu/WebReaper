package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// 知识库素材的入库清洗常量（确定性清洗，无 LLM 调用）。
const (
	MaterialMinContentRunes  = 200  // 正文最小长度（rune），低于此丢弃（低质页面过滤）
	MaterialContentMaxRunes  = 3000 // 正文入库截断长度
	MaterialSummaryMaxRunes  = 300  // 摘要截断长度
	MaterialEmbedTextMaxRunes = 800 // 向量化输入文本截断长度
)

// 知识库素材状态。
const (
	MaterialStatusActive   = "active"   // 可检索
	MaterialStatusRejected = "rejected" // 人工/规则拒绝
)

// KnowledgeMaterial 知识库素材——一条带来源的采集结果。
//
// 溯源与去重的双保险：
//   - SourceURL：原始出处（生成内容时标注给 LLM，引用可查证）
//   - URLFingerprint：sha256(SourceURL)，数据库唯一索引做持久化去重
//     （替代爬虫装饰器的内存 map——重启不丢）
//
// 平台级维度（无 tenant_id）：按行业组织，多租户共享检索。
type KnowledgeMaterial struct {
	ID             string
	Industry       string    // 所属行业（如 "餐饮"/"美业"；须与 Brand.Industry 对齐）
	SourceURL      string    // 来源 URL（原文出处）
	URLFingerprint string    // sha256(SourceURL)，唯一索引
	Title          string    // 页面标题
	Content        string    // 清洗后正文（入库截断 MaterialContentMaxRunes）
	Summary        string    // 摘要（正文前 MaterialSummaryMaxRunes 字）
	Tags           []string  // 标签（预留；LLM 打标 P2）
	CrawlKeyword   string    // 本次采集命中的关键词（采集溯源）
	Embedding      []float32 // 向量（title+summary+正文前 MaterialEmbedTextMaxRunes 字）
	Status         string    // MaterialStatusActive / Rejected
	CreatedAt      time.Time
}

// IsValid 领域规则：素材必须有来源 URL、指纹与行业（否则无法溯源/无法归组）。
func (m KnowledgeMaterial) IsValid() bool {
	return m.SourceURL != "" && m.URLFingerprint != "" && m.Industry != ""
}

// IsSearchable 是否可被向量检索（带向量 + 有效状态）。
func (m KnowledgeMaterial) IsSearchable() bool {
	return m.Status == MaterialStatusActive && len(m.Embedding) > 0
}

// MaterialRef 检索返回的素材引用（带来源，供生成注入）。
type MaterialRef struct {
	Title     string  // 标题
	Summary   string  // 摘要（前 MaterialSummaryMaxRunes 字）
	SourceURL string  // 来源 URL——溯源关键字段（注入 prompt 标注出处）
	Score     float32 // 余弦相似度（0-1，越高越相关）
}

// IndustryCrawlConfig 行业采集配置（system_settings 的 kb_crawl_industries 存 JSON 数组）。
type IndustryCrawlConfig struct {
	Industry string   `json:"industry"`  // 行业名（须与 Brand.Industry 对齐）
	Keywords []string `json:"keywords"`  // 采集关键词组（每组独立一轮搜索）
	PerRound int      `json:"per_round"` // 每轮每关键词入库上限（默认 10）
}

// Normalize 填充默认值（PerRound ≤ 0 → 10；Industry 空 → 通用）。
func (c *IndustryCrawlConfig) Normalize() {
	if c.PerRound <= 0 {
		c.PerRound = 10
	}
	if c.Industry == "" {
		c.Industry = "通用"
	}
}

// FingerprintURL 生成 URL 指纹（sha256 hex）——持久化去重的领域规则。
// 采集前先查库去重，避免同一来源重复入库。
func FingerprintURL(u string) string {
	h := sha256.Sum256([]byte(u))
	return hex.EncodeToString(h[:])
}

// ---- 向量嵌入 / 向量库运行时配置（管理后台可改，30s 生效）----

// SettingKeyEmbeddingConfig 向量嵌入 + 向量库的运行时配置键（system_settings 存 JSON）。
const SettingKeyEmbeddingConfig = "kb_embedding_config"

// SettingKeyCrawlIntervalMinutes 采集间隔配置键（分钟；管理后台可改，下个周期生效）。
const SettingKeyCrawlIntervalMinutes = "kb_crawl_interval_minutes"

// DefaultCrawlIntervalMinutes 默认采集间隔（6 小时）。
const DefaultCrawlIntervalMinutes = 360

// 采集间隔合法范围（分钟）。
const (
	CrawlIntervalMinMinutes = 30  // 0.5h——防过频打爆搜索源/配额
	CrawlIntervalMaxMinutes = 1440 // 24h——防间隔过长素材陈旧
)

// 向量库类型。
const (
	VectorDBMySQL  = "mysql"  // MySQL JSON 列 + Go 余弦（默认，零依赖）
	VectorDBMilvus = "milvus" // Milvus 向量库（驱动接入后可用；未接入时明确报错）
)

// EmbeddingRuntimeConfig 向量嵌入模型 + 向量库的运行时配置。
//
// 设计动机（管理后台动态修改、免重启）：
//   - 换向量模型 / 换向量库是运营高频操作（成本/质量权衡），不应要求重启服务。
//   - 存 system_settings（参照 IndexingConfig 先例），未配置时 main 用 env（EMBEDDING_*）兜底。
type EmbeddingRuntimeConfig struct {
	Model   string `json:"model"`   // 嵌入模型名（如 embedding-3）
	BaseURL string `json:"base_url"` // OpenAI 兼容端点（如 https://open.bigmodel.cn/api/paas/v4）
	APIKey  string `json:"api_key"` // 嵌入 API Key
	// Dimensions 向量维度（0 = 不传，用模型默认——智谱 embedding-3 默认 2048，可设 256-2048）。
	// ⚠️ 修改维度后存量向量失效——必须重建向量。
	Dimensions int `json:"dimensions"`
	// VectorDB 向量库类型：mysql（默认）/ milvus（驱动已接入）
	VectorDB string `json:"vector_db"`
	// Milvus 连接参数（vector_db=milvus 时必填）
	MilvusHost       string `json:"milvus_host"`
	MilvusPort       string `json:"milvus_port"`
	MilvusCollection string `json:"milvus_collection"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// IsConfigured 嵌入能力是否可用（model/base_url/api_key 三者齐备）。
func (c EmbeddingRuntimeConfig) IsConfigured() bool {
	return c.Model != "" && c.BaseURL != "" && c.APIKey != ""
}

// EffectiveVectorDB 返回生效的向量库类型（空 = mysql）。
func (c EmbeddingRuntimeConfig) EffectiveVectorDB() string {
	if c.VectorDB == "" {
		return VectorDBMySQL
	}
	return c.VectorDB
}

// Validate 校验配置（空配置合法——表示未启用；非法返回错误描述）。
func (c EmbeddingRuntimeConfig) Validate() error {
	// embedding 凭据：要么全空（禁用向量化）要么全填
	if (c.Model == "") != (c.BaseURL == "") || (c.Model == "") != (c.APIKey == "") {
		return &IndexingConfigError{msg: "model/base_url/api_key 必须同时配置或同时为空"}
	}
	// dimensions：0=模型默认；显式值时校验合理范围（智谱 embedding-3 支持 256-2048）
	if c.Dimensions < 0 || c.Dimensions > 4096 {
		return &IndexingConfigError{msg: "dimensions 必须在 0-4096 之间（0=用模型默认维度）"}
	}
	switch c.EffectiveVectorDB() {
	case VectorDBMySQL:
	case VectorDBMilvus:
		if c.MilvusHost == "" {
			return &IndexingConfigError{msg: "milvus 模式需要配置 milvus_host"}
		}
	default:
		return &IndexingConfigError{msg: "未知 vector_db: " + c.VectorDB + "（仅支持 mysql/milvus）"}
	}
	return nil
}
