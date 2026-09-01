package works

// moderation_test.go —— 32号：作品处置用例单测（内存仓储）。

import (
	"context"
	"strings"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

type memModerationRepo struct {
	byKey map[string]entity.WorkModeration
}

func (m *memModerationRepo) FindByKey(_ context.Context, key string) (entity.WorkModeration, error) {
	if v, ok := m.byKey[key]; ok {
		return v, nil
	}
	return entity.WorkModeration{}, pkg.ErrNotFound
}
func (m *memModerationRepo) Upsert(_ context.Context, v entity.WorkModeration) error {
	if m.byKey == nil {
		m.byKey = map[string]entity.WorkModeration{}
	}
	m.byKey[v.WorkKey] = v
	return nil
}
func (m *memModerationRepo) Delete(_ context.Context, key string) error {
	delete(m.byKey, key)
	return nil
}
func (m *memModerationRepo) UpdateAppeal(_ context.Context, key, status, text string, at *time.Time) error {
	if v, ok := m.byKey[key]; ok {
		v.AppealStatus, v.AppealText, v.AppealedAt = status, text, at
		m.byKey[key] = v
	}
	return nil
}
func (m *memModerationRepo) ListByTenant(_ context.Context, tenantID string) ([]entity.WorkModeration, error) {
	var out []entity.WorkModeration
	for _, v := range m.byKey {
		if v.TenantID == tenantID {
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *memModerationRepo) ListRecent(_ context.Context, limit int) ([]entity.WorkModeration, error) {
	var out []entity.WorkModeration
	for _, v := range m.byKey {
		out = append(out, v)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func newModerationUC() (*WorksUseCase, *memModerationRepo) {
	uc := &WorksUseCase{}
	repo := &memModerationRepo{}
	uc.SetModerationRepo(repo)
	return uc, repo
}

func TestHideWorkValidation(t *testing.T) {
	uc, _ := newModerationUC()
	ctx := context.Background()

	if err := uc.HideWork(ctx, "", "video", "t1", "hidden", "原因", "op"); err == nil {
		t.Error("空 work_key 应报错")
	}
	if err := uc.HideWork(ctx, "g-1", "video", "t1", "hidden", "", "op"); err == nil || !strings.Contains(err.Error(), "原因") {
		t.Errorf("空 reason 应报错（审计必填），得到 %v", err)
	}
	if err := uc.HideWork(ctx, "g-1", "video", "t1", "nuke", "原因", "op"); err == nil {
		t.Error("非法 action 应报错")
	}
	if err := uc.HideWork(ctx, "g-1", "video", "t1", "hidden", "违规内容", "op"); err != nil {
		t.Fatalf("合法下架应成功: %v", err)
	}
	// 幂等覆盖：同 key 重复处置覆盖动作
	if err := uc.HideWork(ctx, "g-1", "video", "t1", "deleted", "升级处置", "op2"); err != nil {
		t.Fatalf("重复处置应幂等覆盖: %v", err)
	}
	m, _ := uc.modRepo.FindByKey(ctx, "g-1")
	if m.Action != entity.WorkActionDeleted || m.Operator != "op2" {
		t.Errorf("重复处置应覆盖 action/operator: %+v", m)
	}
}

func TestModeratedByKeyAnnotation(t *testing.T) {
	uc, repo := newModerationUC()
	ctx := context.Background()
	_ = uc.HideWork(ctx, "g-task-1", "video", "t1", "hidden", "违规", "op")
	_ = uc.HideWork(ctx, "c-cont-2", "article", "t1", "deleted", "违规", "op")
	_ = uc.HideWork(ctx, "g-task-3", "video", "t2", "hidden", "他租户", "op") // 其他租户

	byKey := uc.moderatedByKey(ctx, "t1")
	if byKey["g-task-1"].Action != entity.WorkActionHidden || byKey["c-cont-2"].Action != entity.WorkActionDeleted {
		t.Error("本租户 hidden/deleted 作品应进标注索引")
	}
	if byKey["g-task-3"].Action != "" {
		t.Error("他租户处置不应影响本租户标注")
	}

	// 恢复后不再标注
	if err := uc.RestoreWork(ctx, "g-task-1"); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	if uc.moderatedByKey(ctx, "t1")["g-task-1"].Action != "" {
		t.Error("恢复后不应再标注")
	}
	if _, err := repo.FindByKey(ctx, "g-task-1"); err == nil {
		t.Error("恢复应清除处置记录")
	}
}

// TestAppealFlow 申诉全流程（32号 P2 终批）：提交→防滥用→维持→限频→再申诉。
func TestAppealFlow(t *testing.T) {
	uc, _ := newModerationUC()
	ctx := context.Background()
	_ = uc.HideWork(ctx, "g-appeal-1", "video", "t1", "hidden", "违规", "op")

	// 未处置作品申诉拒绝
	if err := uc.AppealWork(ctx, "t1", "g-none", "我是清白的"); err == nil {
		t.Error("无处置记录的申诉应拒绝")
	}
	// 空理由拒绝
	if err := uc.AppealWork(ctx, "t1", "g-appeal-1", "  "); err == nil {
		t.Error("空申诉理由应拒绝")
	}
	// 合法申诉
	if err := uc.AppealWork(ctx, "t1", "g-appeal-1", "内容是正规美食探店，请复核"); err != nil {
		t.Fatalf("合法申诉应成功: %v", err)
	}
	// 申诉中重复提交拒绝
	if err := uc.AppealWork(ctx, "t1", "g-appeal-1", "再申诉"); err == nil || !strings.Contains(err.Error(), "审核中") {
		t.Errorf("申诉中应拒绝重复提交: %v", err)
	}
	// 跨租户申诉（记录带租户时）
	if err := uc.AppealWork(ctx, "t2", "g-appeal-1", "别人的作品"); err == nil {
		t.Error("他租户申诉应拒绝（记录带租户归属时）")
	}
	// 管理员维持
	if err := uc.RejectAppeal(ctx, "g-appeal-1"); err != nil {
		t.Fatalf("维持处置应成功: %v", err)
	}
	// 无 pending 时再维持拒绝
	if err := uc.RejectAppeal(ctx, "g-appeal-1"); err == nil {
		t.Error("无待审申诉时维持应拒绝")
	}
	// 维持后 24h 内再申诉拒绝
	if err := uc.AppealWork(ctx, "t1", "g-appeal-1", "马上再试"); err == nil || !strings.Contains(err.Error(), "24") {
		t.Errorf("维持后 24h 内应拒绝再申诉: %v", err)
	}
}

func TestModerationEnabled(t *testing.T) {
	uc := &WorksUseCase{}
	if uc.ModerationEnabled() {
		t.Error("未注入仓储应返回 false（能力关闭）")
	}
	uc2, _ := newModerationUC()
	if !uc2.ModerationEnabled() {
		t.Error("注入后应返回 true")
	}
}
