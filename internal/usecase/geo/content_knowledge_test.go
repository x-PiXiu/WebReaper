package geo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// mockKBRetriever 可控知识库检索器。
type mockKBRetriever struct {
	refs []entity.MaterialRef
	err  error
}

func (m *mockKBRetriever) Retrieve(context.Context, string, string, int) ([]entity.MaterialRef, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.refs, nil
}

var _ port.KnowledgeRetriever = (*mockKBRetriever)(nil)

// mockOnlineRetriever 可控在线检索器（记录调用次数与期望数量）。
type mockOnlineRetriever struct {
	ref     string
	err     error
	calls   int
	lastNum int
}

func (m *mockOnlineRetriever) RetrieveContent(_ context.Context, _ string, num int) (string, error) {
	m.calls++
	m.lastNum = num
	if m.err != nil {
		return "", m.err
	}
	return m.ref, nil
}

var _ port.ContentRAGRetriever = (*mockOnlineRetriever)(nil)

// TestBuildMaterialPrompt 参考资料格式：带来源标注 + 编号 + 标题兜底。
func TestBuildMaterialPrompt(t *testing.T) {
	refs := []entity.MaterialRef{
		{Title: "AI口播视频制作", Summary: "正文摘要一", SourceURL: "https://a.com/1", Score: 0.9},
		{Title: "", Summary: "正文摘要二", SourceURL: "https://b.com/2", Score: 0.8}, // 空标题 → 兜底 URL
	}
	got := buildMaterialPrompt(refs)

	if !strings.Contains(got, "来自平台知识库") {
		t.Errorf("缺少知识库标注: %s", got)
	}
	if !strings.Contains(got, "[1] AI口播视频制作｜来源：https://a.com/1") {
		t.Errorf("第 1 条格式错误（标题+来源）: %s", got)
	}
	if !strings.Contains(got, "[2] https://b.com/2｜来源：https://b.com/2") {
		t.Errorf("空标题应兜底为来源 URL: %s", got)
	}
	if !strings.Contains(got, "正文摘要一") {
		t.Errorf("应包含摘要: %s", got)
	}
	// 空列表 → 空段落
	if got := buildMaterialPrompt(nil); got != "" {
		t.Errorf("空列表应返回空: %q", got)
	}
}

// TestBrandIndustry 行业解析：显式字段 / 仓储缺失 / BrandID 空 / 查询失败。
func TestBrandIndustry(t *testing.T) {
	// BrandID 空 / 仓储未注入 → ""
	uc := &ContentUseCase{}
	if got := uc.brandIndustry(context.Background(), GenerateInput{}); got != "" {
		t.Errorf("空 BrandID 应返回空: %q", got)
	}
	if got := uc.brandIndustry(context.Background(), GenerateInput{BrandID: "b1"}); got != "" {
		t.Errorf("仓储未注入应返回空: %q", got)
	}

	// 显式行业
	repo := newFakeBrandRepo()
	_ = repo.Save(context.Background(), entity.Brand{ID: "b1", TenantID: "t1", Industry: "餐饮"})
	uc3 := &ContentUseCase{brandRepo: repo}
	if got := uc3.brandIndustry(context.Background(), GenerateInput{TenantID: "t1", BrandID: "b1"}); got != "餐饮" {
		t.Errorf("应返回品牌行业: %q", got)
	}

	// 查询失败（品牌不存在）→ ""
	if got := uc3.brandIndustry(context.Background(), GenerateInput{TenantID: "t1", BrandID: "nope"}); got != "" {
		t.Errorf("查询失败应返回空: %q", got)
	}
}

// TestBuildReferencePrompt 检索策略："本地优先 + 在线兜底/补齐"（不牺牲召回）。
func TestBuildReferencePrompt(t *testing.T) {
	kbRefs := []entity.MaterialRef{{Title: "本地素材", Summary: "摘要", SourceURL: "https://kb.com/1", Score: 0.9}}
	in := GenerateInput{BrandID: "b1"}

	// ① 知识库命中 1 条（<3）→ 知识库段落 + 在线补齐差额（num=2）
	online := &mockOnlineRetriever{ref: "在线补充"}
	uc := &ContentUseCase{
		knowledgeRetriever: &mockKBRetriever{refs: kbRefs},
		ragRetriever:       online,
	}
	got := uc.buildReferencePrompt(context.Background(), in, "关键词")
	if !strings.Contains(got, "来自平台知识库") || !strings.Contains(got, "https://kb.com/1") {
		t.Errorf("知识库命中应注入知识库段落: %s", got)
	}
	if !strings.Contains(got, "在线补充") {
		t.Errorf("命中不足应在线补齐: %s", got)
	}
	if online.calls != 1 || online.lastNum != 2 {
		t.Errorf("补齐量应为 3-1=2: calls=%d num=%d", online.calls, online.lastNum)
	}

	// ② 知识库无命中 → 在线全量兜底（num=3）
	online = &mockOnlineRetriever{ref: "在线内容"}
	uc = &ContentUseCase{
		knowledgeRetriever: &mockKBRetriever{refs: nil},
		ragRetriever:       online,
	}
	got = uc.buildReferencePrompt(context.Background(), in, "关键词")
	if !strings.Contains(got, "在线内容") {
		t.Errorf("知识库无命中应在线兜底: %s", got)
	}
	if online.lastNum != 3 {
		t.Errorf("无命中时在线应全量 3 条: %d", online.lastNum)
	}

	// ③ 知识库报错 → 在线兜底
	uc = &ContentUseCase{
		knowledgeRetriever: &mockKBRetriever{err: errors.New("embedding down")},
		ragRetriever:       &mockOnlineRetriever{ref: "在线内容"},
	}
	got = uc.buildReferencePrompt(context.Background(), in, "关键词")
	if !strings.Contains(got, "在线内容") {
		t.Errorf("知识库报错应在线兜底: %s", got)
	}

	// ④ 都不可用 → 空串（纯 LLM）
	uc = &ContentUseCase{}
	if got := uc.buildReferencePrompt(context.Background(), in, "关键词"); got != "" {
		t.Errorf("都不可用应返回空: %q", got)
	}

	// ⑤ 知识库命中满 3 条 → 不再调在线
	online = &mockOnlineRetriever{ref: "在线内容"}
	fullRefs := []entity.MaterialRef{
		{Title: "a", Summary: "s", SourceURL: "https://kb.com/a", Score: 0.9},
		{Title: "b", Summary: "s", SourceURL: "https://kb.com/b", Score: 0.8},
		{Title: "c", Summary: "s", SourceURL: "https://kb.com/c", Score: 0.7},
	}
	uc = &ContentUseCase{
		knowledgeRetriever: &mockKBRetriever{refs: fullRefs},
		ragRetriever:       online,
	}
	got = uc.buildReferencePrompt(context.Background(), in, "关键词")
	if strings.Contains(got, "在线内容") || online.calls != 0 {
		t.Errorf("知识库命中满量时在线不应被调用: calls=%d", online.calls)
	}
}
