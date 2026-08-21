package publisher

import (
	"context"
	"fmt"

	"github.com/chromedp/chromedp"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 抖音/快手发布通道（获客智能体转型：视频分发主战场）----
//
// 架构（与知乎/小红书同模式）：半自动 + 全自动合并在同一个 Channel。
// 能力声明：video（主力）+ image（图文轮播）；不支持 article。
//
// 反检测（Level 2）：全自动发布走 humanize.HumanAction（人类行为模拟 + 指纹伪装）。

// DouyinAutoChannel 抖音发布通道（半自动 + 全自动）。
type DouyinAutoChannel struct{}

var _ port.PublishChannel = (*DouyinAutoChannel)(nil)
var _ port.ChannelInfoProvider = (*DouyinAutoChannel)(nil)
var _ port.AutoPublishChannel = (*DouyinAutoChannel)(nil)

func NewDouyinAutoChannel() *DouyinAutoChannel { return &DouyinAutoChannel{} }

func (c *DouyinAutoChannel) Platform() string             { return "douyin" }
func (c *DouyinAutoChannel) SupportedMediaType() []string { return []string{"video", "image"} }
func (c *DouyinAutoChannel) SupportedContentTypes() []string {
	return []string{entity.ContentTypeVideo, entity.ContentTypeImage}
}

func (c *DouyinAutoChannel) DisplayName() string { return "抖音" }
func (c *DouyinAutoChannel) Constraints() map[string]entity.ChannelConstraints {
	return map[string]entity.ChannelConstraints{
		entity.ContentTypeVideo: {TitleMaxRunes: 55, MinVideos: 1},
		entity.ContentTypeImage: {TitleMaxRunes: 55, MinImages: 1},
	}
}

// PublishSemiAuto 半自动：生成抖音发布页 URL（用户手动上传+发布）。
func (c *DouyinAutoChannel) PublishSemiAuto(_ context.Context, _ entity.PublishJob, _ entity.Account) (string, error) {
	return "https://creator.douyin.com/creator-micro/content/publish-video", nil
}

// PublishAuto 全自动：chromedp + HumanAction 反检测层，RPA 发布视频。
func (c *DouyinAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookie string) (string, error) {
	return publishVideoRPA(ctx, job, cookie, "douyin", "https://creator.douyin.com/creator-micro/content/publish-video")
}

// KuaishouAutoChannel 快手发布通道（半自动 + 全自动）。
type KuaishouAutoChannel struct{}

var _ port.PublishChannel = (*KuaishouAutoChannel)(nil)
var _ port.ChannelInfoProvider = (*KuaishouAutoChannel)(nil)
var _ port.AutoPublishChannel = (*KuaishouAutoChannel)(nil)

func NewKuaishouAutoChannel() *KuaishouAutoChannel { return &KuaishouAutoChannel{} }

func (c *KuaishouAutoChannel) Platform() string             { return "kuaishou" }
func (c *KuaishouAutoChannel) SupportedMediaType() []string { return []string{"video", "image"} }
func (c *KuaishouAutoChannel) SupportedContentTypes() []string {
	return []string{entity.ContentTypeVideo, entity.ContentTypeImage}
}

func (c *KuaishouAutoChannel) DisplayName() string { return "快手" }
func (c *KuaishouAutoChannel) Constraints() map[string]entity.ChannelConstraints {
	return map[string]entity.ChannelConstraints{
		entity.ContentTypeVideo: {TitleMaxRunes: 80, MinVideos: 1},
		entity.ContentTypeImage: {TitleMaxRunes: 80, MinImages: 1},
	}
}

// PublishSemiAuto 半自动：生成快手发布页 URL。
func (c *KuaishouAutoChannel) PublishSemiAuto(_ context.Context, _ entity.PublishJob, _ entity.Account) (string, error) {
	return "https://cp.kuaishou.com/article/publish/video", nil
}

// PublishAuto 全自动：chromedp + HumanAction 反检测层，RPA 发布视频。
func (c *KuaishouAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookie string) (string, error) {
	return publishVideoRPA(ctx, job, cookie, "kuaishou", "https://cp.kuaishou.com/article/publish/video")
}

// publishVideoRPA 通用的视频发布 RPA（混合模式：正常走代码，异常 Agent 接管）。
// 使用 HumanAction 反检测层（人类行为模拟 + 指纹伪装）。
// 完整实现待选择器调通后替换（当前返回占位错误，半自动模式已可用）。
func publishVideoRPA(ctx context.Context, job entity.PublishJob, cookie, platform, publishURL string) (string, error) {
	// TODO: 完整 RPA 实现（下一轮调试选择器后启用）
	// 步骤：
	//   1. humanize.New(ctx) + StealthOptions() 启动带反检测的浏览器
	//   2. InjectFingerprint(ctx) 注入指纹伪装
	//   3. 注入 Cookie（与现有 XHS 模式一致）
	//   4. ha.Navigate(publishURL) → ha.WaitVisible(上传区域)
	//   5. ha.Upload(上传选择器, 视频文件) → 验证
	//   6. ha.Type(标题选择器, 标题) → 验证
	//   7. ha.Click(发布按钮) → 验证成功
	//   8. 成功后调用 extractVideoURLAfterPublish(ctx) 提取视频链接返回
	//  异常处理：ha.VerifySuccess() 失败 → 截屏 → Agent 接管
	_ = ctx
	_ = cookie
	return "", fmt.Errorf("%s 全自动视频发布开发中（当前支持半自动模式——生成链接手动发布）", platform)
}

// extractVideoURLAfterPublish RPA 发布成功后提取视频链接（作品数据详情/数据回读依赖）。
// 抖音创作者中心发布成功后跳转内容管理页，页面 URL/DOM 含视频 item_id（15+ 位数字）；
// 对外可播放链接形如 https://www.douyin.com/video/{item_id}。
// 提取优先级：当前 URL 中的 item_id → DOM 内视频卡片跳转链接中的 item_id。
// （publishVideoRPA 实现完成后在第 8 步调用；函数已就绪待接。）
func extractVideoURLAfterPublish(ctx context.Context) (string, error) {
	var itemID string
	err := chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const m = location.href.match(/(\d{15,})/);
		if (m) return m[1];
		const a = document.querySelector('a[href*="/video/"]');
		if (a) { const m2 = (a.href || '').match(/video\/(\d{15,})/); if (m2) return m2[1]; }
		return '';
	})()`, &itemID))
	if err != nil || itemID == "" {
		return "", fmt.Errorf("未从发布成功页提取到视频 ID")
	}
	return "https://www.douyin.com/video/" + itemID, nil
}
