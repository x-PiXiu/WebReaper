package kuaishou

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"webreaper/internal/domain/entity"
)

func TestKuaishouCrawler_Platform(t *testing.T) {
	c := NewKuaishouCrawler("", nil)
	if c.Platform() != "kuaishou" {
		t.Errorf("Platform() = %v, want kuaishou", c.Platform())
	}
}

func TestKuaishouCrawler_Search(t *testing.T) {
	// Mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != searchPath {
			t.Errorf("unexpected path: %v", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %v", r.Method)
		}

		resp := searchResponse{}
		resp.Data.VisionSearchPhoto.List = []searchItem{
			{
				ID:          "video1",
				Caption:     "快手测试视频1",
				PhotoURL:    "http://example.com/video1.mp4",
				CoverURL:    "http://example.com/cover1.jpg",
				ViewCount:   10000,
				LikeCount:   500,
				CommentCount: 50,
				ShareCount:  20,
				Timestamp:   1700000000000,
				Author:      struct{ Name string `json:"name"`; HeaderUrl string `json:"headerUrl"` }{Name: "测试作者"},
			},
			{
				ID:          "video2",
				Caption:     "快手测试视频2",
				ViewCount:   20000,
				LikeCount:   1000,
				CommentCount: 100,
				ShareCount:  40,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 用 mock 服务器的 URL 替换 host
	c := NewKuaishouCrawler("test_cookie", &CrawlerConfig{MaxResults: 20})
	// 注意：这里我们无法直接替换 host 常量，所以这个测试主要验证结构体转换逻辑

	t.Run("空关键词返回错误", func(t *testing.T) {
		_, err := c.Search(context.Background(), entity.SearchOptions{Keyword: ""})
		if err == nil {
			t.Error("Search() with empty keyword should return error")
		}
	})

	_ = server // 防止未使用警告
}

func TestSearchItemToCrawled(t *testing.T) {
	item := searchItem{
		ID:          "test123",
		Caption:     "测试快手视频",
		PhotoURL:    "http://example.com/video.mp4",
		CoverURL:    "http://example.com/cover.jpg",
		ViewCount:   50000,
		LikeCount:   3000,
		CommentCount: 200,
		ShareCount:  100,
		Timestamp:   1700000000000,
		Author: struct {
			Name      string `json:"name"`
			HeaderUrl string `json:"headerUrl"`
		}{Name: "快手作者", HeaderUrl: "http://example.com/avatar.jpg"},
	}

	c := searchItemToCrawled(item)

	if c.Platform != "kuaishou" {
		t.Errorf("Platform = %v, want kuaishou", c.Platform)
	}
	if c.VideoID != "test123" {
		t.Errorf("VideoID = %v, want test123", c.VideoID)
	}
	if c.Title != "测试快手视频" {
		t.Errorf("Title = %v, want 测试快手视频", c.Title)
	}
	if c.Author != "快手作者" {
		t.Errorf("Author = %v, want 快手作者", c.Author)
	}
	if c.PlayCount != 50000 {
		t.Errorf("PlayCount = %v, want 50000", c.PlayCount)
	}
	if c.DiggCount != 3000 {
		t.Errorf("DiggCount = %v, want 3000", c.DiggCount)
	}
	if c.CommentCount != 200 {
		t.Errorf("CommentCount = %v, want 200", c.CommentCount)
	}
	if c.ShareCount != 100 {
		t.Errorf("ShareCount = %v, want 100", c.ShareCount)
	}
	if c.CoverURL != "http://example.com/cover.jpg" {
		t.Errorf("CoverURL = %v, want http://example.com/cover.jpg", c.CoverURL)
	}
	if c.VideoURL != "http://example.com/video.mp4" {
		t.Errorf("VideoURL = %v, want http://example.com/video.mp4", c.VideoURL)
	}
	if c.AuthorAvatar != "http://example.com/avatar.jpg" {
		t.Errorf("AuthorAvatar = %v, want http://example.com/avatar.jpg", c.AuthorAvatar)
	}
	if c.PublishTime.IsZero() {
		t.Error("PublishTime should not be zero")
	}
}

func TestSearchItemToCrawled_ZeroTimestamp(t *testing.T) {
	item := searchItem{
		ID:        "no_time",
		Caption:   "无时间戳",
		Timestamp: 0,
	}
	c := searchItemToCrawled(item)
	if !c.PublishTime.IsZero() {
		t.Errorf("PublishTime should be zero for timestamp=0, got %v", c.PublishTime)
	}
}

func TestKuaishouCrawler_GetCapabilities(t *testing.T) {
	c := NewKuaishouCrawler("", nil)
	caps := c.GetCapabilities()

	if !caps.SupportSearch {
		t.Error("SupportSearch should be true")
	}
	if !caps.SupportDetail {
		t.Error("SupportDetail should be true")
	}
	if !caps.SupportRefresh {
		t.Error("SupportRefresh should be true")
	}
	if caps.MaxSearchLimit != 20 {
		t.Errorf("MaxSearchLimit = %v, want 20", caps.MaxSearchLimit)
	}
}

func TestKuaishouCrawler_UpdateCookies(t *testing.T) {
	c := NewKuaishouCrawler("old_cookie", nil)
	c.UpdateCookies("new_cookie")
	if c.cookies != "new_cookie" {
		t.Errorf("cookies = %v, want new_cookie", c.cookies)
	}
}

func TestKuaishouCrawler_IsAlive_NoServer(t *testing.T) {
	c := NewKuaishouCrawler("", nil)
	// 没有服务器时应该返回 false
	alive := c.IsAlive(context.Background())
	// 实际会尝试连接 kuaishou.com，可能成功也可能失败
	// 这里只验证不 panic
	_ = alive
}

func TestKuaishouCrawler_DefaultConfig(t *testing.T) {
	c := NewKuaishouCrawler("", nil)
	if c.config.MaxResults != 20 {
		t.Errorf("MaxResults = %v, want 20", c.config.MaxResults)
	}
	if c.config.UserAgent == "" {
		t.Error("UserAgent should not be empty")
	}
}

func TestKuaishouCrawler_CustomConfig(t *testing.T) {
	cfg := &CrawlerConfig{
		MaxResults: 50,
		UserAgent:  "CustomAgent/1.0",
	}
	c := NewKuaishouCrawler("cookie", cfg)
	if c.config.MaxResults != 50 {
		t.Errorf("MaxResults = %v, want 50", c.config.MaxResults)
	}
	if c.config.UserAgent != "CustomAgent/1.0" {
		t.Errorf("UserAgent = %v, want CustomAgent/1.0", c.config.UserAgent)
	}
}
