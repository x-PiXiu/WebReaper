package qrlogin

import (
	"os"
	"strings"
	"testing"
)

// 用抖音真实样本验证空白检测与二维码解码（样本为本地调试产物，CI 环境自动跳过）：
//   - data/douyin_qr_fresh.png：被篡改的 toDataURL 导出的空白图（2598 字节）→ 空白
//   - data/douyin_qr_fixed.png：270x270 装饰插画（曾被视觉模型误判为二维码）→ 非空白但解不出码
//   - data/qrdbg3.png.canvas0.png：902x552 登录卡大 canvas，真二维码在右下角 → 可解出 douyin 链接
//   - data/qrdbg3.png.canvas1.png：180x180 装饰插画 → 非空白但解不出码
func TestIsBlankImage(t *testing.T) {
	blank, err := os.ReadFile("../../../data/douyin_qr_fresh.png")
	if err != nil {
		t.Skipf("空白样本不存在: %v", err)
	}
	if !isBlankImage(blank) {
		t.Errorf("空白图未被识别（%d 字节）", len(blank))
	}

	for _, p := range []string{"../../../data/douyin_qr_fixed.png", "../../../data/qrdbg3.png.canvas1.png"} {
		deco, dErr := os.ReadFile(p)
		if dErr != nil {
			continue
		}
		if isBlankImage(deco) {
			t.Errorf("装饰插画被误判为空白: %s", p)
		}
	}
}

func TestDecodeQRText(t *testing.T) {
	// 真二维码：大 canvas 截图，解码内容含 douyin 扫码登录链接
	real, err := os.ReadFile("../../../data/qrdbg3.png.canvas0.png")
	if err != nil {
		t.Skipf("真二维码样本不存在: %v", err)
	}
	text := decodeQRText(real)
	if text == "" || !strings.Contains(text, "douyin") {
		t.Errorf("真二维码未解出 douyin 链接: %q", text)
	}

	// 裁剪后必须仍是可解码的二维码
	if cropped := cropToQR(real); decodeQRText(cropped) == "" {
		t.Error("裁剪后的二维码不可解码")
	}

	// 装饰插画：视觉上像图但不是二维码，必须解不出内容
	for _, p := range []string{"../../../data/douyin_qr_fixed.png", "../../../data/qrdbg3.png.canvas1.png"} {
		deco, dErr := os.ReadFile(p)
		if dErr != nil {
			continue
		}
		if got := decodeQRText(deco); got != "" {
			t.Errorf("装饰插画 %s 被解出内容: %q", p, got)
		}
	}

	// 空白图：解不出内容
	if blank, bErr := os.ReadFile("../../../data/douyin_qr_fresh.png"); bErr == nil {
		if got := decodeQRText(blank); got != "" {
			t.Errorf("空白图被解出内容: %q", got)
		}
	}
}
