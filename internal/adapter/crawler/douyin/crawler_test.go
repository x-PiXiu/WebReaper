package crawler

import (
	"context"
	"fmt"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// mockSearcher 是 douyinwebSearcher 的 mock 实现。
type mockSearcher struct {
	videos    []port.SocialVideo
	detail    *port.SocialVideo
	alive     bool
	searchErr error
	detailErr error
}

func (m *mockSearcher) SearchHotVideos(ctx context.Context, tenantID, plat, keyword string, limit int) ([]port.SocialVideo, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	if limit > len(m.videos) {
		limit = len(m.videos)
	}
	return m.videos[:limit], nil
}

func (m *mockSearcher) GetVideoDetail(ctx context.Context, tenantID, plat, videoID string) (*port.SocialVideo, error) {
	if m.detailErr != nil {
		return nil, m.detailErr
	}
	return m.detail, nil
}

func (m *mockSearcher) IsAlive(ctx context.Context, tenantID, plat string) bool {
	return m.alive
}

func (m *mockSearcher) CheckCookieAlive(ctx context.Context, cookie string) (bool, string) {
	return m.alive, ""
}

func TestDouyinCrawler_Platform(t *testing.T) {
	c := NewDouyinCrawler(&mockSearcher{}, nil)
	if c.Platform() != "douyin" {
		t.Errorf("Platform() = %v, want douyin", c.Platform())
	}
}

func TestDouyinCrawler_Search(t *testing.T) {
	mock := &mockSearcher{
		videos: []port.SocialVideo{
			{Platform: "douyin", VideoID: "1", Desc: "视频1", Author: "作者1", URL: "http://example.com/1", PlayCount: 1000, DiggCount: 100, CommentCount: 50, ShareCount: 10},
			{Platform: "douyin", VideoID: "2", Desc: "视频2", Author: "作者2", URL: "http://example.com/2", PlayCount: 2000, DiggCount: 200, CommentCount: 80, ShareCount: 20},
		},
		alive: true,
	}
	c := NewDouyinCrawler(mock, nil)

	t.Run("正常搜索", func(t *testing.T) {
		videos, err := c.Search(context.Background(), entity.SearchOptions{Keyword: "川菜", Limit: 10})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(videos) != 2 {
			t.Errorf("Search() returned %d videos, want 2", len(videos))
		}
		if videos[0].VideoID != "1" {
			t.Errorf("videos[0].VideoID = %v, want 1", videos[0].VideoID)
		}
		if videos[0].PlayCount != 1000 {
			t.Errorf("videos[0].PlayCount = %v, want 1000", videos[0].PlayCount)
		}
	})

	t.Run("空关键词返回错误", func(t *testing.T) {
		_, err := c.Search(context.Background(), entity.SearchOptions{Keyword: ""})
		if err == nil {
			t.Error("Search() with empty keyword should return error")
		}
	})

	t.Run("搜索失败", func(t *testing.T) {
		mock.searchErr = fmt.Errorf("网络错误")
		defer func() { mock.searchErr = nil }()
		_, err := c.Search(context.Background(), entity.SearchOptions{Keyword: "测试"})
		if err == nil {
			t.Error("Search() should return error when searcher fails")
		}
	})
}

func TestDouyinCrawler_GetDetail(t *testing.T) {
	mock := &mockSearcher{
		detail: &port.SocialVideo{
			Platform: "douyin", VideoID: "123", Desc: "详情测试", Author: "作者",
			URL: "http://example.com/123", PlayCount: 5000, DiggCount: 500,
		},
		alive: true,
	}
	c := NewDouyinCrawler(mock, nil)

	t.Run("正常获取详情", func(t *testing.T) {
		v, err := c.GetDetail(context.Background(), "123")
		if err != nil {
			t.Fatalf("GetDetail() error = %v", err)
		}
		if v.VideoID != "123" {
			t.Errorf("VideoID = %v, want 123", v.VideoID)
		}
		if v.PlayCount != 5000 {
			t.Errorf("PlayCount = %v, want 5000", v.PlayCount)
		}
	})

	t.Run("空ID返回错误", func(t *testing.T) {
		_, err := c.GetDetail(context.Background(), "")
		if err == nil {
			t.Error("GetDetail() with empty ID should return error")
		}
	})
}

func TestDouyinCrawler_RefreshMetrics(t *testing.T) {
	mock := &mockSearcher{
		detail: &port.SocialVideo{
			Platform: "douyin", VideoID: "123", PlayCount: 9999, DiggCount: 888,
			CommentCount: 77, ShareCount: 66,
		},
	}
	c := NewDouyinCrawler(mock, nil)

	updates, err := c.RefreshMetrics(context.Background(), []string{"123", "456"})
	if err != nil {
		t.Fatalf("RefreshMetrics() error = %v", err)
	}
	// 456 会失败（mock 返回 detail 只有 123），但 123 应该成功
	if len(updates) < 1 {
		t.Errorf("RefreshMetrics() returned %d updates, want >= 1", len(updates))
	}
	if updates[0].PlayCount != 9999 {
		t.Errorf("PlayCount = %v, want 9999", updates[0].PlayCount)
	}
}

func TestDouyinCrawler_IsAlive(t *testing.T) {
	mock := &mockSearcher{alive: true}
	c := NewDouyinCrawler(mock, nil)
	if !c.IsAlive(context.Background()) {
		t.Error("IsAlive() = false, want true")
	}
}

func TestDouyinCrawler_GetCapabilities(t *testing.T) {
	c := NewDouyinCrawler(&mockSearcher{}, nil)
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

func TestSocialVideoToCrawled(t *testing.T) {
	v := port.SocialVideo{
		Platform: "douyin", VideoID: "789", Desc: "转换测试",
		Author: "测试作者", URL: "http://example.com/789",
		PlayCount: 10000, DiggCount: 800, CommentCount: 50, ShareCount: 20,
	}
	c := socialVideoToCrawled(v)
	if c.Platform != "douyin" {
		t.Errorf("Platform = %v, want douyin", c.Platform)
	}
	if c.VideoID != "789" {
		t.Errorf("VideoID = %v, want 789", c.VideoID)
	}
	if c.Title != "转换测试" {
		t.Errorf("Title = %v, want 转换测试", c.Title)
	}
	if c.PlayCount != 10000 {
		t.Errorf("PlayCount = %v, want 10000", c.PlayCount)
	}
	if c.DiggCount != 800 {
		t.Errorf("DiggCount = %v, want 800", c.DiggCount)
	}
}
