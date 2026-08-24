package entity

import (
	"testing"
	"time"
)

func TestCalculateViralScore(t *testing.T) {
	tests := []struct {
		name     string
		play     int64
		digg     int64
		comment  int64
		share    int64
		collect  int64
		wantMin  float64
		wantMax  float64
	}{
		{
			name:    "零播放量返回0",
			play:    0,
			digg:    100,
			comment: 50,
			share:   10,
			collect: 20,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "高互动率视频",
			play:    10000,
			digg:    500,   // 5% 点赞率
			comment: 100,   // 1% 评论率
			share:   50,    // 0.5% 分享率
			collect: 200,   // 2% 收藏率
			wantMin: 50,
			wantMax: 100,
		},
		{
			name:    "低互动率视频",
			play:    1000000,
			digg:    100,   // 0.01% 点赞率
			comment: 10,
			share:   5,
			collect: 10,
			wantMin: 0,
			wantMax: 5,
		},
		{
			name:    "超高互动率（封顶100）",
			play:    1000,
			digg:    500,
			comment: 200,
			share:   100,
			collect: 300,
			wantMin: 99,
			wantMax: 100,
		},
		{
			name:    "典型热门视频",
			play:    100000,
			digg:    8000,
			comment: 500,
			share:   300,
			collect: 1000,
			wantMin: 70,
			wantMax: 90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateViralScore(tt.play, tt.digg, tt.comment, tt.share, tt.collect)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculateViralScore(%d, %d, %d, %d, %d) = %v, want [%v, %v]",
					tt.play, tt.digg, tt.comment, tt.share, tt.collect, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateViralScore_BoundaryCases(t *testing.T) {
	// 测试边界情况
	t.Run("所有计数为0", func(t *testing.T) {
		got := CalculateViralScore(0, 0, 0, 0, 0)
		if got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})

	t.Run("只有播放量无互动", func(t *testing.T) {
		got := CalculateViralScore(1000000, 0, 0, 0, 0)
		if got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})

	t.Run("分数不会为负", func(t *testing.T) {
		got := CalculateViralScore(1000, 0, 0, 0, 0)
		if got < 0 {
			t.Errorf("got %v, want >= 0", got)
		}
	})

	t.Run("分数不会超过100", func(t *testing.T) {
		got := CalculateViralScore(1, 1000, 1000, 1000, 1000)
		if got > 100 {
			t.Errorf("got %v, want <= 100", got)
		}
	})
}

func TestCrawledVideoToInspiration(t *testing.T) {
	now := time.Now()
	v := CrawledVideo{
		Platform:     "douyin",
		VideoID:      "12345",
		Title:        "测试视频标题",
		Description:  "测试描述",
		CoverURL:     "https://example.com/cover.jpg",
		VideoURL:     "https://example.com/video.mp4",
		Author:       "测试作者",
		AuthorAvatar: "https://example.com/avatar.jpg",
		Duration:     30,
		PublishTime:  now,
		PlayCount:    100000,
		DiggCount:    8000,
		CommentCount: 500,
		ShareCount:   300,
		CollectCount: 1000,
		Topics:       []string{"测试", "热门"},
		MusicName:    "测试音乐",
		MusicAuthor:  "音乐作者",
	}

	insp := CrawledVideoToInspiration(v)

	// 验证字段映射
	if insp.Platform != "douyin" {
		t.Errorf("Platform = %v, want douyin", insp.Platform)
	}
	if insp.PlatformVideoID != "12345" {
		t.Errorf("PlatformVideoID = %v, want 12345", insp.PlatformVideoID)
	}
	if insp.Title != "测试视频标题" {
		t.Errorf("Title = %v, want 测试视频标题", insp.Title)
	}
	if insp.Author != "测试作者" {
		t.Errorf("Author = %v, want 测试作者", insp.Author)
	}
	if insp.Duration != 30 {
		t.Errorf("Duration = %v, want 30", insp.Duration)
	}
	if insp.PlayCount != 100000 {
		t.Errorf("PlayCount = %v, want 100000", insp.PlayCount)
	}
	if insp.DiggCount != 8000 {
		t.Errorf("DiggCount = %v, want 8000", insp.DiggCount)
	}
	if insp.Sentiment != "neutral" {
		t.Errorf("Sentiment = %v, want neutral", insp.Sentiment)
	}
	if insp.ViralScore <= 0 || insp.ViralScore > 100 {
		t.Errorf("ViralScore = %v, want (0, 100]", insp.ViralScore)
	}
	if len(insp.Topics) != 2 {
		t.Errorf("Topics length = %v, want 2", len(insp.Topics))
	}
}

func TestCrawlerAccount_Constants(t *testing.T) {
	// 验证常量值正确
	if CrawlerAccountActive != "active" {
		t.Errorf("CrawlerAccountActive = %v, want active", CrawlerAccountActive)
	}
	if CrawlerAccountExpired != "expired" {
		t.Errorf("CrawlerAccountExpired = %v, want expired", CrawlerAccountExpired)
	}
	if CrawlerAccountBanned != "banned" {
		t.Errorf("CrawlerAccountBanned = %v, want banned", CrawlerAccountBanned)
	}
	if HealthHealthy != "healthy" {
		t.Errorf("HealthHealthy = %v, want healthy", HealthHealthy)
	}
	if HealthUnhealthy != "unhealthy" {
		t.Errorf("HealthUnhealthy = %v, want unhealthy", HealthUnhealthy)
	}
}

func TestTaskLog_Constants(t *testing.T) {
	if TaskLogRunning != "running" {
		t.Errorf("TaskLogRunning = %v, want running", TaskLogRunning)
	}
	if TaskLogSuccess != "success" {
		t.Errorf("TaskLogSuccess = %v, want success", TaskLogSuccess)
	}
	if TaskLogFailed != "failed" {
		t.Errorf("TaskLogFailed = %v, want failed", TaskLogFailed)
	}
	if TriggerScheduled != "scheduled" {
		t.Errorf("TriggerScheduled = %v, want scheduled", TriggerScheduled)
	}
	if TriggerManual != "manual" {
		t.Errorf("TriggerManual = %v, want manual", TriggerManual)
	}
}
