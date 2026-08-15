package providerconfig

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- fake 仓储 ----

type fakeRepo struct {
	port.ProviderConfigRepository
	cfgs map[string]entity.ProviderConfig
}

func (f *fakeRepo) List(_ context.Context) ([]entity.ProviderConfig, error) {
	out := make([]entity.ProviderConfig, 0, len(f.cfgs))
	for _, c := range f.cfgs {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeRepo) Get(_ context.Context, provider string) (entity.ProviderConfig, error) {
	if c, ok := f.cfgs[provider]; ok {
		return c, nil
	}
	return entity.ProviderConfig{}, pkg.ErrNotFound
}

func (f *fakeRepo) Upsert(_ context.Context, cfg entity.ProviderConfig) error {
	if f.cfgs == nil {
		f.cfgs = map[string]entity.ProviderConfig{}
	}
	f.cfgs[cfg.Provider] = cfg
	return nil
}

// ---- 用例测试 ----

func TestUpsertValidation(t *testing.T) {
	uc := NewUseCase(&fakeRepo{})
	if err := uc.Upsert(context.Background(), entity.ProviderConfig{}); err == nil {
		t.Error("空厂商名应报错")
	}
	if err := uc.Upsert(context.Background(), entity.ProviderConfig{Provider: "vidu"}); err != nil {
		t.Errorf("有效厂商名不应报错: %v", err)
	}
}

func TestListAndUpsert(t *testing.T) {
	repo := &fakeRepo{cfgs: map[string]entity.ProviderConfig{}}
	uc := NewUseCase(repo)
	_ = uc.Upsert(context.Background(), entity.ProviderConfig{Provider: "vidu", APIKey: "sk-12345678"})
	_ = uc.Upsert(context.Background(), entity.ProviderConfig{Provider: "baidu", APIKey: "bk-87654321"})

	list, err := uc.List(context.Background())
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %d, %v; want 2, nil", len(list), err)
	}
}

// ---- MaskKey 纯函数测试 ----

func TestMaskKey(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"short":                 "****",
		"12345678":              "****", // ≤8 全遮
		"sk-1234567890abcdef":   "sk-1****cdef",
		"abcdefghijklmnopqrstuvwxyz0123456789": "abcd****6789",
	}
	for in, want := range cases {
		if got := MaskKey(in); got != want {
			t.Errorf("MaskKey(%q) = %q, want %q", in, got, want)
		}
	}
}
