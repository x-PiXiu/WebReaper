package port

import "context"

// KeywordSource 关键词来源策略（策略模式接口）。
//
// 每种来源（品牌信息/用户文本/种子词拓展/文件内容/网络爬取）一个实现，可互换、可扩展。
// 新增来源 = 加一个策略实现，零改现有代码（开闭原则）。
//
// 注意：此接口定义在 port 层（非 usecase/geo 包），避免适配器层反向依赖用例层。
type KeywordSource interface {
	// SourceName 来源标识（"brand"/"text"/"seed"/"file"/"web"）。
	SourceName() string
	// Distill 从该来源蒸馏出关键词。
	Distill(ctx context.Context, in KeywordSourceInput) ([]string, error)
}

// KeywordSourceInput 关键词蒸馏的输入（用例层 DTO）。
type KeywordSourceInput struct {
	TenantID  string   // 租户 ID
	BrandID   string   // 品牌来源用
	Text      string   // 文本/文件来源用
	Seeds     []string // 种子词来源用
	Topic     string   // 网络来源用
	LLMConfig string   // 指定 LLM 配置名
}

// WebSearcher 全网搜索抽象（用例层声明，适配器实现）。
// 让关键词生成能结合"全网在搜什么"，而非只凭品牌信息拍脑袋。
type WebSearcher interface {
	SearchByBrand(ctx context.Context, brandName, positioning string, competitors []string) (string, error)
}
