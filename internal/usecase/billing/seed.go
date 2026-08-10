package billing

import (
	"context"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// DefaultPlans 返回内置默认套餐（seed 用）。
//
// 配额设计依据经济系统分析（与本仓库 Docs 同口径）：
//   - monitor（品牌监测）：高频烧 token，是订阅制主要成本——免费版严格限额
//   - content-gen / content-opt（内容生成/优化）：每次烧 token，按篇限额
//   - chat（对话）：按月消息数限额
//   - -1 = 无限；缺省 key = 0（不允许使用该场景）
//
// 与 usecase 内部 fallback 同源——套餐表清空/未 seed 时行为一致。
func DefaultPlans() []entity.Plan {
	now := time.Now()
	return []entity.Plan{
		{
			ID: "plan-free", Name: "免费版", Level: "free", PriceCents: 0, SortOrder: 1,
			Quotas: map[string]int{
				"monitor":     30,  // 每月 30 次监测（手动为主）
				"content-gen": 5,   // 每月 5 篇内容生成
				"content-opt": 10,  // 每月 10 次内容优化
				"chat":        100, // 每月 100 条对话
			},
			Features:  []string{}, // 无高级功能
			Status:    entity.PlanStatusActive,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "plan-pro", Name: "专业版", Level: "pro", PriceCents: 29900, SortOrder: 2, // ¥299/月
			Quotas: map[string]int{
				"monitor":     500,  // 自动盯盘可用
				"content-gen": 50,
				"content-opt": 100,
				"chat":        2000,
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
				"monitor":     -1, // 无限
				"content-gen": -1,
				"content-opt": -1,
				"chat":        -1,
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

// SeedPlans 首次启动写入内置默认套餐（已存在则跳过，保留运营修改）。
// 调用方：main 装配时执行；失败仅记日志不阻断启动（降级到无套餐状态）。
func SeedPlans(ctx context.Context, repo port.PlanRepository) error {
	for _, p := range DefaultPlans() {
		if _, err := repo.FindByID(ctx, p.ID); err == nil {
			continue // 已存在（可能被 admin 改过）——不覆盖
		}
		if err := repo.Save(ctx, p); err != nil {
			return err
		}
	}
	return nil
}
