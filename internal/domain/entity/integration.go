package entity

import "time"

// ---- 能力路由模型（能力路由优先，厂商是供货商）----

// IntegrationVendor 厂商 = 一套凭据。小米 MiMo 存一行，
// 同时提供 LLM + ASR + TTS 三种能力（三条 IntegrationCapability）。
type IntegrationVendor struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	BaseURL   string    `json:"base_url"`
	APIKey    string    `json:"api_key"`
	Protocol  string    `json:"protocol"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IntegrationCapability 能力路由条目 = "我需要 ASR 用谁"的答案。
// 同一 Vendor 可出现在多条 Capability（MiMo 既有 LLM 又有 ASR）。
// 同 CapabilityID 下 IsDefault 互斥——路由依据。
type IntegrationCapability struct {
	ID        string    `json:"id"`         // 主键："{CapabilityID}#{VendorID}"
	CapID     string    `json:"cap_id"`
	VendorID  string    `json:"vendor_id"`
	Endpoint  string    `json:"endpoint"`
	Model     string    `json:"model"`
	IsDefault bool      `json:"is_default"`
	Enabled   bool      `json:"enabled"`
	ExtraJSON string    `json:"extra_json"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResolvedCap 能力解析结果（adapter 层的配置投影——用例层只读）。
type ResolvedCap struct {
	VendorID  string `json:"vendor_id"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	Protocol  string `json:"protocol"`
	Endpoint  string `json:"endpoint"`
	Model     string `json:"model"`
	ExtraJSON string `json:"extra_json"`
}
