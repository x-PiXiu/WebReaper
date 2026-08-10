package indexing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	mockrepo "webreaper/internal/adapter/mock"
	"webreaper/internal/usecase/port"
)

func newTestUC(submitter port.URLSubmitter) (*IndexingUseCase, *mockrepo.MockSystemSettingRepository, *mockrepo.MockIndexingLogRepository, *mockrepo.MockOptimizedContentRepository) {
	setting := mockrepo.NewMockSystemSettingRepository()
	logs := mockrepo.NewMockIndexingLogRepository()
	content := mockrepo.NewMockOptimizedContentRepository()
	return NewIndexingUseCase(setting, logs, content, submitter, "https://content.example.com"), setting, logs, content
}

// ---- 配置校验 ----

func TestUpdateConfig_Validation(t *testing.T) {
	uc, _, _, _ := newTestUC(nil)
	ctx := context.Background()

	if err := uc.UpdateConfig(ctx, entity.IndexingConfig{IndexNowKey: "bad-key!"}); err == nil {
		t.Error("非法 IndexNow key 应报错")
	}
	if err := uc.UpdateConfig(ctx, entity.IndexingConfig{BaiduSite: "x.com"}); err == nil {
		t.Error("百度 site 无 token 应报错")
	}
	if err := uc.UpdateConfig(ctx, entity.IndexingConfig{BaiduToken: "tok"}); err == nil {
		t.Error("百度 token 无 site 应报错")
	}
	// 合法配置
	if err := uc.UpdateConfig(ctx, entity.IndexingConfig{IndexNowKey: "my-valid-key-1"}); err != nil {
		t.Errorf("合法配置应通过: %v", err)
	}
	if err := uc.UpdateConfig(ctx, entity.IndexingConfig{BaiduSite: "x.com", BaiduToken: "tok"}); err != nil {
		t.Errorf("百度配对配置应通过: %v", err)
	}
	// 空配置合法（未启用）
	if err := uc.UpdateConfig(ctx, entity.IndexingConfig{}); err != nil {
		t.Errorf("空配置应通过: %v", err)
	}
}

func TestGetConfig_Empty(t *testing.T) {
	uc, _, _, _ := newTestUC(nil)
	cfg, err := uc.GetConfig(context.Background())
	if err != nil || cfg.IsConfigured() {
		t.Errorf("无配置应返回空且不报错: %+v err=%v", cfg, err)
	}
}

// ---- 提交日志 ----

func TestLogSubmitAndList(t *testing.T) {
	uc, _, _, _ := newTestUC(nil)
	ctx := context.Background()

	if err := uc.LogSubmit(ctx, entity.IndexingSubmitLog{Channel: "indexnow", URL: "https://x/a", Status: "success"}); err != nil {
		t.Fatalf("LogSubmit: %v", err)
	}
	if err := uc.LogSubmit(ctx, entity.IndexingSubmitLog{Channel: "baidu", URL: "https://x/b", Status: "failed", ErrorMsg: "timeout"}); err != nil {
		t.Fatalf("LogSubmit: %v", err)
	}
	logs, err := uc.ListLogs(ctx, 10)
	if err != nil || len(logs) != 2 {
		t.Fatalf("应列出 2 条: %v err=%v", logs, err)
	}
	// 倒序：最新的在前
	if logs[0].URL != "https://x/b" {
		t.Errorf("应倒序排列: %+v", logs[0])
	}
}

// ---- 手动补提交 ----

type testSubmitter struct {
	err error
	got []string
}

func (s *testSubmitter) SubmitURLs(_ context.Context, urls []string) error {
	s.got = append(s.got, urls...)
	return s.err
}

func TestReSubmitAll(t *testing.T) {
	sub := &testSubmitter{}
	uc, _, _, content := newTestUC(sub)
	ctx := context.Background()

	// 预置 2 条已发布内容 + 1 条草稿
	for i, id := range []string{"oc-1", "oc-2", "oc-draft"} {
		status := "published"
		if id == "oc-draft" {
			status = "draft"
		}
		_ = content.Save(ctx, entity.OptimizedContent{
			ID: id, TenantID: "t", BrandID: "b", Title: "标题" + string(rune('0'+i)),
			OptimizedText: "正文", Status: status, CreatedAt: time.Now(),
		})
	}

	submitted, failed, err := uc.ReSubmitAll(ctx)
	if err != nil || submitted != 2 || failed != 0 {
		t.Fatalf("应提交 2 条（草稿不提交）: submitted=%d failed=%d err=%v", submitted, failed, err)
	}
	if len(sub.got) != 2 {
		t.Fatalf("submitter 应收到 2 个 URL: %v", sub.got)
	}
	for _, u := range sub.got {
		if !strings.Contains(u, "https://content.example.com/public/articles/oc-") {
			t.Errorf("URL 格式错误: %s", u)
		}
	}
}

func TestReSubmitAll_SubmitterError(t *testing.T) {
	sub := &testSubmitter{err: errors.New("渠道失败")}
	uc, _, _, content := newTestUC(sub)
	ctx := context.Background()
	_ = content.Save(ctx, entity.OptimizedContent{ID: "oc-1", TenantID: "t", BrandID: "b", Status: "published", CreatedAt: time.Now()})

	if _, _, err := uc.ReSubmitAll(ctx); err == nil {
		t.Error("渠道失败应返回错误")
	}
	// 失败也应记录日志
	logs, _ := uc.ListLogs(ctx, 10)
	if len(logs) != 1 || logs[0].Status != "failed" {
		t.Errorf("失败应记录审计日志: %+v", logs)
	}
}

func TestReSubmitAll_NoSubmitter(t *testing.T) {
	uc, _, _, _ := newTestUC(nil) // submitter 为 nil
	if _, _, err := uc.ReSubmitAll(context.Background()); err == nil {
		t.Error("未配置提交器应报错")
	}
}
