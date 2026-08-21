package handler

import (
	"context"
	"fmt"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// TransportAdminHandler 发布通道管理（admin：查看各平台通道矩阵 + 手动切换 + 恢复自动）。
//
// 场景：官方 API 与浏览器 RPA 双链路共存，自动降级链兜底；管理后台可按平台
// 强制指定通道（override 提到降级链头——管理员优先级，非死命令），
// 持久化 system_settings（重启恢复）。
type TransportAdminHandler struct {
	registry    *port.TransportRegistry
	settingRepo port.SystemSettingRepository
}

func NewTransportAdminHandler(tr *port.TransportRegistry, sr port.SystemSettingRepository) *TransportAdminHandler {
	return &TransportAdminHandler{registry: tr, settingRepo: sr}
}

const transportOverrideKey = "publish.transport_override"

// RestoreOverrides 启动时从 system_settings 恢复手动切换（main 调用）。
func (h *TransportAdminHandler) RestoreOverrides(ctx context.Context) {
	if h.registry == nil || h.settingRepo == nil {
		return
	}
	s, err := h.settingRepo.Get(ctx, transportOverrideKey)
	if err != nil || s.Value == "" {
		return
	}
	var m map[string]string
	if json.Unmarshal([]byte(s.Value), &m) == nil {
		h.registry.RestoreOverrides(m)
	}
}

// HandleList GET /admin/publish/transports —— 各平台通道矩阵 + 当前 override。
func (h *TransportAdminHandler) HandleList(c *gin.Context) {
	overrides := h.registry.Overrides()
	platforms := make([]gin.H, 0)
	for _, p := range h.registry.RegisteredPlatforms() {
		kinds := h.registry.Kinds(p)
		platforms = append(platforms, gin.H{
			"platform":      p,
			"available":     kinds,
			"override":      overrides[p],
			"mode":          overrideLabel(overrides[p]),
		})
	}
	success(c, gin.H{
		"platforms": platforms,
		"note":      "override=管理员指定通道（优先于自动降级链）；清除后恢复自动（api>rea>link 按凭证匹配）",
	})
}

// HandleSet PUT /admin/publish/transports/:platform —— 手动切换通道（body: {"kind":"rpa"}；kind="" 清除）。
func (h *TransportAdminHandler) HandleSet(c *gin.Context) {
	platform := c.Param("platform")
	var req struct {
		Kind string `json:"kind"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	// 校验：kind 必须是该平台已注册通道（防止设置成不存在的通道）
	if req.Kind != "" {
		found := false
		for _, k := range h.registry.Kinds(platform) {
			if k == req.Kind {
				found = true
				break
			}
		}
		if !found {
			fail(c, fmt.Errorf("通道 "+req.Kind+" 未在该平台注册（可用：%s）", joinKinds(h.registry.Kinds(platform))))
			return
		}
	}

	h.registry.SetOverride(platform, req.Kind)

	// 持久化（全量 overrides 快照存 JSON；重启恢复）
	if h.settingRepo != nil {
		if raw, jErr := json.Marshal(h.registry.Overrides()); jErr == nil {
			_ = h.settingRepo.Save(c.Request.Context(), entity.SystemSetting{
				Key: transportOverrideKey, Value: string(raw), UpdatedAt: time.Now(),
			})
		}
	}
	success(c, gin.H{"platform": platform, "override": req.Kind, "mode": overrideLabel(req.Kind)})
}

func overrideLabel(kind string) string {
	if kind == "" {
		return "auto"
	}
	return "manual:" + kind
}

func joinKinds(ks []string) string {
	out := ""
	for i, k := range ks {
		if i > 0 {
			out += ","
		}
		out += k
	}
	return out
}
