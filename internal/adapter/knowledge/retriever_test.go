package knowledge

import (
	"context"
	"errors"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// fakeRepo 内存实现（SearchSimilar 返回预置结果——余弦/排序已在 repository 包测试覆盖）。
type fakeRepo struct {
	refs []entity.MaterialRef
	err  error
}

func (f *fakeRepo) Save(context.Context, *entity.KnowledgeMaterial) error { return nil }
func (f *fakeRepo) ExistsByFingerprint(context.Context, string) (bool, error) { return false, nil }
func (f *fakeRepo) SearchSimilar(_ context.Context, _, _ string, _ []float32, _ int) ([]entity.MaterialRef, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.refs, nil
}
func (f *fakeRepo) Count(context.Context, string) (int64, error) { return 0, nil }
func (f *fakeRepo) CountByBrand(context.Context, string) (int64, error) { return 0, nil }
func (f *fakeRepo) ListByIndustry(context.Context, string, int, int) ([]entity.KnowledgeMaterial, error) {
	return nil, nil
}
func (f *fakeRepo) ListByBrand(context.Context, string, string, int, int) ([]entity.KnowledgeMaterial, error) {
	return nil, nil
}
func (f *fakeRepo) Delete(context.Context, string) error { return nil }
func (f *fakeRepo) DeleteByBrand(context.Context, string, string, string) error { return nil }

var _ port.KnowledgeMaterialRepository = (*fakeRepo)(nil)

// fakeEmbedder 固定返回查询向量（维度 3）。
type fakeEmbedder struct{ err error }

func (f *fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []float32{1, 0, 0}, nil
}
func (f *fakeEmbedder) EmbedBatch(context.Context, []string) ([][]float32, error) { return nil, nil }
func (f *fakeEmbedder) Dimension() int                                            { return 3 }

var _ port.Embedder = (*fakeEmbedder)(nil)

func ref(title string, score float32) entity.MaterialRef {
	return entity.MaterialRef{Title: title, Summary: "摘要", SourceURL: "https://x.com/" + title, Score: score}
}

// TestRetrieve_ThresholdFilter 阈值过滤（<0.25 剔除）。
func TestRetrieve_ThresholdFilter(t *testing.T) {
	r := NewKnowledgeRetriever(&fakeRepo{refs: []entity.MaterialRef{
		ref("high", 0.9), ref("mid", 0.3), ref("low", 0.1), ref("neg", 0.0),
	}}, &fakeEmbedder{})
	got, err := r.Retrieve(context.Background(), "餐饮", "", "关键词", 3)
	if err != nil {
		t.Fatalf("检索失败: %v", err)
	}
	if len(got) != 2 || got[0].Title != "high" || got[1].Title != "mid" {
		t.Errorf("阈值过滤错误（应只留 high/mid）: %+v", got)
	}
	if got[0].SourceURL == "" {
		t.Error("来源 URL 必须保留")
	}
}

// TestRetrieve_NumCap num 裁剪与默认值。
func TestRetrieve_NumCap(t *testing.T) {
	r := NewKnowledgeRetriever(&fakeRepo{refs: []entity.MaterialRef{
		ref("a", 0.9), ref("b", 0.8), ref("c", 0.7), ref("d", 0.6),
	}}, &fakeEmbedder{})
	got, _ := r.Retrieve(context.Background(), "", "", "q", 2)
	if len(got) != 2 {
		t.Errorf("num=2 应裁剪为 2 条: %d", len(got))
	}
	got, _ = r.Retrieve(context.Background(), "", "", "q", 0)
	if len(got) != 3 {
		t.Errorf("num<=0 默认 3: %d", len(got))
	}
}

// TestRetrieve_EmptyAndErrors 无命中 / embedder 失败 / 空查询。
func TestRetrieve_EmptyAndErrors(t *testing.T) {
	// 无命中
	r := NewKnowledgeRetriever(&fakeRepo{refs: nil}, &fakeEmbedder{})
	got, err := r.Retrieve(context.Background(), "餐饮", "", "q", 3)
	if err != nil || len(got) != 0 {
		t.Errorf("无命中应返回空且无错: %v %v", got, err)
	}
	// embedder 失败 → 返回错误（调用方降级）
	r = NewKnowledgeRetriever(&fakeRepo{}, &fakeEmbedder{err: errors.New("api down")})
	if _, err := r.Retrieve(context.Background(), "餐饮", "", "q", 3); err == nil {
		t.Error("embedder 失败应返回错误")
	}
	// 空查询 → 直接空
	r = NewKnowledgeRetriever(&fakeRepo{refs: []entity.MaterialRef{ref("a", 0.9)}}, &fakeEmbedder{})
	if got, _ := r.Retrieve(context.Background(), "餐饮", "", "", 3); len(got) != 0 {
		t.Error("空查询应返回空")
	}
}
