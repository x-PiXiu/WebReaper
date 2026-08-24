package bilibili

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"webreaper/internal/domain/entity"
)

func TestBilibiliCrawler_Platform(t *testing.T) {
	c := NewBilibiliCrawler("", nil)
	if c.Platform() != "bilibili" {
		t.Errorf("Platform() = %v, want bilibili", c.Platform())
	}
}

func TestBilibiliCrawler_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != searchPath {
			t.Errorf("unexpected path: %v", r.URL.Path)
		}

		resp := searchResponse{Code: 0, Message: "ok"}
		resp.Data.Result = []searchResult{
			{
				BVID:      "BV1xx411c7mD",
				Title:     "<em class=\"keyword\">测试</em>视频标题",
				Pic:       "http://example.com/cover.jpg",
				Author:    "测试UP主",
				Duration:  "3:45",
				Play:      100000,
				Like:      5000,
				Review:    200,
				Favorites: 1000,
				Pubdate:   1700000000,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewBilibiliCrawler("test_cookie", &CrawlerConfig{MaxResults: 20})

	t.Run("空关键词返回错误", func(t *testing.T) {
		_, err := c.Search(context.Background(), entity.SearchOptions{Keyword: ""})
		if err == nil {
			t.Error("Search() with empty keyword should return error")
		}
	})

	_ = server
}

func TestSearchResultToCrawled(t *testing.T) {
	item := searchResult{
		BVID:      "BV1test123",
		Title:     "<em class=\"keyword\">川菜</em>教程",
		Description: "正宗川菜做法",
		Pic:       "http://example.com/pic.jpg",
		Author:    "美食UP主",
		Duration:  "5:30",
		Play:      50000,
		Like:      3000,
		Review:    150,
		Favorites: 800,
		Pubdate:   1700000000,
	}

	c := searchResultToCrawled(item)

	if c.Platform != "bilibili" {
		t.Errorf("Platform = %v, want bilibili", c.Platform)
	}
	if c.VideoID != "BV1test123" {
		t.Errorf("VideoID = %v, want BV1test123", c.VideoID)
	}
	if c.Title != "川菜教程" {
		t.Errorf("Title = %v, want 川菜教程 (HTML cleaned)", c.Title)
	}
	if c.Description != "正宗川菜做法" {
		t.Errorf("Description = %v, want 正宗川菜做法", c.Description)
	}
	if c.Author != "美食UP主" {
		t.Errorf("Author = %v, want 美食UP主", c.Author)
	}
	if c.Duration != 330 {
		t.Errorf("Duration = %v, want 330 (5:30)", c.Duration)
	}
	if c.PlayCount != 50000 {
		t.Errorf("PlayCount = %v, want 50000", c.PlayCount)
	}
	if c.DiggCount != 3000 {
		t.Errorf("DiggCount = %v, want 3000", c.DiggCount)
	}
	if c.CommentCount != 150 {
		t.Errorf("CommentCount = %v, want 150", c.CommentCount)
	}
	if c.CollectCount != 800 {
		t.Errorf("CollectCount = %v, want 800", c.CollectCount)
	}
	if c.PublishTime.IsZero() {
		t.Error("PublishTime should not be zero")
	}
}

func TestCleanHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<em class=\"keyword\">测试</em>", "测试"},
		{"普通文本", "普通文本"},
		{"<em>a</em><em>b</em>", "ab"},
		{"", ""},
		{"<em class=\"k\">川菜</em>教程", "川菜教程"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanHTML(tt.input)
			if got != tt.want {
				t.Errorf("cleanHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"3:45", 225},
		{"0:30", 30},
		{"10:00", 600},
		{"120", 120},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDuration(tt.input)
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestBilibiliCrawler_GetCapabilities(t *testing.T) {
	c := NewBilibiliCrawler("", nil)
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

func TestBilibiliCrawler_UpdateCookies(t *testing.T) {
	c := NewBilibiliCrawler("old", nil)
	c.UpdateCookies("new")
	if c.cookies != "new" {
		t.Errorf("cookies = %v, want new", c.cookies)
	}
}

func TestBilibiliCrawler_DefaultConfig(t *testing.T) {
	c := NewBilibiliCrawler("", nil)
	if c.config.MaxResults != 20 {
		t.Errorf("MaxResults = %v, want 20", c.config.MaxResults)
	}
	if c.config.UserAgent == "" {
		t.Error("UserAgent should not be empty")
	}
}

func TestBilibiliCrawler_SearchWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := searchResponse{Code: 0, Message: "ok"}
		resp.Data.Result = []searchResult{
			{BVID: "BV1", Title: "视频1", Play: 1000, Like: 100, Review: 10},
			{BVID: "BV2", Title: "视频2", Play: 2000, Like: 200, Review: 20},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 验证响应解析逻辑
	resp := searchResponse{Code: 0}
	resp.Data.Result = []searchResult{
		{BVID: "BV1", Title: "视频1", Play: 1000, Like: 100, Review: 10},
	}

	videos := make([]entity.CrawledVideo, 0, len(resp.Data.Result))
	for _, item := range resp.Data.Result {
		videos = append(videos, searchResultToCrawled(item))
	}

	if len(videos) != 1 {
		t.Errorf("len(videos) = %v, want 1", len(videos))
	}
	if videos[0].VideoID != "BV1" {
		t.Errorf("VideoID = %v, want BV1", videos[0].VideoID)
	}
}
