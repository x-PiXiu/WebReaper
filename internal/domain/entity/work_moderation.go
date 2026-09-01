package entity

import "time"

// WorkModeration 作品处置记录（32号：管理与内容安全）。
// work_key 与 WorksUseCase 的 WorkItem.ID 同构（g-{taskID} / c-{contentID}）；
// 物理不删源数据（B-Roll 血缘/发布关联留痕）——处置是运营动作不是任务属性。
type WorkModeration struct {
	ID        string    `json:"id"`
	WorkKey   string    `json:"work_key"`   // g-{taskID}（成片）/ c-{contentID}（文章）
	WorkKind  string    `json:"work_kind"`  // article / video / image / audio
	TenantID  string    `json:"tenant_id"`  // 归属租户（跨租户巡查定位）
	Action    string    `json:"action"`     // hidden（下架）/ deleted（逻辑删除：不可见+不可发布）
	Reason    string    `json:"reason"`     // 处置原因（审计必填）
	Operator  string    `json:"operator"`   // admin 操作者（审计）
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 作品处置动作。
const (
	WorkActionHidden  = "hidden"  // 下架：用户端不可见
	WorkActionDeleted = "deleted" // 逻辑删除：不可见 + 发布拦截（最高处置）
)

// Active 处置是否生效（deleted 恢复语义走 Restore 清记录）。
func (m WorkModeration) Active() bool {
	return m.Action == WorkActionHidden || m.Action == WorkActionDeleted
}
