package works

// moderation_test.go —— 32号：作品处置用例单测（内存仓储）。

import (
	"context"
	"strings"
	"testing"

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

func TestModeratedKeysFilter(t *testing.T) {
	uc, repo := newModerationUC()
	ctx := context.Background()
	_ = uc.HideWork(ctx, "g-task-1", "video", "t1", "hidden", "违规", "op")
	_ = uc.HideWork(ctx, "c-cont-2", "article", "t1", "deleted", "违规", "op")
	_ = uc.HideWork(ctx, "g-task-3", "video", "t2", "hidden", "他租户", "op") // 其他租户

	keys := uc.moderatedKeys(ctx, "t1")
	if !keys["g-task-1"] || !keys["c-cont-2"] {
		t.Error("本租户 hidden/deleted 作品应进过滤集合")
	}
	if keys["g-task-3"] {
		t.Error("他租户处置不应影响本租户过滤")
	}

	// 恢复后不再过滤
	if err := uc.RestoreWork(ctx, "g-task-1"); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	keys = uc.moderatedKeys(ctx, "t1")
	if keys["g-task-1"] {
		t.Error("恢复后不应再过滤")
	}
	if _, err := repo.FindByKey(ctx, "g-task-1"); err == nil {
		t.Error("恢复应清除处置记录")
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
