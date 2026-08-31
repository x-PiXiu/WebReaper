// subject_asset.go 主体资产实体（26 号计划——从 generation_tasks 物化到独立资产表）。
//
// 设计动机：
//   - 分身/环境是长期资产，不应寄生在"一次生成行为"的任务行里
//   - 任务清理器（30天）不应误删用户资产
//   - 失败任务不应泄漏到资产列表
package entity

import "time"

// 主体资产作用域常量。
const (
	SubjectScopePersonal = "personal" // 个人分身/环境
	SubjectScopeOfficial = "official" // 官方主体（平台运营）
)

// 主体资产类型常量。
const (
	SubjectKindPerson = "person" // 人物分身
	SubjectKindScene  = "scene"  // 环境主体
)

// 主体资产状态常量。
const (
	SubjectStatusActive   = "active"   // 正常
	SubjectStatusDisabled = "disabled" // 下架（官方主体管理用）
)

// SubjectAsset 主体资产（个人分身/环境 + 官方主体统一表）。
//
// 物化时机：sub_type=subject 任务终态 success 时，server_id 唯一键幂等 upsert。
// 读路径：用户"我的分身"改读本表（不再从 tasks 聚合——失败任务天然不出现）。
type SubjectAsset struct {
	ID             string    // 主键（与任务 ID 同源：task-{nano}）
	TenantID       string    // 租户隔离（官方行用平台运营租户）
	Scope          string    // personal / official
	Kind           string    // person / scene
	Name           string    // 主体名称
	ServerID       string    // Vidu 主体 id（唯一索引——幂等 upsert 键）
	PortraitURL    string    // 封面图 URL（images[0] 或视频封面）
	AvatarVideoURL string    // 链式形象视频产物 URL（成功后回填）
	VoiceID        string    // 绑定音色 ID
	Tags           string    // 标签（JSON 数组或逗号分隔）
	SortOrder      int       // 官方主体排序
	Status         string    // active / disabled
	SourceTaskID   string    // 溯源任务 ID（仅记录，不做外键依赖）
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
