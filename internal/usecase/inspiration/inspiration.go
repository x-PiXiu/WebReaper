// Package inspiration 实现灵感广场用例（整洁架构·Usecase层）。
//
// 产品语义：商户打开灵感广场，看到各品牌的热门视频数据，无需登录。
// 数据来源：平台方账号统一爬取，存入 DB，用户只读。
package inspiration

import (
	"sync"
	"path/filepath"
	"net/url"
	"net/http"
	"io"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// UseCase 灵感广场用例。
type UseCase struct {
	videoRepo   port.InspirationVideoRepository
	brandRepo   port.BrandInspirationRepository
	configRepo  port.CrawlerConfigRepository
	accountRepo port.CrawlerAccountRepository
	platforms   map[string]port.CrawlerPlatform
	llm         port.AIGenerator // LLM 用于生成关键词
	media       port.MediaAssetStore // 可选；本地转存（站内播放不依赖原站防盗链）
}

// SetMediaStore 注入媒体存储（可选；注入后爬取入库的视频异步转存到本地）。
func (uc *UseCase) SetMediaStore(m port.MediaAssetStore) { uc.media = m }

// NewUseCase 创建灵感广场用例。
func NewUseCase(
	videoRepo port.InspirationVideoRepository,
	brandRepo port.BrandInspirationRepository,
	configRepo port.CrawlerConfigRepository,
	accountRepo port.CrawlerAccountRepository,
) *UseCase {
	return &UseCase{
		videoRepo:   videoRepo,
		brandRepo:   brandRepo,
		configRepo:  configRepo,
		accountRepo: accountRepo,
		platforms:   make(map[string]port.CrawlerPlatform),
	}
}

// SetLLM 注入 LLM 用于生成关键词（可选）。
func (uc *UseCase) SetLLM(llm port.AIGenerator) {
	uc.llm = llm
}

// RegisterPlatform 注册平台爬虫。
func (uc *UseCase) RegisterPlatform(platform string, crawler port.CrawlerPlatform) {
	uc.platforms[platform] = crawler
}

// List 查询灵感视频列表（用户端，无需登录）。
func (uc *UseCase) List(ctx context.Context, brandID, platform, keyword, sortBy string, page, pageSize int) ([]entity.InspirationVideo, int, error) {
	return uc.videoRepo.List(ctx, brandID, platform, keyword, sortBy, page, pageSize)
}

// GetByID 查询单个灵感视频详情。
func (uc *UseCase) GetByID(ctx context.Context, id string) (entity.InspirationVideo, error) {
	return uc.videoRepo.FindByID(ctx, id)
}

// CrawlBrand 采集指定品牌的热门视频。
//
// 流程：
//  1. 获取品牌的搜索关键词
//  2. 调用平台爬虫搜索
//  3. 保存到 DB + 建立品牌关联
func (uc *UseCase) CrawlBrand(ctx context.Context, platform, brandID string, keywords []string) (*CrawlResult, error) {
	crawler, ok := uc.platforms[platform]
	if !ok {
		return nil, fmt.Errorf("平台 %s 未注册", platform)
	}

	startAt := time.Now()

	// 逐关键词搜索
	allVideos := make([]entity.CrawledVideo, 0)
	for _, keyword := range keywords {
		videos, err := crawler.Search(ctx, entity.SearchOptions{
			Keyword: keyword,
			Limit:   20,
		})
		if err != nil {
			log.Printf("[inspiration] 搜索失败 platform=%s keyword=%s: %v", platform, keyword, err)
			continue
		}
		allVideos = append(allVideos, videos...)
	}

	// 转换为 InspirationVideo
	inspirations := make([]entity.InspirationVideo, 0, len(allVideos))
	for _, v := range allVideos {
		insp := entity.CrawledVideoToInspiration(v)
		insp.ID = fmt.Sprintf("insp-%s-%s", v.Platform, v.VideoID)
		inspirations = append(inspirations, insp)
	}

	// 批量保存（去重）
	newCount, err := uc.videoRepo.SaveBatch(ctx, inspirations)
	if err != nil {
		return nil, fmt.Errorf("保存视频失败: %w", err)
	}

	// 异步本地转存（新入库视频——下载到 /media 后前端站内播放，与点赞/评论数同屏；
	// 失败保留原始链接。后台执行不阻塞爬取返回）
	if uc.media != nil {
		go uc.archiveToLocal(context.Background(), inspirations)
	}

	// 建立品牌关联
	for _, insp := range inspirations {
		if err := uc.brandRepo.Link(ctx, brandID, insp.ID, ""); err != nil {
			log.Printf("[inspiration] 建立品牌关联失败 brand=%s video=%s: %v", brandID, insp.ID, err)
		}
	}

	now := time.Now()
	return &CrawlResult{
		Platform:     platform,
		BrandID:      brandID,
		Keywords:     keywords,
		VideosFound:  len(allVideos),
		VideosNew:    newCount,
		DurationMs:   int(now.Sub(startAt).Milliseconds()),
		FinishedAt:   now,
	}, nil
}

// RefreshMetrics 刷新指定品牌视频的互动指标。
func (uc *UseCase) RefreshMetrics(ctx context.Context, platform string, videoIDs []string) (int, error) {
	crawler, ok := uc.platforms[platform]
	if !ok {
		return 0, fmt.Errorf("平台 %s 未注册", platform)
	}

	updates, err := crawler.RefreshMetrics(ctx, videoIDs)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, u := range updates {
		if err := uc.videoRepo.UpdateMetrics(ctx, u.VideoID, u); err != nil {
			log.Printf("[inspiration] 更新指标失败 video=%s: %v", u.VideoID, err)
			continue
		}
		updated++
	}
	return updated, nil
}

// IsPlatformAlive 检测平台爬虫是否可用。
func (uc *UseCase) IsPlatformAlive(ctx context.Context, platform string) bool {
	crawler, ok := uc.platforms[platform]
	if !ok {
		return false
	}
	return crawler.IsAlive(ctx)
}

// CheckAccountAlive 用指定账号 cookie 检测平台登录态（按账号健康检测）。
// 与 IsPlatformAlive 的区别：不经过账号池选择——unhealthy 账号也能被
// 重新检测救回（手动健康检查用，避免死锁：被标不健康→池子不再选→检测永远失败）。
func (uc *UseCase) CheckAccountAlive(ctx context.Context, platform, cookie string) (bool, string) {
	crawler, ok := uc.platforms[platform]
	if !ok {
		return false, "平台 " + platform + " 未接入"
	}
	return crawler.CheckAccountAlive(ctx, cookie)
}

// ListPlatforms 列出所有已注册的平台。
func (uc *UseCase) ListPlatforms() []string {
	platforms := make([]string, 0, len(uc.platforms))
	for p := range uc.platforms {
		platforms = append(platforms, p)
	}
	return platforms
}

// GenerateKeywords 调用 LLM 根据品牌信息生成搜索关键词。
func (uc *UseCase) GenerateKeywords(ctx context.Context, brandName, industry, positioning string) ([]string, error) {
	if uc.llm == nil {
		return nil, fmt.Errorf("LLM 未配置")
	}

	prompt := fmt.Sprintf(`根据以下品牌信息，生成 5-8 个短视频平台搜索关键词，用于搜索该品牌的热门视频参考。
品牌名称：%s
行业：%s
定位：%s
要求：
1. 每个关键词 2-8 个字
2. 关键词要多样化，覆盖不同角度（行业词、场景词、情感词）
3. 只输出关键词，用逗号分隔，不要其他内容`, brandName, industry, positioning)

	messages := []port.ChatMessage{
		{Role: "user", Content: prompt},
	}

	var resp string
	result, err := uc.llm.ChatStream(ctx, "", "", messages, func(delta string) {
		resp += delta
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 生成关键词失败: %w", err)
	}
	if result != "" {
		resp = result
	}

	// 解析关键词
	keywords := parseKeywords(resp)
	if len(keywords) == 0 {
		return nil, fmt.Errorf("LLM 未生成有效关键词")
	}

	return keywords, nil
}

// parseKeywords 解析 LLM 返回的关键词（逗号/空格分隔）。
func parseKeywords(resp string) []string {
	// 去除引号和多余空白
	resp = strings.TrimSpace(resp)
	resp = strings.Trim(resp, "\"'`")

	var keywords []string
	// 按逗号或换行分割
	for _, sep := range []string{",", "，", "\n"} {
		if strings.Contains(resp, sep) {
			parts := strings.Split(resp, sep)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" && len([]rune(p)) >= 2 {
					keywords = append(keywords, p)
				}
			}
			break
		}
	}

	// 如果没有分隔符，整个字符串作为一个关键词
	if len(keywords) == 0 && len([]rune(resp)) >= 2 {
		keywords = append(keywords, resp)
	}

	return keywords
}

// RefreshKeywordPool 刷新品牌的关键词池（调用 LLM 生成新一批关键词）。
func (uc *UseCase) RefreshKeywordPool(ctx context.Context, config *entity.CrawlerConfig, brandName, industry, positioning string) error {
	keywords, err := uc.GenerateKeywords(ctx, brandName, industry, positioning)
	if err != nil {
		return err
	}
	config.KeywordPool = keywords
	config.LastKeywordIndex = 0
	return uc.configRepo.Save(ctx, *config)
}

// CrawlResult 采集结果。
type CrawlResult struct {
	Platform    string    `json:"platform"`
	BrandID     string    `json:"brand_id"`
	Keywords    []string  `json:"keywords"`
	VideosFound int       `json:"videos_found"`
	VideosNew   int       `json:"videos_new"`
	DurationMs  int       `json:"duration_ms"`
	FinishedAt  time.Time `json:"finished_at"`
}


// archiveToLocal 异步转存：下载 VideoURL → 本地存储 → 回填 LocalVideoURL。
// 单个失败静默跳过（保留原始链接——用户要求的兜底语义）；并发限制 2 防打满带宽。
func (uc *UseCase) archiveToLocal(ctx context.Context, videos []entity.InspirationVideo) {
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	for _, v := range videos {
		if v.VideoURL == "" || v.LocalVideoURL != "" {
			continue
		}
		wg.Add(1)
		go func(v entity.InspirationVideo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			localURL, err := downloadToLocal(ctx, uc.media, v.VideoURL)
			if err != nil {
				log.Printf("[inspiration] 转存失败（保留原始链接）id=%s: %v", v.ID, err)
				return
			}
			v.LocalVideoURL = localURL
			if uErr := uc.videoRepo.Update(ctx, v); uErr != nil {
				log.Printf("[inspiration] 转存回填失败 id=%s: %v", v.ID, uErr)
			} else {
				log.Printf("[inspiration] 视频已本地转存 id=%s → %s", v.ID, localURL)
			}
		}(v)
	}
	wg.Wait()
}

// downloadToLocal 下载远端视频到媒体存储，返回可访问 URL。
// 大小上限 100MB（B站 360P 教程约 3.5MB/3.6min；抖音/快手短视频普遍 <30MB）。
func downloadToLocal(ctx context.Context, store port.MediaAssetStore, videoURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36")
	// B站 CDN 防盗链（其他平台无此校验不受影响）
	if u, pErr := url.Parse(videoURL); pErr == nil {
		if strings.Contains(u.Host, "bilivideo.com") || strings.Contains(u.Host, "hdslb.com") {
			req.Header.Set("Referer", "https://www.bilibili.com")
		}
	}
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 100<<20))
	if err != nil || len(data) == 0 {
		return "", fmt.Errorf("下载读取失败")
	}
	ext := ".mp4"
	if e := filepath.Ext(strings.Split(videoURL, "?")[0]); strings.EqualFold(e, ".webm") {
		ext = ".webm"
	}
	asset, err := store.SaveFile(ctx, "", "", "creation", data, "video/mp4", ext)
	if err != nil {
		return "", err
	}
	return asset.SourceURL, nil
}
