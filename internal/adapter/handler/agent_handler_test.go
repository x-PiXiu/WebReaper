package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/port"
)

// TestHandleRun_BindsRequest 验证 AgentHandler 能正确解析请求体并调用 runner。
// 关键点：handler 依赖 port.AgentSyncRunner 接口（而非具体 struct），
// 这个测试用 stub 注入，证明 handler 与具体 adapter 实现解耦（DIP 落地）。
func TestHandleRun_BindsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stub := &stubRunner{}
	h := NewAgentHandler(stub)

	r := gin.New()
	r.POST("/agents/run", h.HandleRun)

	body := `{"task":"采集数据","system_prompt":"你是助手"}`
	req := httptest.NewRequest("POST", "/agents/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if stub.gotTask != "采集数据" {
		t.Errorf("runner got task = %q, want 采集数据", stub.gotTask)
	}
	if stub.gotPrompt != "你是助手" {
		t.Errorf("runner got prompt = %q, want 你是助手", stub.gotPrompt)
	}
}

// stubRunner 实现 port.AgentSyncRunner，记录入参。
type stubRunner struct {
	gotTask   string
	gotPrompt string
}

func (s *stubRunner) RunSync(_ context.Context, in port.AgentRunInput) (port.AgentRunOutput, error) {
	s.gotTask = in.Task
	s.gotPrompt = in.SystemPrompt
	return port.AgentRunOutput{Response: "ok"}, nil
}
