package qrlogin

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// TestAcceptAnchorQRImage 用真实抖音登录页 DOM 提取的二维码（data/douyin_qr_real_dom.b64）
// 验证阶段0的采信逻辑。实测关键事实（2026-08）：中央叠 40x40 抖音 logo 的二维码
// gozxing 可正常解出（logo 面积 ~9% 在容错内），走「解码确认 + cropToQR 裁剪」路径；
// 解不出时的放行分支是样式化二维码的防御（同快手 URL 直传先例）。
// 样本缺失时自动跳过（CI 环境）。
func TestAcceptAnchorQRImage(t *testing.T) {
	b64, err := os.ReadFile("../../../data/douyin_qr_real_dom.b64")
	if err != nil {
		t.Skipf("真实样本不存在: %v", err)
	}
	src := "data:image/png;base64," + strings.TrimSpace(string(b64))

	if text := decodeQRText(decodeB64(t, src)); text != "" {
		t.Logf("注意：样本已可被 gozxing 解码（%s）——logo 遮挡场景的假设不再成立，但采信逻辑仍兼容", truncURL(text))
	}

	q := NewChromedpQRLogin(false)
	q.sessions["t1"] = &loginSession{status: "preparing", platform: "douyin"}
	if !q.acceptAnchorQRImage(context.Background(), "t1", src) {
		t.Fatal("真实抖音二维码（带中央 logo）被锚点采信逻辑拒绝")
	}
	sess := q.sessions["t1"]
	if sess.status != "waiting" || sess.qrImage == "" {
		t.Fatalf("会话未进入 waiting：status=%s qrImage=%d 字符", sess.status, len(sess.qrImage))
	}

	// 空白图必须被拦截（toDataURL 篡改场景）
	if blank, bErr := os.ReadFile("../../../data/douyin_qr_fresh.png"); bErr == nil {
		q.sessions["t2"] = &loginSession{status: "preparing"}
		blankSrc := "data:image/png;base64," + encodeB64(blank)
		if q.acceptAnchorQRImage(context.Background(), "t2", blankSrc) {
			t.Error("空白图未被拦截")
		}
		if s := q.sessions["t2"]; s.status == "waiting" {
			t.Error("空白图被错误写入会话")
		}
	}
}

// TestAcceptAnchorQRImageKuaishou 用真实快手登录页 DOM 提取的二维码（data/kuaishou_qr_real_dom.b64）
// 验证锚点采信对艺术字二维码的兼容：gozxing 可能解不出（样式化码），但锚点命中
// （img[alt="qrcode"] / .qrcode 容器）即返回原始图供手机扫码。样本 ~510 字节，
// 同时固化 data URL 长度阈值（原 500 贴边误杀）。样本缺失时自动跳过（CI 环境）。
func TestAcceptAnchorQRImageKuaishou(t *testing.T) {
	b64, err := os.ReadFile("../../../data/kuaishou_qr_real_dom.b64")
	if err != nil {
		t.Skipf("真实样本不存在: %v", err)
	}
	src := "data:image/png;base64," + strings.TrimSpace(string(b64))

	q := NewChromedpQRLogin(false)
	q.sessions["ks1"] = &loginSession{status: "preparing", platform: "kuaishou"}
	if !q.acceptAnchorQRImage(context.Background(), "ks1", src) {
		t.Fatal("真实快手艺术字二维码被锚点采信逻辑拒绝（检查长度阈值/空白误判）")
	}
	sess := q.sessions["ks1"]
	if sess.status != "waiting" || sess.qrImage == "" {
		t.Fatalf("会话未进入 waiting：status=%s qrImage=%d 字符", sess.status, len(sess.qrImage))
	}
	// 解码成功则应为裁剪图，解不出应为原始图——两者都算通过，日志区分
	if text := decodeQRText(decodeB64(t, src)); text != "" {
		t.Logf("快手样本可解码 → %s（应走裁剪增强路径）", truncURL(text))
	} else {
		t.Log("快手样本 gozxing 解不出（艺术字样式）——已走放行分支返回原始图，手机可扫")
	}
}

func decodeB64(t *testing.T, dataURL string) []byte {
	t.Helper()
	comma := strings.Index(dataURL, ",")
	if comma < 0 {
		t.Fatal("data URL 无逗号")
	}
	return []byte(dataURL[comma+1:])
}

func encodeB64(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

// TestPollStatusCancelledForMissingSession 固化修复：用户点叉取消（DELETE 清理）后
// 迟到的轮询 GET 不再返回 "session not found" 500——「会话已终结」是正常生命周期，
// 返回 cancelled 状态（HTTP 200），前端据此静默停止轮询。
func TestPollStatusCancelledForMissingSession(t *testing.T) {
	q := NewChromedpQRLogin(false)
	res, err := q.PollStatus(context.Background(), "qr-not-exist")
	if err != nil {
		t.Fatalf("不存在的 session 不应返回错误: %v", err)
	}
	if res.Status != "cancelled" {
		t.Fatalf("期望 cancelled 状态，得到 %q", res.Status)
	}
}

// TestAcceptAnchorQRImageBilibili 用真实 B站登录页 DOM 提取的二维码
// （data/bilibili_qr_real_dom.b64，来自 img[alt="Scan me!"] 的 data URL）
// 验证锚点采信：B站 qrcode 库渲染 canvas 后转 img 并隐藏 canvas（隐藏元素截图
// 必空白），阶段0直取 img 的 data URL。样本缺失时自动跳过（CI 环境）。
func TestAcceptAnchorQRImageBilibili(t *testing.T) {
	b64, err := os.ReadFile("../../../data/bilibili_qr_real_dom.b64")
	if err != nil {
		t.Skipf("真实样本不存在: %v", err)
	}
	src := "data:image/png;base64," + strings.TrimSpace(string(b64))

	if text := decodeQRText(decodeB64(t, src)); text == "" {
		t.Log("样本 gozxing 解不出（可能带中央 logo）——将走放行分支")
	} else if !strings.Contains(text, "bilibili") {
		t.Errorf("解码内容不含 bilibili 链接: %q", truncURL(text))
	}

	q := NewChromedpQRLogin(false)
	q.sessions["bili1"] = &loginSession{status: "preparing", platform: "bilibili"}
	if !q.acceptAnchorQRImage(context.Background(), "bili1", src) {
		t.Fatal("真实 B站二维码被锚点采信逻辑拒绝")
	}
	sess := q.sessions["bili1"]
	if sess.status != "waiting" || sess.qrImage == "" {
		t.Fatalf("会话未进入 waiting：status=%s qrImage=%d 字符", sess.status, len(sess.qrImage))
	}
}
