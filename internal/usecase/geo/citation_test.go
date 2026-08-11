package geo

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
)

// monitoringResultForCitation 构造带 Sources 的监测结果。
func monitoringResultForCitation(sources []string) entity.MonitoringResult {
	return entity.MonitoringResult{ID: "r", Sources: sources}
}

// P5-02 内容引用统计测试（归因细化到篇）。

func TestCitationUseCase_GetByBrand(t *testing.T) {
	ctx := context.Background()
	results := &fakeResultRepo{}
	uc := NewCitationUseCase(results)

	// sources 里混有：自营文章链接（应计数）、外部链接（不计数）、平台名（不计数）
	results.results = append(results.results,
		monitoringResultForCitation([]string{"https://content.example.com/public/articles/oc-1", "https://zhihu.com/p/123"}),
		monitoringResultForCitation([]string{"https://content.example.com/public/articles/oc-1", "https://content.example.com/public/articles/oc-2"}),
		monitoringResultForCitation([]string{"知乎", "小红书"}), // 无 URL——不计数
	)

	counts, err := uc.GetByBrand(ctx, "t1", "b1")
	if err != nil {
		t.Fatalf("GetByBrand: %v", err)
	}
	if counts["oc-1"] != 2 {
		t.Errorf("oc-1 引用数 = %d, want 2", counts["oc-1"])
	}
	if counts["oc-2"] != 1 {
		t.Errorf("oc-2 引用数 = %d, want 1", counts["oc-2"])
	}
	if len(counts) != 2 {
		t.Errorf("应只统计到 2 篇内容: %+v", counts)
	}
}

func TestCitationUseCase_Empty(t *testing.T) {
	ctx := context.Background()
	uc := NewCitationUseCase(&fakeResultRepo{})
	counts, err := uc.GetByBrand(ctx, "t1", "b1")
	if err != nil {
		t.Fatalf("GetByBrand: %v", err)
	}
	if len(counts) != 0 {
		t.Errorf("无数据应返回空 map: %+v", counts)
	}
}
