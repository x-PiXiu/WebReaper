package billing

import (
	"context"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// DefaultPlans 返回内置默认套餐（seed 用）。
//
// ⚠️ 配额语义（X-01 修正，2026-08-11）：
//   配额计数 = usages 表记录数（每条 LLM 调用记一条，见 GormUsageRecorder.CountSince）。
//   一次监测 = 关键词数 × 引擎数 × 采样数 × 2（提问 + 解析）。例：
//     1 关键词 × 1 引擎 × 5 采样 = 10 条记录
//     10 关键词 × 3 引擎 × 5 采样（"全量监测"）= 300 条记录
//   修正前旧配额（monitor 30/500）按"批次"理解，实际只够 3~1.7 次全量——
//   免费用户连一次完整监测都做不了。修正后按"LLM 调用次数"语义设计。
//
// 成本模型（配置依赖，运营按实际模型单价复核）：
//   每次 LLM 调用平均 ~800 tokens（提问短、解析长）。
//   MiniMax M2.5 市场价约 ¥1/百万 tokens：
//     - free  monitor 500 次 ≈ 40 万 tokens ≈ ¥0.4/月
//     - pro   monitor 8000 次 ≈ 640 万 tokens ≈ ¥6.4/月（对 ¥299 定价毛利充足）
//     - team  无限（30 次全量 ≈ ¥30/月级）
//
// 场景说明：
//   - monitor（监测）：高频烧 token，订阅制主要成本
//   - content-gen / content-opt（生成/优化）：每次烧 token，按篇限额
//   - chat（对话）：按月消息数限额
//   - nearby（附近同行 POI 搜索）：地图 API 调用（非 LLM，配额按次计）
//   - diagnose（诊断）：RAG + LLM 建议，按次限额
//   - -1 = 无限；缺省 key = 0（不允许使用该场景）
//
// 与 usecase 内部 fallback 同源——套餐表清空/未 seed 时行为一致。
func DefaultPlans() []entity.Plan {
	now := time.Now()
	return []entity.Plan{
		{
			ID: "plan-free", Name: "免费版", Level: "free", PriceCents: 0, SortOrder: 1,
			Quotas: map[string]int{
				"monitor":         500, // ≈17 次单引擎单关键词监测，或 1.7 次全量（体验门槛：能完整跑通闭环）
				"content-gen":     20,  // 每月 20 篇内容生成
				"content-opt":     30,  // 每月 30 次内容优化
				"chat":            200, // 每月 200 条对话
				"nearby":          30,  // 每月 30 次附近同行搜索（地图 API）
				"diagnose":        10,  // 每月 10 次诊断
				"keyword-distill": 30,  // 每月 30 次关键词蒸馏
			},
			Features:  []string{}, // 无高级功能
			Status:    entity.PlanStatusActive,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "plan-pro", Name: "专业版", Level: "pro", PriceCents: 29900, SortOrder: 2, // ¥299/月
			Quotas: map[string]int{
				"monitor":         8000, // ≈27 次全量监测（自动盯盘主力）
				"content-gen":     150,
				"content-opt":     400,
				"chat":            3000,
				"nearby":          300,
				"diagnose":        80,
				"keyword-distill": 300,
			},
			Features: []string{
				"auto-monitor",  // 自动盯盘
				"scheduled-publish", // 定时发送
				"index-verify", // 收录验证
			},
			Status:    entity.PlanStatusActive,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "plan-team", Name: "团队版", Level: "team", PriceCents: 89900, SortOrder: 3, // ¥899/月
			Quotas: map[string]int{
				"monitor":         -1, // 无限
				"content-gen":     -1,
				"content-opt":     -1,
				"chat":            -1,
				"nearby":          -1,
				"diagnose":        -1,
				"keyword-distill": -1,
			},
			Features: []string{
				"auto-monitor",
				"scheduled-publish",
				"index-verify",
				"video",         // 视频生成
				"multi-account", // 多账号矩阵
				"rag-enhance",   // RAG 内容增强
			},
			Status:    entity.PlanStatusActive,
			CreatedAt: now, UpdatedAt: now,
		},
	}
}

// SeedPlans 写入/升级内置默认套餐（幂等，兼容已部署库）。
//
// 策略：
//   - 套餐不存在 → 完整写入（首次部署）
//   - 套餐已存在 → 只补齐缺失场景的配额（如旧库没有 nearby/diagnose）——
//     不覆盖运营在管理后台调整过的场景值（保留"运营修改优先"原设计）。
//   - ⚠️ 配额语义修正（X-01）：旧库 monitor=30/500 是按"批次"理解的失衡值，
//     已存在则不会自动改——需要运营在管理后台按新语义（LLM 调用次数）调整，
//     参考 DefaultPlans 注释的成本模型。
func SeedPlans(ctx context.Context, repo port.PlanRepository) error {
	for _, p := range DefaultPlans() {
		existing, err := repo.FindByID(ctx, p.ID)
		if err != nil {
			// 套餐不存在 → 完整写入
			if err := repo.Save(ctx, p); err != nil {
				return err
			}
			continue
		}
		// 套餐已存在 → 版本升级：只补齐缺失场景配额
		changed := false
		for k, v := range p.Quotas {
			if _, ok := existing.Quotas[k]; !ok {
				if existing.Quotas == nil {
					existing.Quotas = map[string]int{}
				}
				existing.Quotas[k] = v
				changed = true
			}
		}
		if changed {
			if err := repo.Save(ctx, existing); err != nil {
				return err
			}
		}
	}
	return nil
}
