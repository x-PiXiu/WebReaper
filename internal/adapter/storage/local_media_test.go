package storage

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadAndStore(t *testing.T) {
	dir := t.TempDir()
	// 本地 HTTP 服务模拟 Vidu 产物（24h URL）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Write([]byte("fake-mp4-content"))
	}))
	defer srv.Close()

	store, err := NewLocalMediaStore(dir, "http://localhost:8082")
	if err != nil {
		t.Fatalf("NewLocalMediaStore: %v", err)
	}
	stored, err := store.DownloadAndStore(context.Background(), "t1", srv.URL+"/creation.mp4", nil)
	if err != nil {
		t.Fatalf("DownloadAndStore: %v", err)
	}
	if !strings.Contains(stored, "/media/t1/") || !strings.Contains(stored, "/c-") || !strings.HasSuffix(stored, ".mp4") {
		t.Errorf("stored URL 格式不对: %s", stored)
	}
	// 文件真实落盘（子目录结构：{dir}/{tenantID}/{date}/{shortID}.ext）
	// 用 filepath.Walk 递归查找文件
	var found string
	filepath.Walk(filepath.Join(dir, "t1"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".mp4") {
			found = p
		}
		return nil
	})
	if found == "" {
		t.Fatalf("未找到落盘的 mp4 文件")
	}
	data, _ := os.ReadFile(found)
	if string(data) != "fake-mp4-content" {
		t.Errorf("文件内容不对: %q", string(data))
	}
}

func TestDownloadAndStoreFailure(t *testing.T) {
	store, err := NewLocalMediaStore(t.TempDir(), "http://localhost:8082")
	if err != nil {
		t.Fatal(err)
	}
	// 不可达域名（mock 产物场景）——应报错不 panic
	if _, err := store.DownloadAndStore(context.Background(), "t1", "https://mock.vidu.local/x.mp4", nil); err == nil {
		t.Error("不可达 URL 应报错")
	}
}

func TestSaveFileAndPublicURL(t *testing.T) {
	store, err := NewLocalMediaStore(t.TempDir(), "http://localhost:8082")
	if err != nil {
		t.Fatal(err)
	}
	asset, err := store.SaveFile(context.Background(), "t1", "b1", "material", []byte("png-data"), "image/png", ".png")
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if !strings.Contains(asset.SourceURL, "/media/t1/") || !strings.HasSuffix(asset.SourceURL, ".png") {
		t.Errorf("URL 格式不对: %s", asset.SourceURL)
	}
}

func TestReadLocal(t *testing.T) {
	s, err := NewLocalMediaStore(t.TempDir(), "http://localhost:8082")
	if err != nil {
		t.Fatal(err)
	}
	asset, err := s.SaveFile(context.Background(), "t1", "", "material", []byte("pngdata"), "image/png", ".png")
	if err != nil {
		t.Fatal(err)
	}
	// 本站托管 URL → 读到内容与 MIME
	data, mime, ok := s.ReadLocal(context.Background(), asset.SourceURL)
	if !ok || string(data) != "pngdata" || mime != "image/png" {
		t.Errorf("本站 URL 应读到本地文件，得到 ok=%v data=%q mime=%q", ok, data, mime)
	}
	// 外部 URL → 不处理
	if _, _, ok := s.ReadLocal(context.Background(), "https://cdn.example.com/x.png"); ok {
		t.Error("外部 URL 不应读取")
	}
	// 路径穿越 → 拒绝
	if _, _, ok := s.ReadLocal(context.Background(), "http://localhost:8082/media/../secret"); ok {
		t.Error("路径穿越应被拒绝")
	}
}

// TestDownloadAndStoreDataURI 缺口B修复：data: URI（MiMo 内联 base64）直接落盘。
func TestDownloadAndStoreDataURI(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalMediaStore(dir, "http://localhost:8082")
	if err != nil {
		t.Fatalf("NewLocalMediaStore: %v", err)
	}
	uri := "data:audio/mp3;base64," + base64.StdEncoding.EncodeToString([]byte("fake-mimo-audio"))
	stored, err := store.DownloadAndStore(context.Background(), "t1", uri, nil)
	if err != nil {
		t.Fatalf("DownloadAndStore data URI: %v", err)
	}
	if !strings.HasPrefix(stored, "http://localhost:8082/media/t1/") {
		t.Errorf("stored URL 前缀异常: %s", stored)
	}
	if !strings.HasSuffix(stored, ".mp3") {
		t.Errorf("扩展名应按 mime 推断为 .mp3: %s", stored)
	}
	// 文件落盘校验
	rel := strings.TrimPrefix(stored, "http://localhost:8082/media/")
	b, rErr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if rErr != nil || string(b) != "fake-mimo-audio" {
		t.Errorf("落盘内容不符: err=%v content=%q", rErr, string(b))
	}
}

// TestParseDataURI 非 data: 形态返回 ok=false（走原 HTTP 路径）。
func TestParseDataURI(t *testing.T) {
	if _, _, ok := ParseDataURI("https://example.com/a.mp3"); ok {
		t.Error("http URL 不应命中 data URI 分支")
	}
	if _, _, ok := ParseDataURI("data:"); ok {
		t.Error("缺 payload 的 data: 应解析失败")
	}
	d, ext, ok := ParseDataURI("data:image/png;base64,iVBORw0KGgo=")
	if !ok || ext != ".png" || len(d) != 8 {
		t.Errorf("png data URI 解析异常: ok=%v ext=%s len=%d", ok, ext, len(d))
	}
}
