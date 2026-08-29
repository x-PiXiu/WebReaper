// compose_store_adapter.go videocompose 产物上传适配器（mediaStore → CreationUploader）。
package main

import (
	"context"
	"fmt"
	"os"

	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/videocompose"
)

// composeStoreAdapter 把现有 mediaStore 适配为 videocompose.CreationUploader。
type composeStoreAdapter struct {
	store port.MediaAssetStore
}

var _ videocompose.CreationUploader = composeStoreAdapter{}

// Upload 本地文件读入 → SaveFile（ownerType=creation 产物）→ 返回可访问 URL。
func (a composeStoreAdapter) Upload(ctx context.Context, tenantID, localPath, kind string) (string, error) {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return "", fmt.Errorf("读取合成产物失败: %w", err)
	}
	mime := "video/mp4"
	if kind == "image" {
		mime = "image/jpeg"
	}
	asset, err := a.store.SaveFile(ctx, tenantID, "", "creation", data, mime, "mp4")
	if err != nil {
		return "", err
	}
	u := asset.StoredURL
	if u == "" {
		u = asset.SourceURL
	}
	return u, nil
}
