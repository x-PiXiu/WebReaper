package videotranscript

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"webreaper/internal/usecase/port"
)

// fakeAI 固定返回的 LLM 桩。
type fakeAI struct{ out string }

func (f *fakeAI) ChatStream(ctx context.Context, conv, cfg string, msgs []port.ChatMessage, delta func(string)) (string, error) {
	return f.out, nil
}
func (f *fakeAI) RunWithTools(ctx context.Context, conv, cfg, task, sys string, tools []string, onEvent func(port.ToolEvent)) error {
	return nil
}

// fakeASR 固定文本桩。
type fakeASR struct{ text string }

func (f fakeASR) Transcribe(ctx context.Context, path, mime string, size int64) (string, error) {
	return f.text, nil
}

func TestParseScriptJSON(t *testing.T) {
	// ```json 包裹 + 思考标签容忍
	out := "思考中...\n```json\n{\"clean\":\"清洗版\",\"rewrite\":\"改写版\"}\n```"
	res := parseScriptJSON(out)
	if res == nil || res.Clean != "清洗版" || res.Rewrite != "改写版" {
		t.Errorf("应容忍包裹解析双产出，得到 %+v", res)
	}
}

func TestRewriteScriptDualOutput(t *testing.T) {
	uc := NewUseCase(nil, nil, fakeASR{}, &fakeAI{out: `{"clean":"干净原文","rewrite":"品牌改写"}`})
	res, err := uc.RewriteScript(context.Background(), "嗯大家好这个鱼是现杀的", "酸菜鱼餐馆")
	if err != nil {
		t.Fatalf("改写失败: %v", err)
	}
	if res.Clean != "干净原文" || res.Rewrite != "品牌改写" {
		t.Errorf("双产出不符: %+v", res)
	}
}

func TestRewriteScriptFallback(t *testing.T) {
	// LLM 未按 JSON 输出 → 整段当清洗版，不失败
	uc := NewUseCase(nil, nil, fakeASR{}, &fakeAI{out: "这不是JSON"})
	res, err := uc.RewriteScript(context.Background(), "原文", "主题")
	if err != nil {
		t.Fatalf("降级失败: %v", err)
	}
	if res.Clean != "这不是JSON" {
		t.Errorf("非 JSON 输出应降级为清洗版，得到 %q", res.Clean)
	}
}

func TestExtractSSRFBlocked(t *testing.T) {
	// 内网地址必须被拒（R1）——httptest 监听 127.0.0.1，safeDownload 应拒绝
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("should-never-download"))
	}))
	defer srv.Close()
	uc := NewUseCase(nil, nil, fakeASR{}, nil)
	if _, err := uc.Extract(context.Background(), ExtractInput{VideoURL: srv.URL}); err == nil {
		t.Error("内网/环回 URL 应被 SSRF 防护拒绝")
	} else if !strings.Contains(err.Error(), "内网") {
		t.Errorf("错误应为内网拒绝语义，得到 %v", err)
	}
}

func TestExtractFromFileFFmpegAbsentDirectASR(t *testing.T) {
	// ffmpeg 不可用 → ≤25MB 文件直传 ASR（降级路径）
	dir := t.TempDir()
	p := filepath.Join(dir, "v.mp4")
	os.WriteFile(p, []byte("fake-mp4-data"), 0o644)
	uc := NewUseCase(nil, nil, fakeASR{text: "识别文本"}, nil)
	res, err := uc.ExtractFromFile(context.Background(), "t1", p, "标题")
	if err != nil {
		t.Fatalf("降级提取失败: %v", err)
	}
	if res.RawText != "识别文本" || res.Method != "asr-direct" {
		t.Errorf("降级路径产物不符: %+v", res)
	}
}
