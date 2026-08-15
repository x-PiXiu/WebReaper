package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPEmbedder_Embed 请求/响应契约 + 维度推导。
func TestHTTPEmbedder_Embed(t *testing.T) {
	var gotModel string
	var gotInput []string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/embeddings") {
			t.Errorf("端点应为 /embeddings: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var req embedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotModel = req.Model
		gotInput = req.Input
		_ = json.NewEncoder(w).Encode(embedResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0}},
		})
	}))
	defer srv.Close()

	e := NewHTTPEmbedder("key-1", srv.URL, "embedding-test")
	vec, err := e.Embed(context.Background(), "你好世界")
	if err != nil {
		t.Fatalf("Embed 失败: %v", err)
	}
	if len(vec) != 3 || vec[1] != 0.2 {
		t.Errorf("向量解析错误: %v", vec)
	}
	if gotModel != "embedding-test" {
		t.Errorf("model 未传: %s", gotModel)
	}
	if len(gotInput) != 1 || gotInput[0] != "你好世界" {
		t.Errorf("input 错误: %v", gotInput)
	}
	if gotAuth != "Bearer key-1" {
		t.Errorf("Authorization 错误: %s", gotAuth)
	}
	if e.Dimension() != 3 {
		t.Errorf("维度应从响应推导为 3: %d", e.Dimension())
	}
}

// TestHTTPEmbedder_EmbedBatch 批量对齐 + 维度锁定。
func TestHTTPEmbedder_EmbedBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		// 按请求顺序返回不同向量
		out := embedResponse{Data: []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		}{}}
		for i := range req.Input {
			out.Data = append(out.Data, struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{Embedding: []float32{float32(i), float32(i) + 1}, Index: i})
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()

	e := NewHTTPEmbedder("k", srv.URL, "m")
	vecs, err := e.EmbedBatch(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("EmbedBatch 失败: %v", err)
	}
	if len(vecs) != 3 || vecs[2][0] != 2 {
		t.Errorf("批量结果未按请求顺序对齐: %v", vecs)
	}
	if e.Dimension() != 2 {
		t.Errorf("维度应为 2: %d", e.Dimension())
	}
}

// TestHTTPEmbedder_Dimensions 显式 dimensions 传请求体；0=不传（模型默认）。
func TestHTTPEmbedder_Dimensions(t *testing.T) {
	var gotReq embedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = embedRequest{} // 重置：json.Decode 复用对象不会清空缺失字段
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		_ = json.NewEncoder(w).Encode(embedResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{{Embedding: []float32{1}, Index: 0}},
		})
	}))
	defer srv.Close()

	// 显式维度 → 请求体带 dimensions
	e := NewHTTPEmbedder("k", srv.URL, "embedding-3", 512)
	if _, err := e.Embed(context.Background(), "x"); err != nil {
		t.Fatalf("Embed 失败: %v", err)
	}
	if gotReq.Dimensions != 512 {
		t.Errorf("dimensions 应传 512: %d", gotReq.Dimensions)
	}

	// 不传（0）→ 请求体无 dimensions
	e = NewHTTPEmbedder("k", srv.URL, "embedding-3")
	if _, err := e.Embed(context.Background(), "x"); err != nil {
		t.Fatalf("Embed 失败: %v", err)
	}
	if gotReq.Dimensions != 0 {
		t.Errorf("dimensions 应为 0（不传）: %d", gotReq.Dimensions)
	}
}

// TestHTTPEmbedder_Errors 非 2xx / 数量不符 / 空列表。
func TestHTTPEmbedder_Errors(t *testing.T) {
	// 非 2xx
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	e := NewHTTPEmbedder("bad", srv.URL, "m")
	if _, err := e.Embed(context.Background(), "x"); err == nil {
		t.Error("401 应报错")
	}
	srv.Close()

	// 响应数量不符
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResponse{})
	}))
	e = NewHTTPEmbedder("k", srv.URL, "m")
	if _, err := e.Embed(context.Background(), "x"); err == nil {
		t.Error("空 data 应报错")
	}
	srv.Close()

	// 空列表直接返回 nil（不发起请求）
	e = NewHTTPEmbedder("k", "http://127.0.0.1:1", "m")
	vecs, err := e.EmbedBatch(context.Background(), nil)
	if err != nil || vecs != nil {
		t.Errorf("空列表应返回 nil: vecs=%v err=%v", vecs, err)
	}
}
