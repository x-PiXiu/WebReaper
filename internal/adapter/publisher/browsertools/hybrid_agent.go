// hybrid_agent.go 混合 Agent 循环：正常代码流程 + 异常时视觉 LLM 接管。
//
// 工作方式：
//   1. RPA 代码正常执行（chromedp + HumanAction）
//   2. 每步操作后调用 VerifySuccess() 检查 DOM
//   3. 验证失败时 → 调用 AgentRecover()（本文件的入口）
//      → 截屏 → 发给视觉 LLM → LLM 决策调用哪个浏览器工具 → 执行 → 验证恢复
//   4. 恢复后回到正常流程继续
//
// 成本：正常路径零 LLM 调用；异常路径每次约 0.05 元（1 次视觉分析 + 2-3 次工具调用）。
package browsertools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// VisionLLM 视觉 LLM 接口（由 adapter/ai 的 TrpcAgentGenerator 实现——
// 发图片+文本 prompt，返回决策 JSON）。
type VisionLLM interface {
	// AnalyzeScreenshot 分析截图并返回浏览器操作决策。
	// systemPrompt 描述 Agent 的任务上下文；screenshotBase64 是当前页面截图。
	// 返回 JSON: {"tool": "browser_click", "args": {"selector": "#close-btn"}, "reasoning": "看到弹窗需要关闭"}
	AnalyzeScreenshot(ctx context.Context, systemPrompt, screenshotBase64 string) (*AgentDecision, error)
}

// AgentDecision LLM 的决策结果。
type AgentDecision struct {
	Tool      string          `json:"tool"`      // 要调用的工具名（browser_click 等）
	Args      json.RawMessage `json:"args"`      // 工具参数（JSON）
	Reasoning string          `json:"reasoning"` // LLM 的推理过程（审计用）
	Done      bool            `json:"done"`      // 是否认为任务已完成
	Result    string          `json:"result"`    // Done=true 时的结果描述
}

// AgentRecoverInput 异常恢复的输入。
type AgentRecoverInput struct {
	Session        *BrowserSession // 浏览器会话
	LLM            VisionLLM      // 视觉 LLM
	TaskContext    string          // 任务描述（如"发布视频到抖音"）
	FailedStep     string          // 失败的步骤（如"等待上传按钮出现"）
	FailedSelector string          // 失败的选择器
	MaxRecoveries  int             // 最大恢复尝试次数（防死循环，默认 5）
}

// AgentRecover 混合 Agent 的异常恢复入口。
// 截屏 → LLM 分析 → 执行工具 → 验证 → 循环直到恢复或超限。
// 返回 nil = 恢复成功（可以继续正常流程）；返回 error = 恢复失败。
func AgentRecover(ctx context.Context, in AgentRecoverInput) error {
	if in.MaxRecoveries <= 0 {
		in.MaxRecoveries = 5
	}

	systemPrompt := buildRecoverPrompt(in.TaskContext, in.FailedStep, in.FailedSelector)

	for attempt := 0; attempt < in.MaxRecoveries; attempt++ {
		// 1. 截屏
		var screenshot []byte
		if err := chromedp.Run(in.Session.ctx, chromedp.FullScreenshot(&screenshot, 80)); err != nil {
			return fmt.Errorf("恢复失败：截屏失败 %w", err)
		}
		screenshotB64 := base64.StdEncoding.EncodeToString(screenshot)

		// 2. LLM 分析
		decision, err := in.LLM.AnalyzeScreenshot(ctx, systemPrompt, screenshotB64)
		if err != nil {
			return fmt.Errorf("恢复失败：LLM 分析出错 %w", err)
		}

		// 3. 检查是否认为已完成
		if decision.Done {
			return nil // LLM 认为已经恢复/完成
		}

		// 4. 执行 LLM 决策的工具
		result, err := executeToolByName(in.Session, decision.Tool, decision.Args)
		if err != nil {
			// 工具执行失败——让 LLM 下一轮看到错误重试
			fmt.Printf("[AgentRecover] 工具 %s 执行失败: %v（第 %d/%d 次）\n", decision.Tool, err, attempt+1, in.MaxRecoveries)
			continue
		}

		// 5. 验证是否恢复（检查原失败的选择器是否出现了）
		if in.FailedSelector != "" {
			verifyCtx, cancel := context.WithTimeout(in.Session.ctx, 5*time.Second)
			err := chromedp.Run(verifyCtx, chromedp.WaitVisible(in.FailedSelector))
			cancel()
			if err == nil {
				return nil // 选择器出现了——恢复正常流程
			}
		}

		// 6. 没恢复——继续循环（LLM 下一轮看到新的截图）
		fmt.Printf("[AgentRecover] 第 %d/%d 次恢复尝试：%s → %v\n",
			attempt+1, in.MaxRecoveries, decision.Reasoning, result)
	}

	return fmt.Errorf("恢复失败：已尝试 %d 次仍未恢复（步骤 %s，选择器 %s）",
		in.MaxRecoveries, in.FailedStep, in.FailedSelector)
}

// executeToolByName 按工具名执行（从 AllTools 中查找）。
func executeToolByName(session *BrowserSession, toolName string, args json.RawMessage) (any, error) {
	for _, t := range AllTools(session) {
		if t.Declaration().Name == toolName {
			return t.Call(context.Background(), args)
		}
	}
	return nil, fmt.Errorf("未知工具: %s", toolName)
}

// buildRecoverPrompt 构建恢复任务的 system prompt。
func buildRecoverPrompt(taskContext, failedStep, failedSelector string) string {
	var sb strings.Builder
	sb.WriteString("你是一个浏览器操作专家。当前任务：")
	sb.WriteString(taskContext)
	sb.WriteString("。\n\n")
	sb.WriteString(fmt.Sprintf("正常流程在步骤「%s」失败了（期望看到元素 %s 但未出现）。", failedStep, failedSelector))
	sb.WriteString("可能是遇到了以下情况之一：\n")
	sb.WriteString("- 弹出了意外的对话框/提示（需要关闭）\n")
	sb.WriteString("- 页面需要滑块验证（需要拖拽滑块）\n")
	sb.WriteString("- 页面改版了（需要用新的选择器）\n")
	sb.WriteString("- 页面还没加载完（需要等待）\n\n")
	sb.WriteString("请根据截图判断出了什么问题，然后调用相应的浏览器工具来处理。\n")
	sb.WriteString("处理完成后，如果看到期望的元素已经出现，调用 browser_done 表示恢复。\n\n")
	sb.WriteString("可用工具：browser_screenshot, browser_click, browser_type, browser_navigate, browser_wait, browser_scroll, browser_solve_slider, browser_get_text, browser_done\n\n")
	sb.WriteString("输出 JSON 格式：\n")
	sb.WriteString(`{"tool": "工具名", "args": {工具参数}, "reasoning": "你的分析", "done": false, "result": ""}`)
	return sb.String()
}

// ---- 常见异常的快捷处理（不经过 LLM——纯代码判断，零成本）----

// TryQuickRecover 尝试快速恢复（不调 LLM，纯代码判断常见异常）。
// 返回 true = 已恢复；false = 需要调用 AgentRecover（LLM 接管）。
func TryQuickRecover(session *BrowserSession, failedSelector string) bool {
	// 常见可快速处理的异常
	quickFixes := []struct {
		name     string // 异常描述
		detectJS string // 检测 JS（返回 true = 存在此异常）
		fixFunc  func(*BrowserSession) error // 修复操作
	}{
		{
			name: "意外弹窗（带关闭按钮）",
			detectJS: `(() => {
				const closeBtns = document.querySelectorAll('[aria-label="Close"], [class*="close"], button:contains("×"), [data-dismiss]');
				for (const btn of closeBtns) {
					if (btn.offsetWidth > 0 && btn.offsetHeight > 0) return true;
				}
				return false;
			})()`,
			fixFunc: func(s *BrowserSession) error {
				return chromedp.Run(s.ctx, chromedp.Evaluate(`
					const btns = document.querySelectorAll('[aria-label="Close"], [class*="close"], [data-dismiss]');
					for (const btn of btns) {
						if (btn.offsetWidth > 0) { btn.click(); break; }
					}
				`, nil))
			},
		},
		{
			name: "页面还在加载",
			detectJS: `document.readyState !== 'complete'`,
			fixFunc: func(s *BrowserSession) error {
				time.Sleep(3 * time.Second)
				return nil
			},
		},
	}

	for _, fix := range quickFixes {
		var detected bool
		if err := chromedp.Run(session.ctx, chromedp.Evaluate(fix.detectJS, &detected)); err != nil {
			continue
		}
		if detected {
			if err := fix.fixFunc(session); err != nil {
				continue
			}
			// 修复后检查目标选择器
			verifyCtx, cancel := context.WithTimeout(session.ctx, 3*time.Second)
			err := chromedp.Run(verifyCtx, chromedp.WaitVisible(failedSelector))
			cancel()
			if err == nil {
				return true // 快速修复成功
			}
		}
	}

	return false // 快速修复不成功——需要 Agent 接管
}
