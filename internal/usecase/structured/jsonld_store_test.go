package structured

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
)

// 门店 NAP 增强（本地生活 P0）：文章页 JSON-LD @graph 双节点测试。

func TestBuildJSONLD_WithStore(t *testing.T) {
	store := &entity.StoreLocation{
		ID: "s1", BrandID: "b1", Name: "望京川菜馆", Address: "北京市朝阳区望京街10号",
		City: "北京市", District: "朝阳区", Lat: 39.99, Lng: 116.47,
		Phone: "010-12345678", Hours: "10:00-22:00", PriceLevel: "¥80/人", BizType: "Restaurant",
	}
	sd, err := buildJSONLD(jsonldInput{
		Title:        "望京川菜馆为什么值得去？",
		Content:      "正文内容……",
		URL:          "https://content.example.com/public/articles/oc-1",
		Author:       "望京川菜馆",
		BrandName:    "望京川菜馆",
		PublishDate:  time.Now(),
		ForceArticle: true,
		Store:        store,
	})
	if err != nil {
		t.Fatalf("buildJSONLD error: %v", err)
	}
	if !strings.Contains(sd.JSONLD, `"@graph"`) {
		t.Fatalf("有门店时应输出 @graph 双节点: %s", sd.JSONLD)
	}
	var parsed struct {
		Graph []map[string]any `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(sd.JSONLD), &parsed); err != nil {
		t.Fatalf("JSON-LD 非法: %v\n%s", err, sd.JSONLD)
	}
	if len(parsed.Graph) != 2 {
		t.Fatalf("@graph 应有 2 个节点, got %d", len(parsed.Graph))
	}
	// 找到门店节点
	var bizNode map[string]any
	for _, node := range parsed.Graph {
		if node["@type"] == "Restaurant" {
			bizNode = node
		}
	}
	if bizNode == nil {
		t.Fatalf("缺少 Restaurant 节点: %s", sd.JSONLD)
	}
	if bizNode["telephone"] != "010-12345678" || bizNode["openingHours"] != "10:00-22:00" {
		t.Errorf("门店 NAP 字段缺失: %+v", bizNode)
	}
	addr, ok := bizNode["address"].(map[string]any)
	if !ok || addr["streetAddress"] != "北京市朝阳区望京街10号" || addr["addressLocality"] != "北京市" {
		t.Errorf("门店地址结构错误: %+v", bizNode["address"])
	}
	geo, ok := bizNode["geo"].(map[string]any)
	if !ok || geo["latitude"] != 39.99 || geo["longitude"] != 116.47 {
		t.Errorf("门店坐标结构错误: %+v", bizNode["geo"])
	}
	// 主节点仍是 Article
	foundArticle := false
	for _, node := range parsed.Graph {
		if node["@type"] == "Article" {
			foundArticle = true
		}
	}
	if !foundArticle {
		t.Error("主节点应为 Article")
	}
}

func TestBuildJSONLD_NoStore_BehaviorUnchanged(t *testing.T) {
	// 无门店时行为与改造前一致：无 @graph，单节点 Article
	sd, err := buildJSONLD(jsonldInput{
		Title:        "标题",
		Content:      "正文……",
		ForceArticle: true,
	})
	if err != nil {
		t.Fatalf("buildJSONLD error: %v", err)
	}
	if strings.Contains(sd.JSONLD, `"@graph"`) {
		t.Errorf("无门店时不应输出 @graph: %s", sd.JSONLD)
	}
	if !strings.Contains(sd.JSONLD, `"@type": "Article"`) {
		t.Errorf("主节点应为 Article: %s", sd.JSONLD)
	}
}

func TestBuildStoreJSONLDNode_DefaultBizType(t *testing.T) {
	// BizType 为空 → 默认 LocalBusiness
	node := buildStoreJSONLDNode(jsonldInput{Store: &entity.StoreLocation{Address: "A街1号"}})
	if node["@type"] != "LocalBusiness" {
		t.Errorf("@type = %v, want LocalBusiness", node["@type"])
	}
}
