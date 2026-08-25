package account

import (
	"context"
	"testing"

	"webreaper/internal/usecase/port"
)

func TestDefaultContentAdapter_Adapt(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		platform  string
		req       port.AdaptRequest
		wantTitle string
		wantErr   bool
	}{
		{
			name:     "抖音-标题截断",
			platform: "douyin",
			req: port.AdaptRequest{
				Platform:    "douyin",
				Title:       "这是一个超过三十个字的标题需要被截断处理看看效果如何测试一下标题截断功能是否正常工作",
				Description: "测试描述",
				Tags:        []string{"#测试"},
			},
			wantTitle: "这是一个超过三十个字的标题需要被截断处理看看效果如何测试一…",
		},
		{
			name:     "抖音-标题未超限",
			platform: "douyin",
			req: port.AdaptRequest{
				Platform:    "douyin",
				Title:       "短标题",
				Description: "测试描述",
				Tags:        []string{"#测试"},
			},
			wantTitle: "短标题",
		},
		{
			name:     "快手-无Emoji",
			platform: "kuaishou",
			req: port.AdaptRequest{
				Platform:    "kuaishou",
				Title:       "测试标题",
				Description: "测试描述👍❤️",
				Tags:        []string{"#测试"},
			},
			wantTitle: "测试标题",
		},
		{
			name:     "小红书-标题20字限制",
			platform: "xiaohongshu",
			req: port.AdaptRequest{
				Platform:    "xiaohongshu",
				Title:       "这是一个超过二十个字的小红书标题需要被截断",
				Description: "测试描述",
			},
			wantTitle: "这是一个超过二十个字的小红书标题需要被…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultPlatformConfigs[tt.platform]
			adapter := NewDefaultContentAdapter(config)

			result, err := adapter.Adapt(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Adapt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result.Title != tt.wantTitle {
				t.Errorf("Adapt() title = %v, want %v", result.Title, tt.wantTitle)
			}
		})
	}
}

func TestDefaultContentAdapter_AdaptTags(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		platform string
		tags     []string
		desc     string
		wantLen  int
	}{
		{
			name:     "抖音-标签截断",
			platform: "douyin",
			tags:     []string{"#标签1", "#标签2", "#标签3", "#标签4"},
			desc:     "测试描述",
			wantLen:  3,
		},
		{
			name:     "快手-标签2个",
			platform: "kuaishou",
			tags:     []string{"#标签1", "#标签2", "#标签3"},
			desc:     "测试描述",
			wantLen:  2,
		},
		{
			name:     "抖音-标签不足补充默认",
			platform: "douyin",
			tags:     []string{"#自定义"},
			desc:     "测试描述",
			wantLen:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultPlatformConfigs[tt.platform]
			adapter := NewDefaultContentAdapter(config)

			req := port.AdaptRequest{
				Platform:    tt.platform,
				Title:       "测试标题",
				Description: tt.desc,
				Tags:        tt.tags,
			}

			result, err := adapter.Adapt(ctx, req)
			if err != nil {
				t.Errorf("Adapt() error = %v", err)
				return
			}
			if len(result.Tags) != tt.wantLen {
				t.Errorf("Adapt() tags len = %v, want %v, tags = %v", len(result.Tags), tt.wantLen, result.Tags)
			}
		})
	}
}

func TestDefaultContentAdapter_AdaptCTA(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		platform string
		wantCTA  bool
	}{
		{
			name:     "抖音-需要CTA",
			platform: "douyin",
			wantCTA:  true,
		},
		{
			name:     "小红书-不需要CTA",
			platform: "xiaohongshu",
			wantCTA:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultPlatformConfigs[tt.platform]
			adapter := NewDefaultContentAdapter(config)

			req := port.AdaptRequest{
				Platform:    tt.platform,
				Title:       "测试标题",
				Description: "测试描述",
			}

			result, err := adapter.Adapt(ctx, req)
			if err != nil {
				t.Errorf("Adapt() error = %v", err)
				return
			}
			if tt.wantCTA && result.CTA == "" {
				t.Errorf("Adapt() CTA should not be empty for platform %s", tt.platform)
			}
			if !tt.wantCTA && result.CTA != "" {
				t.Errorf("Adapt() CTA should be empty for platform %s", tt.platform)
			}
		})
	}
}

func TestContentAdapterRegistry(t *testing.T) {
	registry := NewContentAdapterRegistryImpl()

	// 注册适配器
	for _, config := range DefaultPlatformConfigs {
		adapter := NewDefaultContentAdapter(config)
		registry.Register(adapter)
	}

	// 测试获取
	adapter, err := registry.Get("douyin")
	if err != nil {
		t.Errorf("Get() error = %v", err)
		return
	}
	if adapter.Platform() != "douyin" {
		t.Errorf("Get() platform = %v, want douyin", adapter.Platform())
	}

	// 测试不存在的平台
	_, err = registry.Get("unknown")
	if err == nil {
		t.Error("Get() should return error for unknown platform")
	}

	// 测试列表
	adapters := registry.List()
	if len(adapters) != len(DefaultPlatformConfigs) {
		t.Errorf("List() len = %v, want %v", len(adapters), len(DefaultPlatformConfigs))
	}
}
