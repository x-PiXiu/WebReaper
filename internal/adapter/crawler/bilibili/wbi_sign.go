package bilibili

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// WBI 签名实现（参考 MediaCrawler bilibili/help.py）。
//
// 算法：MD5(url编码参数 + salt)
// salt 从 img_key + sub_key 通过固定映射表派生
// 密钥从 /x/web-interface/nav API 获取

// mixinKeyEncTab WBI 密钥混淆映射表（参考 MediaCrawler bilibili/help.py 第 39-44 行）。
var mixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49,
	33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16, 24, 55, 40,
	61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11,
	36, 20, 34, 44, 52,
}

// WBISigner WBI 签名器。
type WBISigner struct {
	imgKey  string
	subKey  string
	fetchedAt time.Time
}

// NewWBISigner 创建 WBI 签名器。
func NewWBISigner() *WBISigner {
	return &WBISigner{}
}

// SetKeys 手动设置密钥（管理后台配置时使用）。
func (s *WBISigner) SetKeys(imgKey, subKey string) {
	s.imgKey = imgKey
	s.subKey = subKey
	s.fetchedAt = time.Now()
}

// FetchKeys 从 B站 API 获取 WBI 密钥。
func (s *WBISigner) FetchKeys(client *http.Client, cookies string) error {
	req, err := http.NewRequest("GET", "https://api.bilibili.com/x/web-interface/nav", nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("获取 WBI 密钥失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			WbiImg struct {
				ImgURL string `json:"img_url"`
				SubURL string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析 WBI 密钥响应失败: %w", err)
	}

	// 从 URL 中提取文件名作为密钥
	// 例：https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png
	// 密钥 = 7cd084941338484aae1ad9425b84077c
	s.imgKey = extractKeyFromURL(result.Data.WbiImg.ImgURL)
	s.subKey = extractKeyFromURL(result.Data.WbiImg.SubURL)
	s.fetchedAt = time.Now()

	if s.imgKey == "" || s.subKey == "" {
		return fmt.Errorf("WBI 密钥为空")
	}

	return nil
}

// NeedRefresh 检查是否需要刷新密钥（密钥定期轮换，建议每 30 分钟刷新）。
func (s *WBISigner) NeedRefresh() bool {
	return s.imgKey == "" || s.subKey == "" || time.Since(s.fetchedAt) > 30*time.Minute
}

// Sign 对请求参数进行 WBI 签名。
//
// 算法步骤（参考 MediaCrawler bilibili/help.py 第 57-77 行）：
//  1. 添加 wts 时间戳
//  2. 按 key 排序
//  3. 过滤特殊字符 !'()*
//  4. URL 编码
//  5. MD5(url编码 + salt) 生成 w_rid
func (s *WBISigner) Sign(params map[string]string) map[string]string {
	// 1. 添加时间戳
	params["wts"] = strconv.FormatInt(time.Now().Unix(), 10)

	// 2. 按 key 排序
	sortedKeys := make([]string, 0, len(params))
	for k := range params {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// 3. 过滤特殊字符并构建有序参数
	filtered := make(map[string]string, len(params))
	for _, k := range sortedKeys {
		v := params[k]
		filtered[k] = filterChars(v, "!'()*")
	}

	// 4. URL 编码
	query := encodeParams(filtered)

	// 5. 生成签名
	salt := s.getSalt()
	wrid := md5Hex(query + salt)
	params["w_rid"] = wrid

	return params
}

// getSalt 派生 salt（固定映射表重排 img_key+sub_key，取前 32 字符）。
func (s *WBISigner) getSalt() string {
	mixinKey := s.imgKey + s.subKey
	if len(mixinKey) < 64 {
		return ""
	}
	salt := make([]byte, 0, 32)
	for _, i := range mixinKeyEncTab {
		salt = append(salt, mixinKey[i])
	}
	return string(salt[:32])
}

// extractKeyFromURL 从 WBI 图片 URL 中提取文件名作为密钥。
func extractKeyFromURL(u string) string {
	// https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png
	// → 7cd084941338484aae1ad9425b84077c
	parts := strings.Split(u, "/")
	if len(parts) == 0 {
		return ""
	}
	filename := parts[len(parts)-1]
	// 去掉扩展名
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		return filename[:idx]
	}
	return filename
}

// filterChars 过滤字符串中的指定字符。
func filterChars(s string, chars string) string {
	var result strings.Builder
	for _, ch := range s {
		if !strings.ContainsRune(chars, ch) {
			result.WriteRune(ch)
		}
	}
	return result.String()
}

// encodeParams 将参数 map 编码为 URL 查询字符串。
func encodeParams(params map[string]string) string {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return values.Encode()
}

// md5Hex 计算 MD5 哈希并返回十六进制字符串。
func md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", h)
}
