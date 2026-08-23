// Package hotvideo 实现热门同款视频发现用例（人设档案 · 热门同款 Tab）。
//
// 产品语义：给商户看"跟你品牌同赛道、最近很火"的短视频——可直接播放参考，
// 一键「拍摄同款」带选题进创作模式。获客智能体的"抄爆款"入口。
//
// 数据链路（LLM + 网络搜索，个体户无平台数据 API 权限的现实约束下）：
//  1. 品牌档案（行业/定位/卖点）→ LLM 生成 3-4 个搜索词（"行业词 + 爆款/热门视频"形态）
//  2. LinkSearcher 逐词搜索 → 合并去重（保留标题+URL+摘要）
//  3. LLM 从搜索结果中筛选结构化出 ≤8 个视频：标题/链接/火爆点/拍摄同款选题建议
//  4. 24h 内存缓存（按品牌）——搜索+两次 LLM 有成本，商户一天内反复进 tab 不重跑
package hotvideo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// HotVideo 热门同款视频（类型别名——entity 包统一定义，热路径无需 import entity）。
type HotVideo = entity.HotVideo

// ListOptions 热门视频列表查询选项（类型别名）。
type ListOptions = entity.HotVideoListOptions

// HotVideoUseCase 热门视频发现用例。
type HotVideoUseCase struct {
	brandRepo    port.BrandRepository
	searcher     port.LinkSearcher   // 通用搜索引擎（Bing/DDG）——降级数据源
	douyin       port.SocialSearcher // 站内搜索——主数据源（真实爆款+数据，需 cookie 账号；多平台泛化）
	aiGen        port.AIGenerator
	hotVideoRepo entity.HotVideoRepository // 热门视频持久化（可选注入；nil=不落库只走缓存）

	mu               sync.Mutex
	cache            map[string]cacheEntry // brandID → 结果缓存（24h）
	lastDouyinSearch time.Time             // 上次站内搜索时间（全局频率限制——保护商户账号）
}

// SetHotVideoRepo 注入持久化仓储（可选——未注入则不落库，只走内存缓存）。
func (uc *HotVideoUseCase) SetHotVideoRepo(repo entity.HotVideoRepository) {
	uc.hotVideoRepo = repo
}

// douyinSearchCooldown 站内搜索全局冷却时间。
// 原因：每次搜索 = 开Chrome→cookie注入→搜索页→XHR→关Chrome，
// 短时间反复触发会被抖音风控标记（verify_check→账号标记→极端情况封号）。
// 24h 缓存是主防线；force=true（换一批）也受此冷却限制。
const douyinSearchCooldown = 10 * time.Minute

type cacheEntry struct {
	videos   []HotVideo
	expireAt time.Time
	cachedAt time.Time
}

const cacheTTL = 12 * time.Hour

const forceCooldownTTL = 24 * time.Hour

// SetSocialSearcher 注入站内搜索（可选；未注入或无 cookie 账号时走通用搜索链路）。
func (uc *HotVideoUseCase) SetSocialSearcher(ds port.SocialSearcher) {
	uc.douyin = ds
}

func NewHotVideoUseCase(br port.BrandRepository, searcher port.LinkSearcher, aiGen port.AIGenerator) *HotVideoUseCase {
	return &HotVideoUseCase{
		brandRepo: br,
		searcher:  searcher,
		aiGen:     aiGen,
		cache:     make(map[string]cacheEntry),
	}
}

// ListHotVideos 发现品牌同赛道的热门视频（缓存 24h；force=true 跳过缓存重搜）。
func (uc *HotVideoUseCase) ListHotVideos(ctx context.Context, tenantID, brandID string, force bool) ([]HotVideo, error) {
	uc.mu.Lock()
	if e, ok := uc.cache[brandID]; ok {
		if !force && time.Now().Before(e.expireAt) {
			uc.mu.Unlock()
			return e.videos, nil
		}
		if force && time.Since(e.cachedAt) < forceCooldownTTL {
			remaining := int(forceCooldownTTL/time.Hour) - int(time.Since(e.cachedAt)/time.Hour)
			uc.mu.Unlock()
			return nil, fmt.Errorf("冷却中（约 %d 小时后可换一批）——频繁搜索有账号风控风险", remaining)
		}
	}
	uc.mu.Unlock()

	brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
	if err != nil {
		return nil, fmt.Errorf("品牌不存在: %w", err)
	}

	// 主数据源：抖音站内搜索（真实爆款视频+播放/点赞数据）。失败降级通用搜索链路。
	if uc.douyin != nil {
		if videos, dErr := uc.searchDouyinHot(ctx, tenantID, brand); dErr == nil && len(videos) > 0 {
			uc.mu.Lock()
			uc.cache[brandID] = cacheEntry{videos: videos, expireAt: time.Now().Add(cacheTTL), cachedAt: time.Now()}
			uc.mu.Unlock()
			uc.persistVideos(ctx, tenantID, brandID, videos, "douyin")
			return videos, nil
		} else if dErr != nil {
			log.Printf("[HotVideo] 抖音站内搜索降级（%v），走通用搜索引擎链路", dErr)
		}
	}

	// 步骤 1：LLM 生成搜索词（品牌行业+定位+卖点 → 3-4 个"找爆款视频"查询）
	queries, err := uc.genQueries(ctx, brand)
	if err != nil {
		return nil, err
	}

	// 步骤 2：逐词搜索合并去重（词间留 600ms 间隔——连续请求易触发搜索引擎限流）
	seen := make(map[string]bool)
	var links []port.SearchLink
	for _, q := range queries {
		for _, l := range uc.searcher.SearchLinks(ctx, q, 5) {
			if l.URL == "" || seen[l.URL] {
				continue
			}
			seen[l.URL] = true
			links = append(links, l)
		}
		time.Sleep(600 * time.Millisecond)
	}
	log.Printf("[HotVideo] 搜索完成：queries=%d links=%d brand=%s", len(queries), len(links), brandID)
	if len(links) == 0 {
		return []HotVideo{}, nil
	}

	// 步骤 3：LLM 筛选结构化（≤8 个，含火爆点与拍摄同款选题）
	videos := uc.curate(ctx, brand, links)
	log.Printf("[HotVideo] curate 完成：videos=%d", len(videos))

	uc.mu.Lock()
	uc.cache[brandID] = cacheEntry{videos: videos, expireAt: time.Now().Add(cacheTTL)}
	uc.mu.Unlock()
	uc.persistVideos(ctx, tenantID, brandID, videos, "search")
	return videos, nil
}

// ListFromDB 从 DB 列出热门视频（支持搜索/排序/分页——替代纯缓存的 ListHotVideos）。
func (uc *HotVideoUseCase) ListFromDB(ctx context.Context, brandID string, opts ListOptions) ([]HotVideo, int, error) {
	if uc.hotVideoRepo == nil {
		return nil, 0, nil
	}
	return uc.hotVideoRepo.List(ctx, brandID, entity.HotVideoListOptions{Platform: opts.Platform, Keyword: opts.Keyword, SortBy: opts.SortBy, Limit: opts.Limit, Offset: opts.Offset})
}

// persistVideos 搜索结果落库（异步，失败仅日志不阻断主流程）。
func (uc *HotVideoUseCase) persistVideos(ctx context.Context, tenantID, brandID string, videos []HotVideo, source string) {
	if uc.hotVideoRepo == nil || len(videos) == 0 {
		return
	}
	for i := range videos {
		videos[i].TenantID = tenantID
		videos[i].BrandID = brandID
		if videos[i].Source == "" {
			videos[i].Source = source
		}
	}
	if n, err := uc.hotVideoRepo.SaveBatch(ctx, videos); err != nil {
		log.Printf("[HotVideo] 持久化失败: %v", err)
	} else if n > 0 {
		log.Printf("[HotVideo] 持久化 %d 条新视频（brand=%s, source=%s）", n, brandID, source)
	}
}

// searchDouyinHot 主数据源：抖音站内搜索（一周内+最多点赞）→ LLM 生成火爆点与拍摄同款选题。
// 搜索词用品牌行业/定位直构（站内搜索对短词效果最好，无需 LLM 造句）。
func (uc *HotVideoUseCase) searchDouyinHot(ctx context.Context, tenantID string, brand entity.Brand) ([]HotVideo, error) {
	// 站内搜索对短词（2-6 字）效果最好——优先用行业词，其次从定位中拆分短词
	var keywords []string
	if brand.Industry != "" {
		keywords = append(keywords, brand.Industry)
	}
	if brand.Positioning != "" {
		for _, seg := range strings.FieldsFunc(brand.Positioning, func(r rune) bool { return r == '，' || r == '。' || r == ' ' }) {
			if 2 <= len([]rune(seg)) && len([]rune(seg)) <= 8 {
				keywords = append(keywords, seg)
			}
		}
	}
	if len(keywords) == 0 {
		keywords = append(keywords, clamp(brand.Positioning, 6))
	} else if len(keywords) > 3 {
		keywords = keywords[:3] // 最多搜 3 个词
	}

	// 全局频率限制：保护商户账号（短时间反复搜索触发抖音风控→账号标记）
	uc.mu.Lock()
	if time.Since(uc.lastDouyinSearch) < douyinSearchCooldown {
		remaining := int(douyinSearchCooldown/time.Minute) - int(time.Since(uc.lastDouyinSearch)/time.Minute)
		uc.mu.Unlock()
		return nil, fmt.Errorf("站内搜索冷却中（约 %d 分钟后恢复）——频繁搜索有账号风控风险", remaining)
	}
	uc.lastDouyinSearch = time.Now()
	uc.mu.Unlock()

	// 单个关键词失败不放弃——太具体的词（"20年老店"）返回空是正常的，
	// 只要其他词有结果就继续；全部失败才降级到通用搜索引擎
	seen := make(map[string]bool)
	var videos []port.SocialVideo
	var succeeded int
	for _, kw := range keywords {
		list, err := uc.douyin.SearchHotVideos(ctx, tenantID, "douyin", kw, 10)
		if err != nil {
			log.Printf("[HotVideo] 站内搜索 %q 失败: %v（继续下一词）", kw, err)
			continue
		}
		succeeded++
		for _, v := range list {
			if v.VideoID == "" || seen[v.VideoID] {
				continue
			}
			seen[v.VideoID] = true
			videos = append(videos, v)
		}
		time.Sleep(5 * time.Second) // 两次搜索留间隔（太短触发抖音 verify_check 频率风控）
	}
	if succeeded == 0 || len(videos) == 0 {
		return nil, fmt.Errorf("站内搜索全部关键词无结果（%d 个词）", len(keywords))
	}
	log.Printf("[HotVideo] 站内搜索：%d/%d 个词成功，共 %d 个视频", succeeded, len(keywords), len(videos))
	// 按点赞数排序取前 8（最近很火）
	sort.Slice(videos, func(i, j int) bool { return videos[i].DiggCount > videos[j].DiggCount })
	if len(videos) > 8 {
		videos = videos[:8]
	}
	return uc.curateDouyin(ctx, brand, videos), nil
}

// curateDouyin LLM 为真实爆款视频生成"为什么火 + 拍摄同款选题"（一次调用批量生成）。
// LLM 失败时降级：数据+链接仍然真实可用，选题用行业兜底文案。
func (uc *HotVideoUseCase) curateDouyin(ctx context.Context, brand entity.Brand, videos []port.SocialVideo) []HotVideo {
	var b strings.Builder
	for i, v := range videos {
		fmt.Fprintf(&b, "%d. 标题：%s（@%s，播放%d 赞%d 评%d）\n", i+1,
			clamp(v.Desc, 40), v.Author, v.PlayCount, v.DiggCount, v.CommentCount)
	}
	prompt := fmt.Sprintf(`你是短视频获客专家。以下是抖音上"%s"行业最近一周的爆款视频（真实数据）：
%s
为每个视频输出：
- hot_point：为什么火（一句话，说清商户能抄的点）
- topic：拍摄同款选题建议（15-30 字，结合行业"%s"，不写具体品牌名和账号名）
只输出 JSON：{"items": [{"i": 序号, "hot_point": "...", "topic": "..."}]}`,
		industryOf(brand), b.String(), industryOf(brand))

	hotPoints := make(map[int]string)
	topics := make(map[int]string)
	if resp, err := uc.chat(ctx, prompt); err == nil {
		var out struct {
			Items []struct {
				I        int    `json:"i"`
				HotPoint string `json:"hot_point"`
				Topic    string `json:"topic"`
			} `json:"items"`
		}
		if jErr := json.Unmarshal([]byte(pkg.ExtractJSONBlock(resp)), &out); jErr == nil {
			for _, it := range out.Items {
				hotPoints[it.I] = it.HotPoint
				topics[it.I] = it.Topic
			}
		}
	}

	result := make([]HotVideo, 0, len(videos))
	for i, v := range videos {
		result = append(result, HotVideo{
			Title:    clamp(v.Desc, 40),
			URL:      v.URL,
			Platform: "douyin",
			HotPoint: hotPoints[i+1],
			Topic:    topics[i+1],
		})
		if result[i].Topic == "" {
			result[i].Topic = "参考这条爆款的角度，拍一条" + industryOf(brand) + "同款获客视频"
		}
	}
	return result
}

// genQueries 步骤 1：LLM 生成搜索词。
func (uc *HotVideoUseCase) genQueries(ctx context.Context, brand entity.Brand) ([]string, error) {
	prompt := fmt.Sprintf(`你是短视频获客专家。品牌信息：
- 名称：%s
- 行业：%s
- 定位：%s
- 核心卖点：%s

生成 4 个搜索引擎查询词，用于找到"该行业最近很火的短视频"。
要求：
- 目标是搜到【爆款视频盘点文章、行业榜单、具体热门视频页】——平台首页/官网/登录页没有用
- 优先形态："{行业} 抖音爆款视频盘点"、"{行业} 热门短视频 榜单"、"{行业} 账号 火了 复盘" 等
- 查询词面向搜索引擎（Bing），自然口语，避免堆砌；每个 10-25 字。
只输出 JSON：{"queries": ["...", "...", "...", "..."]}`,
		brand.Name, industryOf(brand), clamp(brand.Positioning, 60), strings.Join(brand.CoreSelling, "、"))

	resp, err := uc.chat(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("生成搜索词失败: %w", err)
	}
	var out struct {
		Queries []string `json:"queries"`
	}
	if jErr := json.Unmarshal([]byte(pkg.ExtractJSONBlock(resp)), &out); jErr != nil || len(out.Queries) == 0 {
		// LLM 输出异常：降级用行业词直搜，链路不断
		return fallbackQueries(brand), nil
	}
	if len(out.Queries) > 4 {
		out.Queries = out.Queries[:4]
	}
	return out.Queries, nil
}

// curate 步骤 3：LLM 从搜索结果筛选结构化视频列表；解析失败降级返回原始链接。
func (uc *HotVideoUseCase) curate(ctx context.Context, brand entity.Brand, links []port.SearchLink) []HotVideo {
	var b strings.Builder
	for i, l := range links {
		if i >= 24 {
			break // 控制上下文长度
		}
		fmt.Fprintf(&b, "%d. 标题：%s\n   链接：%s\n   摘要：%s\n", i+1, clamp(l.Title, 60), l.URL, clamp(l.Content, 100))
	}

	prompt := fmt.Sprintf(`你是短视频获客专家。以下是关于"%s"行业热门内容的搜索结果：
%s
从中挑选最多 8 个【适合本地商户参考模仿的爆款短视频】（优先链接直接指向抖音/快手/小红书视频页的条目；盘点文章里提到的代表性视频也可，链接用文章链接）。
为每个输出：
- title：视频标题（或从条目提炼）
- url：链接（原样保留）
- hot_point：为什么火（一句话，说清可抄的点，如"前后对比反差强"、"探店沉浸感"）
- topic：拍摄同款选题建议（15-30 字，商户照着拍的角度，结合行业"%s"，不要写品牌名）

合格条目分两类（都给用户参考价值）：
1. 具体视频页（抖音/快手/小红书/B站站内视频链接）
2. 与行业强相关的【爆款盘点文章/拍摄方法复盘】——标题或摘要必须与"%s"行业明确相关，且内容涉及具体视频案例、爆款拍法或账号案例
⚠️ 剔除：平台首页/官网/登录页/直播入口/帮助中心，以及与行业无关的杂讯。没有合格条目才返回 {"videos": []}。
只输出 JSON：{"videos": [{"title": "...", "url": "...", "hot_point": "...", "topic": "..."}]}`,
		industryOf(brand), b.String(), industryOf(brand), industryOf(brand))

	resp, err := uc.chat(ctx, prompt)
	if err != nil {
		log.Printf("[HotVideo] curate LLM 调用失败: %v（降级用原始搜索结果）", err)
		return degrade(links)
	}
	log.Printf("[HotVideo] curate LLM 响应长度=%d 首部=%.120s", len(resp), resp)
	var out struct {
		Videos []HotVideo `json:"videos"`
	}
	if jErr := json.Unmarshal([]byte(pkg.ExtractJSONBlock(resp)), &out); jErr != nil {
		log.Printf("[HotVideo] curate JSON 解析失败: %v（降级）", jErr)
		return degrade(links)
	}
	if len(out.Videos) == 0 {
		// LLM 判空（通用搜索引擎基本不索引抖音站内视频页的客观天花板）——
		// 二次放宽：行业强相关的文章作为内容参考，保证 tab 有料
		log.Printf("[HotVideo] curate 判空，降级为行业文章参考")
		if fb := articleFallback(brand, links); len(fb) > 0 {
			return fb
		}
		return degrade(links)
	}
	if len(out.Videos) > 8 {
		out.Videos = out.Videos[:8]
	}
	for i := range out.Videos {
		out.Videos[i].URL = strings.TrimSpace(out.Videos[i].URL)
		out.Videos[i].Platform = platformOf(out.Videos[i].URL)
	}
	return out.Videos
}

// articleFallback 行业文章参考降级：标题与行业词相关的普通网页（菜谱/榜单/方法文），
// 作为"内容方向参考"展示——比空 tab 好，比垃圾链接诚实。
func articleFallback(brand entity.Brand, links []port.SearchLink) []HotVideo {
	industry := strings.ToLower(industryOf(brand))
	videos := make([]HotVideo, 0, 6)
	for _, l := range links {
		if len(videos) >= 6 {
			break
		}
		if l.URL == "" || isPlatformUtilityPage(l.URL) {
			continue
		}
		title := strings.ToLower(l.Title)
		if !strings.Contains(title, industry) {
			continue
		}
		videos = append(videos, HotVideo{
			Title:    clamp(l.Title, 40),
			URL:      l.URL,
			Platform: platformOf(l.URL),
			HotPoint: "行业内容参考",
			Topic:    "参考《" + clamp(l.Title, 16) + "》的内容方向，拍一条同款获客视频",
		})
	}
	return videos
}

// degrade LLM 不可用/解析失败时的降级：只保留短视频平台链接（抖音/快手/小红书/B站）——
// 全是普通网页说明搜索质量差，宁返回空让前端提示"未发现"，不塞无关结果充数。
func degrade(links []port.SearchLink) []HotVideo {
	videos := make([]HotVideo, 0, 8)
	for _, l := range links {
		if len(videos) >= 8 {
			break
		}
		if l.URL == "" || platformOf(l.URL) == "web" || isPlatformUtilityPage(l.URL) {
			continue
		}
		videos = append(videos, HotVideo{
			Title:    clamp(l.Title, 40),
			URL:      l.URL,
			Platform: platformOf(l.URL),
			Topic:    "拍一条同行业热门同款视频",
		})
	}
	return videos
}

// chat 无状态单轮对话（conversationID 空 = 无上下文）。
func (uc *HotVideoUseCase) chat(ctx context.Context, prompt string) (string, error) {
	return uc.aiGen.ChatStream(ctx, "", "", []port.ChatMessage{
		{Role: "user", Content: prompt},
	}, nil)
}

// ---- 纯函数辅助 ----

func industryOf(b entity.Brand) string {
	if b.Industry != "" {
		return b.Industry
	}
	return clamp(b.Positioning, 12)
}

func fallbackQueries(b entity.Brand) []string {
	ind := industryOf(b)
	return []string{
		ind + " 抖音爆款视频",
		ind + " 热门短视频 盘点",
		ind + " 获客 短视频 怎么拍",
	}
}

// platformOf 从 URL 域名推断平台标识（前端徽标展示用）。
func platformOf(rawURL string) string {
	u := strings.ToLower(rawURL)
	switch {
	case strings.Contains(u, "douyin.com"):
		return "douyin"
	case strings.Contains(u, "kuaishou.com") || strings.Contains(u, "ksurl"):
		return "kuaishou"
	case strings.Contains(u, "xiaohongshu.com") || strings.Contains(u, "xhslink"):
		return "xiaohongshu"
	case strings.Contains(u, "bilibili.com") || strings.Contains(u, "b23.tv"):
		return "bilibili"
	case strings.Contains(u, "weishi"):
		return "weishi"
	default:
		return "web"
	}
}

// isPlatformUtilityPage 判定平台功能页（首页/官网/登录/直播入口）——不是具体视频，展示无意义。
func isPlatformUtilityPage(rawURL string) bool {
	u := strings.ToLower(rawURL)
	// 子路径关键词：命中即平台功能页（登录/创作者中心/直播/开放平台/DOU+投放）
	for _, kw := range []string{
		"douyin.com/login", "live.douyin.com", "creator.douyin.com",
		"open.douyin.com", "e.douyin.com", "douplus",
		"cp.kuaishou.com", "kuaishou.com/login", "open.kuaishou.com",
		"xiaohongshu.com/login", "bilibili.com/login",
	} {
		if strings.Contains(u, kw) {
			return true
		}
	}
	// 根路径精确匹配（平台首页）：域名后无路径
	for _, host := range []string{"www.douyin.com", "www.kuaishou.com", "www.xiaohongshu.com", "www.bilibili.com"} {
		if u == "https://"+host || u == "https://"+host+"/" || u == "https://"+host+"/?" {
			return true
		}
	}
	return false
}

func clamp(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
