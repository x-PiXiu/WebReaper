package repository

import "time"

// ---- 经济系统持久化对象 ----
//
// Quotas（map）与 Features（[]string）用 JSON 字符串存库——
// MySQL 无原生 map 类型，JSON 列既保留结构又便于 admin 直读。

// PlanPO 套餐。
type PlanPO struct {
	ID         string    `gorm:"primaryKey;size:64"`
	Name       string    `gorm:"size:64"`
	Level      string    `gorm:"size:16;index"`
	PriceCents int
	Quotas     string `gorm:"type:text"` // JSON: {"monitor":500,"content-gen":50}
	Features   string `gorm:"type:text"` // JSON: ["auto-monitor","video"]
	Status     string `gorm:"size:16;index"`
	SortOrder  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (PlanPO) TableName() string { return "plans" }

// SubscriptionPO 租户订阅。
type SubscriptionPO struct {
	ID          string    `gorm:"primaryKey;size:64"`
	TenantID    string    `gorm:"size:64;uniqueIndex"` // 一租户一有效订阅
	PlanID      string    `gorm:"size:64;index"`
	Status      string    `gorm:"size:16;index"`
	PeriodStart time.Time
	PeriodEnd   time.Time `gorm:"index"` // 到期预警查询用
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SubscriptionPO) TableName() string { return "subscriptions" }

// OrderPO 订单。
type OrderPO struct {
	ID             string     `gorm:"primaryKey;size:64"`
	TenantID       string     `gorm:"size:64;index"`
	PlanID         string     `gorm:"size:64"`
	AmountCents    int
	Status         string     `gorm:"size:16;index"`
	PaymentGateway string     `gorm:"size:32"`
	PaymentID      string     `gorm:"size:128"`
	CreatedAt      time.Time  `gorm:"index"`
	PaidAt         *time.Time // 支付时间（DATETIME NULL 列，未支付为 NULL）
}

func (OrderPO) TableName() string { return "orders" }
