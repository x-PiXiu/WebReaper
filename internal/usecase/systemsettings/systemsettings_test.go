package systemsettings

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- fake 仓储 ----

type fakeSettingRepo struct {
	port.SystemSettingRepository
	m map[string]string
}

func (f *fakeSettingRepo) Get(_ context.Context, key string) (entity.SystemSetting, error) {
	if v, ok := f.m[key]; ok {
		return entity.SystemSetting{Key: key, Value: v}, nil
	}
	return entity.SystemSetting{}, pkg.ErrNotFound
}

func (f *fakeSettingRepo) Save(_ context.Context, s entity.SystemSetting) error {
	if f.m == nil {
		f.m = map[string]string{}
	}
	f.m[s.Key] = s.Value
	return nil
}

type fakeTenantRepo struct {
	port.TenantSettingRepository
	m map[string]string // key = tenantID|settingKey
}

func (f *fakeTenantRepo) Get(_ context.Context, tenantID, key string) (entity.TenantSetting, error) {
	if v, ok := f.m[tenantID+"|"+key]; ok {
		return entity.TenantSetting{TenantID: tenantID, Key: key, Value: v}, nil
	}
	return entity.TenantSetting{}, pkg.ErrNotFound
}

func (f *fakeTenantRepo) Save(_ context.Context, s entity.TenantSetting) error {
	if f.m == nil {
		f.m = map[string]string{}
	}
	f.m[s.TenantID+"|"+s.Key] = s.Value
	return nil
}

type fakeSyncer struct{ called, headed bool }

func (f *fakeSyncer) SyncBrowserHeaded(headed bool) { f.called = true; f.headed = headed }

// ---- 平台级设置测试 ----

func TestAutoMonitorToggle(t *testing.T) {
	repo := &fakeSettingRepo{}
	uc := NewSystemSettingsUseCase(repo)

	// 未配置 → false
	if v, _ := uc.GetAutoMonitor(context.Background()); v {
		t.Error("未配置应返回 false")
	}
	// 开启
	if err := uc.SetAutoMonitor(context.Background(), true); err != nil {
		t.Fatalf("SetAutoMonitor(true): %v", err)
	}
	if v, _ := uc.GetAutoMonitor(context.Background()); !v {
		t.Error("开启后应返回 true")
	}
	// 关闭
	_ = uc.SetAutoMonitor(context.Background(), false)
	if v, _ := uc.GetAutoMonitor(context.Background()); v {
		t.Error("关闭后应返回 false")
	}
}

func TestBrowserHeadedSync(t *testing.T) {
	repo := &fakeSettingRepo{}
	uc := NewSystemSettingsUseCase(repo)
	syncer := &fakeSyncer{}
	uc.SetHeadedSyncer(syncer)

	// 未配置 → false（headless 生产默认）
	if v, _ := uc.GetBrowserHeaded(context.Background()); v {
		t.Error("未配置应返回 false")
	}
	// 开启 → 同步器应被调用
	_ = uc.SetBrowserHeaded(context.Background(), true)
	if !syncer.called || !syncer.headed {
		t.Error("同步器应被调用且值为 true")
	}
	if v, _ := uc.GetBrowserHeaded(context.Background()); !v {
		t.Error("开启后应返回 true")
	}
}

// ---- 租户级设置测试 ----

func TestTenantAutoMonitorDefault(t *testing.T) {
	// 未注入 tenantRepo → 默认开启
	uc := NewSystemSettingsUseCase(&fakeSettingRepo{})
	if v, _ := uc.GetTenantAutoMonitor(context.Background(), "t1"); !v {
		t.Error("未注入 tenantRepo 应默认开启")
	}

	// 注入但未配置 → 默认开启
	uc2 := NewSystemSettingsUseCase(&fakeSettingRepo{})
	uc2.SetTenantSettingRepo(&fakeTenantRepo{})
	if v, _ := uc2.GetTenantAutoMonitor(context.Background(), "t1"); !v {
		t.Error("未配置应默认开启")
	}

	// 显式关闭
	uc2.SetTenantAutoMonitor(context.Background(), "t1", false)
	if v, _ := uc2.GetTenantAutoMonitor(context.Background(), "t1"); v {
		t.Error("显式关闭后应返回 false")
	}
}

func TestTenantAutoMonitorConfigDefault(t *testing.T) {
	uc := NewSystemSettingsUseCase(&fakeSettingRepo{})
	uc.SetTenantSettingRepo(&fakeTenantRepo{})

	// 未配置 → 默认值
	cfg, _ := uc.GetTenantAutoMonitorConfig(context.Background(), "t1")
	def := entity.DefaultAutoMonitorConfig()
	if cfg != def {
		t.Errorf("未配置应返回默认: got %+v, want %+v", cfg, def)
	}

	// 写入再读回
	newCfg := entity.AutoMonitorConfig{Frequency: "weekly", SampleSize: 5}
	if err := uc.SetTenantAutoMonitorConfig(context.Background(), "t1", newCfg); err != nil {
		t.Fatalf("SetTenantAutoMonitorConfig: %v", err)
	}
	got, _ := uc.GetTenantAutoMonitorConfig(context.Background(), "t1")
	if got.Frequency != "weekly" || got.SampleSize != 5 {
		t.Errorf("写读不一致: got %+v, want frequency=weekly sample=5", got)
	}
}

// ---- 损坏配置容错 ----

func TestCorruptedConfigFallback(t *testing.T) {
	repo := &fakeTenantRepo{m: map[string]string{
		"t1|auto_monitor_config": `{invalid json`,
	}}
	uc := NewSystemSettingsUseCase(&fakeSettingRepo{})
	uc.SetTenantSettingRepo(repo)

	cfg, _ := uc.GetTenantAutoMonitorConfig(context.Background(), "t1")
	if cfg != entity.DefaultAutoMonitorConfig() {
		t.Error("损坏配置应回退默认值")
	}
}
