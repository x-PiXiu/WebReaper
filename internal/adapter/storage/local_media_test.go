package storage

import (
	"context"
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
