// panda_calibration.go 发布通道校准工具层（平台选择器知识来源：panda-video-automations-publisher
// ——Playwright RPA 发布器的真机实测选择器链，2026-08 移植校准）。
//
// 背景：本包各通道的选择器一直是"多策略候选但真机未验证"状态；panda 项目
// 在 5 个平台真机跑通，其候选链/平台必选项（抖音自主声明、B站创作声明、
// 快手事件派发、微信位置关闭）是现成的实测答案。本文件把这些知识沉淀为
// 四通道共用的流程原语。
package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/page"

	"github.com/chromedp/chromedp"
)

// ---- 上传文件：三级降级（panda 实测模式）----

// setUploadFileCascade 向页面上传视频文件（四级降级，panda 实测模式 + 2026-08-28 DRY_RUN 修正）：
//  ① 现存 input 直传（CDP SetUploadFiles 是 trusted 事件；隐藏元素也有效——panda 实测
//     input 候选不限于 type=file，快手的 input 只带 accept 属性）
//  ② filechooser 拦截（真触发平台上传逻辑：点平台自己的上传区 → CDP EventFileChooser
//     事件携带 backendNodeId → 注入文件。2026-08-28 DRY_RUN 实测：快手无 type=file
//     input 且 DOM 注入的 input 平台不监听，必须走此路径）
//  ③ DOM 注入 input 兜底（panda B站实测：页面监听 document 级 change 的平台有效）
//  ④ 顺序重试一轮（部分平台点击后才动态创建 input）
func setUploadFileCascade(ctx context.Context, filePath string) error {
	// ① 现存 input（presence 判定，不要求可见；候选对齐 panda）
	inputQuery := `input[type=file], input[accept*="video"], input[accept*="mp4"]`
	if ok, _ := evalBool(ctx, `document.querySelectorAll('input[type=file], input[accept*="video"], input[accept*="mp4"]').length > 0`); ok {
		if err := setInputFilesByQuery(ctx, inputQuery, filePath); err == nil {
			log.Printf("[PublishAuto] 上传完成（现存 input 直传）")
			return nil
		}
	}
	// ② filechooser 拦截 + 点击平台上传区（CDP 等价 Playwright filechooser 模式）
	if uploadViaFileChooser(ctx, filePath) {
		return nil
	}
	// ③ DOM 注入 input（B站类平台有效）
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = 'video/*';
		input.style.display = 'none';
		input.id = 'wr-injected-file-input';
		document.body.appendChild(input);
	})()`, nil)); err == nil {
		if err := chromedp.Run(ctx,
			chromedp.SetUploadFiles(`#wr-injected-file-input`, []string{filePath}, chromedp.ByID),
		); err == nil {
			log.Printf("[PublishAuto] 上传完成（DOM 注入 input 兜底）")
			return nil
		}
	}
	// ④ 再试一轮现存 input（点击副作用的兜底）
	if ok, _ := evalBool(ctx, `document.querySelectorAll('input[type=file], input[accept*="video"], input[accept*="mp4"]').length > 0`); ok {
		if err := setInputFilesByQuery(ctx, inputQuery, filePath); err == nil {
			log.Printf("[PublishAuto] 上传完成（二次探测直传）")
			return nil
		}
	}
	return fmt.Errorf("上传文件失败（四级降级均失败）")
}

// setInputFilesByQuery 对首个匹配 input 注入文件（trusted 事件）。
func setInputFilesByQuery(ctx context.Context, query, filePath string) error {
	return chromedp.Run(ctx, chromedp.SetUploadFiles(query, []string{filePath}, chromedp.ByQuery))
}

// uploadViaFileChooser CDP filechooser 拦截上传（panda Playwright filechooser 等价）：
// 启用拦截 → 点击平台上传区 → EventFileChooser 事件（backendNodeId）→ 注入文件。
func uploadViaFileChooser(ctx context.Context, filePath string) bool {
	done := make(chan bool, 1)
	var interceptOnce sync.Once
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if fc, ok := ev.(*page.EventFileChooserOpened); ok && fc.BackendNodeID != 0 {
			go func() {
				err := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
					return dom.SetFileInputFiles([]string{filePath}).WithBackendNodeID(fc.BackendNodeID).Do(c)
				}))
				done <- (err == nil)
			}()
		}
	})
	// 启用 filechooser 拦截
	if err := chromedp.Run(ctx, page.SetInterceptFileChooserDialog(true)); err != nil {
		return false
	}
	defer interceptOnce.Do(func() {
		_ = chromedp.Run(ctx, page.SetInterceptFileChooserDialog(false))
	})
	// 点击平台上传区（panda 候选链含 upload-wrapper——2026-08-28 快手 DRY_RUN 实测补全）
	clicked := false
	// class 候选（点击前 scrollIntoView——2026-08-28 快手实测上传区可能在视口外，
	// CDP 坐标点击对视口外元素无效）
	for _, sel := range []string{
		`[class*="upload-area"]`, `[class*="upload-zone"]`, `[class*="UploadArea"]`,
		`[class*="drop-zone"]`, `div[class*="upload"]`, `[class*="upload-wrapper"]`,
		`[class*="video-upload"]`, `[class*="upload-trigger"]`,
	} {
		if visible, _ := evalBool(ctx, selVisibleJS(sel)); visible {
			_ = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`document.querySelector(%q).scrollIntoView({block:'center'})`, sel), nil))
			chromedp.Sleep(500 * time.Millisecond)
			if err := chromedp.Run(ctx, chromedp.Click(sel, chromedp.ByQuery)); err == nil {
				clicked = true
				log.Printf("[PublishAuto] 已点击上传区（%s），等待 filechooser…", sel)
			}
			break
		}
	}
	// 文本型上传区兜底（快手实测形态："点击或拖拽上传"文案的虚线框）
	if !clicked {
		for _, text := range []string{"点击或拖拽上传", "拖拽上传", "点击上传", "上传视频"} {
			if done, _ := evalBool(ctx, fmt.Sprintf(`(() => {
				const els = [...document.querySelectorAll('div, span, button')].filter(e =>
					e.children.length === 0 && (e.textContent || '').trim().includes(%q) && e.offsetParent !== null);
				if (!els.length) return false;
				els[0].closest('div[class], section, [class]')?.scrollIntoView({block: 'center'});
				return true;
			})()`, text)); done {
				chromedp.Sleep(500 * time.Millisecond)
				if ok, _ := evalBool(ctx, fmt.Sprintf(`(() => {
					const els = [...document.querySelectorAll('div, span, button')].filter(e =>
						e.children.length === 0 && (e.textContent || '').trim().includes(%q) && e.offsetParent !== null);
					if (!els.length) return false;
					els[0].click();
					// 父容器也点一次（文案叶子可能是纯文本节点）
					els[0].parentElement?.click();
					return true;
				})()`, text)); ok {
					clicked = true
					log.Printf("[PublishAuto] 已点击文本上传区（%s），等待 filechooser…", text)
					break
				}
			}
		}
	}
	if !clicked {
		return false
	}
	select {
	case ok := <-done:
		if ok {
			log.Printf("[PublishAuto] 上传完成（filechooser 拦截）")
		}
		return ok
	case <-time.After(15 * time.Second):
		log.Printf("[PublishAuto] filechooser 超时未触发")
		return false
	}
}

// ---- 平台必选项：声明/弹窗（panda 实测流程原语）----

// selectDeclarationOption 通用"声明选择器"流程（抖音自主声明 / B站创作声明同构）：
// 触发文本 → 弹层内 radio/选项（按可见文本匹配）→ 确定。
// 任一步找不到都静默返回 false（声明项随平台版本时有时无——panda 同样按可选处理）。
func selectDeclarationOption(ctx context.Context, triggerText, optionText, confirmText string) bool {
	visible, _ := evalBool(ctx, textVisibleJS(triggerText))
	if !visible {
		return false
	}
	// 点击触发文本（滚动到可视区）
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const els = [...document.querySelectorAll('*')].filter(e =>
			e.children.length === 0 && (e.textContent || '').trim().includes(%q));
		if (els.length) { els[0].scrollIntoView({block: 'center'}); els[0].click(); return true; }
		return false;
	})()`, triggerText), nil)); err != nil {
		return false
	}
	chromedp.Sleep(1 * time.Second)
	// 选 radio/选项（含关联 label；force 点击——panda 实测 radio 需要 force）
	if ok, _ := evalBool(ctx, fmt.Sprintf(`(() => {
		const targets = [...document.querySelectorAll('[role=radio], input[type=radio], label, li, div')].filter(e => {
			const t = (e.textContent || '').trim();
			return t.includes(%q) && t.length < 40 && e.offsetParent !== null;
		});
		if (!targets.length) return false;
		let target = targets[0];
		for (let n = target; n && n !== document.body; n = n.parentElement) {
			if (n.tagName === 'LABEL' || n.getAttribute('role') === 'radio' ||
				(n.querySelector && n.querySelector('input[type=radio]'))) { target = n; break; }
		}
		target.scrollIntoView({block: 'center'});
		target.click();
		const input = target.querySelector('input[type=radio]');
		if (input) input.click();
		return true;
	})()`, optionText)); !ok {
		return false
	}
	chromedp.Sleep(time.Second)
	// 选中态校验（2026-08-28 实测教训：合成 click 可能被受控 radio 忽略——必须验证）
	if ok, _ := evalBool(ctx, fmt.Sprintf(`(() => {
		const c = document.querySelector('[role=radio][aria-checked="true"], input[type=radio]:checked');
		if (!c) return false;
		const scope = c.closest('label, div, li') || c.parentElement;
		return (scope ? scope.textContent : '').includes(%q);
	})()`, optionText)); !ok {
		log.Printf("[PublishAuto] ⚠️ 声明选中态校验未过（%s）——截图将如实呈现", optionText)
		return false
	}
	// 确定
	_ = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const btns = [...document.querySelectorAll('button, [role=button]')].filter(b =>
			(b.textContent || '').trim() === %q && b.offsetParent !== null);
		if (btns.length) { btns[btns.length-1].click(); return true; }
		return false;
	})()`, confirmText), nil))
	chromedp.Sleep(800 * time.Millisecond)
	log.Printf("[PublishAuto] 声明已选择：%s → %s", triggerText, optionText)
	return true
}

// selectDouyinDeclarationDialog 抖音声明弹窗处理（上传成功后自动弹出）。
//
// 2026-08-28 DRY_RUN 实测教训：旧实现点击后不校验选中态——JS click 打在文本叶子上
// 抖音受控 radio 不响应，日志假成功（截图显示 6 选项全空）。修复为三段校验闭环：
//  ① 轮询等弹窗出现（上传后异步弹出，最长 30s）
//  ② 点击选项：向上找 label/radio 容器行点击 + 原生 input 兜底
//  ③ 校验选中态（radio:checked / aria-checked=true 且含目标文本），未中重试
//  ④ 点「完成」
//  ⑤ 校验弹窗关闭（选项文本不可见）
// 任一段失败返回 false 并打日志——DRY_RUN 截图如实呈现，绝不假成功。
func selectDouyinDeclarationDialog(ctx context.Context, optionText string) bool {
	// ① 轮询等弹窗
	visible := false
	for i := 0; i < 15; i++ {
		if v, _ := evalBool(ctx, textVisibleJS(optionText)); v {
			visible = true
			break
		}
		chromedp.Sleep(2 * time.Second)
	}
	if !visible {
		return false
	}
	// ② 点击选项行（label/radio 容器优先，原生 input 兜底）
	_ = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const els = [...document.querySelectorAll('label, [role=radio], li, div, span')].filter(e =>
			(e.textContent || '').trim().includes(%q) && (e.textContent || '').trim().length < 30 && e.offsetParent !== null);
		if (!els.length) return false;
		let target = els[els.length - 1];
		for (let n = target; n && n !== document.body; n = n.parentElement) {
			if (n.tagName === 'LABEL' || n.getAttribute('role') === 'radio' ||
				(n.querySelector && n.querySelector('input[type=radio]'))) { target = n; break; }
		}
		target.scrollIntoView({block: 'center'});
		target.click();
		const input = target.querySelector('input[type=radio]');
		if (input) input.click();
		return true;
	})()`, optionText), nil))
	chromedp.Sleep(time.Second)
	// ③ 选中态校验（含目标文本的 radio 处于 checked）
	checked := func() bool {
		ok, _ := evalBool(ctx, fmt.Sprintf(`(() => {
			const c = document.querySelector('[role=radio][aria-checked="true"], input[type=radio]:checked');
			if (!c) return false;
			const scope = c.closest('label, div, li') || c.parentElement;
			return (scope ? scope.textContent : '').includes(%q);
		})()`, optionText))
		return ok
	}
	if !checked() {
		log.Printf("[PublishAuto] 声明选中态校验未过，重试点击 radio 本体")
		_ = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
			const els = [...document.querySelectorAll('[role=radio], label, [class*="radio"]')].filter(e =>
				(e.textContent || '').includes(%q) && e.offsetParent !== null);
			if (els.length) { els[0].click(); return true; }
			return false;
		})()`, optionText), nil))
		chromedp.Sleep(time.Second)
	}
	if !checked() {
		log.Printf("[PublishAuto] ⚠️ 声明选项未选中（radio 不响应合成点击）——截图将如实呈现")
		return false
	}
	// ④ 点「完成」
	clickByText(ctx, "完成")
	chromedp.Sleep(1500 * time.Millisecond)
	// ⑤ 弹窗关闭校验
	if v, _ := evalBool(ctx, textVisibleJS(optionText)); !v {
		log.Printf("[PublishAuto] 声明已选择并确认（%s），弹窗已关闭", optionText)
		return true
	}
	log.Printf("[PublishAuto] ⚠️ 声明弹窗未关闭——截图将如实呈现")
	return false
}

// selectBiliDeclaration B站创作声明（2026-08-28 诊断程序实锤：声明区渲染在
// micro-app[name=video-up] 的内嵌 iframe——主文档 querySelector 永远找不到，
// 这是此前轮询 16s 仍失败的根因）。
//
// 穿透方案：allDocs() 递归收集所有可访问文档（主文档 + micro-app shadowRoot +
// 任意深度同域 iframe），查找/点击/校验三段都在全文档空间执行。
func selectBiliDeclaration(ctx context.Context, optionText string) bool {
	// ① 全文档空间找触发器（bcc-select input）并点击（声明区上传视频后渲染）
	clicked := false
	for i := 0; i < 8 && !clicked; i++ {
		// 轮询等声明区渲染（上传后条件渲染，异步）
		clicked, _ = evalBool(ctx, `(() => {
			const docs = [document];
			const walk = (doc) => {
				for (const f of doc.querySelectorAll('iframe')) {
					try { if (f.contentDocument) { docs.push(f.contentDocument); walk(f.contentDocument); } } catch (e) {}
				}
			};
			walk(document);
			for (const m of document.querySelectorAll('micro-app')) { if (m.shadowRoot) docs.push(m.shadowRoot); }
			for (const doc of docs) {
				const input = doc.querySelector(
					'.creation-statement-container input.bcc-select-input-inner, input[placeholder*="创作声明"]');
				if (input) {
					input.scrollIntoView({block: 'center'});
					(input.closest('.bcc-select') || input).click();
					input.click();
					return true;
				}
			}
			return false;
		})()`)
		if !clicked {
			chromedp.Sleep(2 * time.Second)
		}
	}
	if !clicked {
		log.Printf("[PublishAuto] ⚠️ B站创作声明触发器未找到（全文档穿透轮询 16s）")
		return false
	}
	chromedp.Sleep(1 * time.Second)
	// ② 全文档空间展开选项中点目标
	if ok, _ := evalBool(ctx, fmt.Sprintf(`(() => {
		const docs = [document];
		const walk = (doc) => {
			for (const f of doc.querySelectorAll('iframe')) {
				try { if (f.contentDocument) { docs.push(f.contentDocument); walk(f.contentDocument); } } catch (e) {}
			}
		};
		walk(document);
		for (const m of document.querySelectorAll('micro-app')) { if (m.shadowRoot) docs.push(m.shadowRoot); }
		for (const doc of docs) {
			const opts = [...doc.querySelectorAll('.bcc-select-option-list .bcc-option, li.bcc-option')].filter(e =>
				(e.textContent || '').trim().includes(%q));
			if (opts.length) { opts[0].scrollIntoView({block: 'center'}); opts[0].click(); return true; }
		}
		return false;
	})()`, optionText)); !ok {
		log.Printf("[PublishAuto] ⚠️ B站声明选项未找到（%s）——下拉可能未展开", optionText)
		return false
	}
	chromedp.Sleep(1 * time.Second)
	// ③ 全文档空间选中态校验（input.value 显示所选文本）
	if ok, _ := evalBool(ctx, fmt.Sprintf(`(() => {
		const docs = [document];
		const walk = (doc) => {
			for (const f of doc.querySelectorAll('iframe')) {
				try { if (f.contentDocument) { docs.push(f.contentDocument); walk(f.contentDocument); } } catch (e) {}
			}
		};
		walk(document);
		for (const m of document.querySelectorAll('micro-app')) { if (m.shadowRoot) docs.push(m.shadowRoot); }
		for (const doc of docs) {
			const input = doc.querySelector('input[placeholder*="创作声明"]');
			if (input && (input.value || '').includes(%q)) return true;
		}
		return false;
	})()`, optionText)); ok {
		log.Printf("[PublishAuto] B站创作声明已选并校验（%s，全文档穿透命中）", optionText)
		return true
	}
	log.Printf("[PublishAuto] ⚠️ B站声明选中态校验未过——截图将如实呈现")
	return false
}

// dismissSurveyDialog 关闭满意度调查类弹窗（快手实测：右下角"你对PC版创作者服务
// 平台是否满意"遮挡表单交互）。找 dialog 容器内外的关闭按钮（×）点击。
func dismissSurveyDialog(ctx context.Context, featureText string) {
	visible, _ := evalBool(ctx, textVisibleJS(featureText))
	if !visible {
		return
	}
	done, _ := evalBool(ctx, `(() => {
		const dialog = document.querySelector('[role=dialog], [class*="modal"], [class*="dialog"], [class*="survey"], [class*="Survey"]');
		const root = dialog || document;
		const closes = root.querySelectorAll('[class*="close"], [class*="Close"], [aria-label*="关闭"], [aria-label*="close"]');
		if (closes.length) { closes[closes.length-1].click(); return true; }
		return false;
	})()`)
	if done {
		chromedp.Sleep(800 * time.Millisecond)
		log.Printf("[PublishAuto] 调查弹窗已关闭：%s", featureText)
	}
}

// ---- 话题独立填写 + 封面上传（panda 实测：job.Tags/CoverURL 消费）----

// douyinTagSels 抖音话题输入框候选（panda Douyin spec L321-330）。
var douyinTagSels = []string{
	`input[placeholder*="话题"]`, `input[placeholder*="标签"]`, `input[placeholder*="tag"]`,
	`input[placeholder*="hashtag"]`, `[class*="tag-input"] input`, `[class*="TagInput"] input`,
	`[class*="topic-input"] input`, `[class*="hashtag"] input`,
}

// fillHashtags 话题填写（panda 模式）：tag 统一 # 前缀、空格连接，填进话题输入候选链。
func fillHashtags(ctx context.Context, tags []string, sels ...string) bool {
	if len(tags) == 0 {
		return false
	}
	var parts []string
	for _, t := range tags {
		t = strings.TrimPrefix(strings.TrimSpace(t), "#")
		if t != "" {
			parts = append(parts, "#"+t)
		}
	}
	if len(parts) == 0 {
		return false
	}
	return fillFirstEditable(ctx, strings.Join(parts, " "), sels...)
}

// hashtagText tags → "#tag1 #tag2" 话题文本（去重 # 前缀、空格连接）。
func hashtagText(tags []string) string {
	var parts []string
	for _, t := range tags {
		t = strings.TrimPrefix(strings.TrimSpace(t), "#")
		if t != "" {
			parts = append(parts, "#"+t)
		}
	}
	return strings.Join(parts, " ")
}

// uploadCoverImage 封面上传（panda 抖音封面候选模式）：
// 图片 input 直传（隐藏元素 presence 判定）→ 点"添加封面/上传封面"按钮后重试。
// B站的"封面制作"弹窗交互由调用方在上传后追加。
func uploadCoverImage(ctx context.Context, coverPath string) bool {
	if coverPath == "" {
		return false
	}
	// ① 图片 input 直传
	if ok, _ := evalBool(ctx, `document.querySelectorAll('input[type=file][accept*="image"]').length > 0`); ok {
		if err := chromedp.Run(ctx,
			chromedp.SetUploadFiles(`input[type=file][accept*="image"]`, []string{coverPath}, chromedp.ByQuery),
		); err == nil {
			log.Printf("[PublishAuto] 封面已上传（图片 input 直传）")
			return true
		}
	}
	// ② 点封面按钮（触发 input 出现后重试）
	for _, text := range []string{"添加封面", "上传封面", "选择封面", "Upload cover"} {
		clicked, _ := evalBool(ctx, fmt.Sprintf(`(() => {
			const els = [...document.querySelectorAll('button, [role=button], a, span, div')].filter(e =>
				e.children.length === 0 && (e.textContent || '').trim().includes(%q) && e.offsetParent !== null);
			if (els.length) { els[0].click(); return true; }
			return false;
		})()`, text))
		if clicked {
			chromedp.Sleep(2 * time.Second)
			if ok, _ := evalBool(ctx, `document.querySelectorAll('input[type=file][accept*="image"]').length > 0`); ok {
				if err := chromedp.Run(ctx,
					chromedp.SetUploadFiles(`input[type=file][accept*="image"]`, []string{coverPath}, chromedp.ByQuery),
				); err == nil {
					log.Printf("[PublishAuto] 封面已上传（点击「%s」后直传）", text)
					return true
				}
			}
		}
	}
	log.Printf("[PublishAuto] 封面上传未命中候选（页面无接收控件——跳过，用平台自动截帧）")
	return false
}

// dismissBannerDialog 关闭带特征文本的公告弹窗（抖音"共创中心"等）：
// banner 特征文本可见时，找 dialog 容器内的确认按钮点掉。
func dismissBannerDialog(ctx context.Context, bannerText, buttonText string) {
	visible, _ := evalBool(ctx, textVisibleJS(bannerText))
	if !visible {
		return
	}
	_ = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const dialog = document.querySelector('[role=dialog], .semi-modal, [class*="modal"], [class*="dialog"]');
		const root = dialog || document;
		const btns = [...root.querySelectorAll('button, [role=button]')].filter(b =>
			(b.textContent || '').trim().includes(%q) && b.offsetParent !== null);
		if (btns.length) { btns[0].click(); return true; }
		return false;
	})()`, buttonText), nil))
	chromedp.Sleep(800 * time.Millisecond)
	log.Printf("[PublishAuto] 公告弹窗已关闭：%s", bannerText)
}

// waitForProcessingDone 处理中信号检测（panda 实测：上传后平台显示
// "处理中/上传中/转码中"时应额外等待，过早填表会被重置）。
func waitForProcessingDone(ctx context.Context, extraWait time.Duration) {
	for _, text := range []string{"处理中", "上传中", "转码中"} {
		if visible, _ := evalBool(ctx, textVisibleJS(text)); visible {
			log.Printf("[PublishAuto] 检测到「%s」，额外等待 %s", text, extraWait)
			chromedp.Sleep(extraWait)
			return
		}
	}
}

// fillFirstEditable 在候选链中找首个可交互的输入控件并填入文本。
// contenteditable 用 JS 设 textContent + 派发 input 事件（panda 快手/微信实测：
// fill() 对 contenteditable 常失败，必须手动派发事件让框架识别）。
func fillFirstEditable(ctx context.Context, text string, selectors ...string) bool {
	for _, sel := range selectors {
		ok, _ := evalBool(ctx, fmt.Sprintf(`(() => {
			const el = document.querySelector(%q);
			if (!el || el.offsetParent === null) return false;
			el.focus();
			if (el.isContentEditable) {
				el.textContent = %q;
			} else {
				el.value = %q;
			}
			el.dispatchEvent(new Event('input', {bubbles: true}));
			el.dispatchEvent(new Event('change', {bubbles: true}));
			return true;
		})()`, sel, text, text))
		if ok {
			log.Printf("[PublishAuto] 已填充（选择器=%s）", sel)
			return true
		}
	}
	return false
}

// scrollToBottom 滚到底部（发布按钮在页面底部的平台：抖音/B站/YouTube）。
func scrollToBottom(ctx context.Context) {
	_ = chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil))
	chromedp.Sleep(800 * time.Millisecond)
}

// ---- JS 探测原语 ----

func evalBool(ctx context.Context, js string) (bool, error) {
	var out bool
	err := chromedp.Run(ctx, chromedp.Evaluate(js, &out))
	return out, err
}

// selVisibleJS 元素存在且可见的探测 JS。
func selVisibleJS(sel string) string {
	return fmt.Sprintf(`(() => { const el = document.querySelector(%q); return !!el && el.offsetParent !== null; })()`, sel)
}

// textVisibleJS 精确文本可见的探测 JS（叶子元素匹配，避免容器误报）。
func textVisibleJS(text string) string {
	return fmt.Sprintf(`(() => {
		return [...document.querySelectorAll('*')].some(e =>
			e.children.length === 0 && (e.textContent || '').trim().includes(%q) && e.offsetParent !== null);
	})()`, text)
}

// panda 平台候选选择器（真机实测链——顺序即优先级；B站三项 2026-08-28 DOM 快照实证）。
var (
	// 抖音标题（panda Douyin/upload-video.spec.ts L237-248）
	douyinTitleSels = []string{
		`input[placeholder*="作品标题"]`, `input[placeholder*="标题"]`, `input[placeholder*="title"]`,
		`textarea[placeholder*="作品标题"]`, `textarea[placeholder*="标题"]`, `input[name="title"]`,
		`[class*="title-input"] input`, `[class*="TitleInput"] input`,
		`[class*="title"] input[type="text"]`, `[class*="Title"] input`, `input[maxlength]`,
	}
	// 抖音描述（L280-291）
	douyinDescSels = []string{
		`textarea[placeholder*="作品描述"]`, `textarea[placeholder*="描述"]`, `textarea[placeholder*="简介"]`,
		`textarea[placeholder*="description"]`, `textarea[name="desc"]`, `textarea[name="description"]`,
		`[class*="desc-input"] textarea`, `[class*="DescInput"] textarea`,
		`[class*="desc"] textarea`, `[class*="Desc"] textarea`, `textarea[maxlength]`,
	}
	// B站标题（panda Bilibili/upload-video.spec.ts L177-185）
	biliTitleSels = []string{
		`input[placeholder*="标题"]`, `input[placeholder*="标题（必填）"]`, `.input-val`,
		`textarea[placeholder*="标题"]`, `input[name="title"]`,
		`[class*="title-input"] input`, `[class*="TitleInput"] input`, `input[maxlength]`,
	}
	// B站简介（2026-08-28 DOM 快照实证：新版是 contenteditable + data-placeholder
	//「填写更全面的相关信息…」，非 textarea——panda 的 textarea 候选系全部作废，前排实证选择器）
	biliDescSels = []string{
		`[contenteditable=true][data-placeholder*="相关信息"]`,
		`[data-placeholder*="让更多的人能找到"]`,
		`textarea[placeholder*="简介"]`, `textarea[placeholder*="描述"]`,
		`.desc-input textarea`, `[class*="desc"] textarea`,
	}
	// B站标签输入（DOM 实证：input.input-val placeholder="按回车键Enter创建标签"——
	// 回车确认模式；panda 的逗号 fill 是旧版行为）
	biliTagSels = []string{
		`input.input-val[placeholder*="创建标签"]`, `input[placeholder*="按回车"]`,
		`input[placeholder*="标签"]`, `.tag-input input`, `[class*="tag"] input`,
	}
	// 微信标题（contenteditable 系）
	weixinTitleSels = []string{
		`.input-editor[contenteditable=true]`, `[contenteditable=true].input-editor`,
		`[class*="title"][contenteditable]`, `[contenteditable=true]`,
	}
	// 快手标题+描述合并目标（contenteditable div）
	kuaishouContentSels = []string{
		`#work-description-edit`, `[contenteditable=true]`, `.ql-editor`,
	}
)

// ---- DRY_RUN 发布按钮就绪探测（完成标准：定位到可点的发布按钮，不点击）----

// probePublishButton 按平台特征探测发布按钮是否就绪（存在 + 可见 + 未禁用）。
// DRY_RUN 的最后一步：找到即全链路成功（截图留证），找不到如实报失败。
// 注意：只探测绝不点击——真发风险为零。
func probePublishButton(ctx context.Context, platform string) (bool, string) {
	// 各平台发布按钮特征（文本精确/包含 + 特征选择器；B站在 micro-app iframe 内）
	type spec struct {
		texts []string
		sels  []string
		excl  string // 排除文本
	}
	// 2026-08-29 实测精确文案（包含匹配会命中"发布设置"等侧栏标题——收紧为 ===）
	specs := map[string]spec{
		"douyin":      {texts: []string{"作品发布", "发布"}},
		"kuaishou":    {texts: []string{"发布作品", "发布"}, excl: "存草稿"},
		"bilibili":    {texts: []string{"立即投稿", "投稿"}, sels: []string{"span.submit-add", ".submit-add"}, excl: "草稿"},
		"zhihu":       {texts: []string{"发布", "确认发布"}, excl: "设置"},
		"weixin":      {texts: []string{"发表", "立即发布", "发布", "确认发布", "提交"}, excl: "取消"},
		"youtube":     {texts: []string{"Publish", "发布"}},
		"xiaohongshu": {texts: []string{"发布", "发送"}, excl: "存草稿"},
	}
	sp, ok := specs[platform]
	if !ok {
		sp = spec{texts: []string{"发布", "Publish"}}
	}

	// 全文档穿透（B站 micro-app iframe；其他平台主文档即命中）
	detail := ""
	found, _ := evalBool(ctx, fmt.Sprintf(`(() => {
		const docs = [document];
		const walk = (doc) => { for (const f of doc.querySelectorAll('iframe')) {
			try { if (f.contentDocument) { docs.push(f.contentDocument); walk(f.contentDocument); } } catch (e) {} } };
		walk(document);
		for (const m of document.querySelectorAll('micro-app')) { if (m.shadowRoot) docs.push(m.shadowRoot); }
		const sels = %s;
		const texts = %s;
		const excl = %q;
		for (const doc of docs) {
			// ① 特征选择器优先（B站 span.submit-add）
			for (const sel of sels) {
				const el = doc.querySelector(sel);
				if (el && el.offsetParent !== null) {
					const btn = el.closest('button, [role=button]') || el;
					const dis = el.closest('[disabled], [class*="disabled"], [aria-disabled="true"]');
					window.__wrBtnDetail = '[特征选择器 ' + sel + '] ' + (el.textContent || '').trim().slice(0, 12) + (dis ? '（疑似禁用）' : '（可点）');
					return !dis;
				}
			}
			// ② 文本匹配（排除顶栏/导航区同名按钮——抖音顶栏全局"发布"入口会抢先命中）
			const inNav = (el) => !!el.closest('header, nav, [class*="header"], [class*="nav"], [class*="sidebar"], [class*="menu"]');
			const leaves = [...doc.querySelectorAll('button, [role=button], span, div')].filter(b => {
				const t = (b.textContent || '').trim();
				if (!t || t.length > 12 || b.children.length > 0) return false;
				if (excl && t.includes(excl)) return false;
				if (inNav(b)) return false;
				return texts.some(x => t === x);
			});
			for (const leaf of leaves) {
				if (leaf.offsetParent === null) continue;
				const btn = leaf.closest('button, [role=button], [class*="submit"], [class*="publish"]') || leaf;
				const dis = btn.disabled || btn.getAttribute('aria-disabled') === 'true' ||
					String(btn.className).includes('disabled');
				window.__wrBtnDetail = '[文本 ' + (leaf.textContent || '').trim() + '] ' + (dis ? '（禁用）' : '（可点）');
				if (!dis) return true;
			}
		}
		window.__wrBtnDetail = '未找到任何候选按钮';
		return false;
	})()`, jsStrArr(sp.sels), jsStrArr(sp.texts), sp.excl))
	if v, _ := evalBool(ctx, `!!window.__wrBtnDetail`); v {
		_ = chromedp.Run(ctx, chromedp.Evaluate(`window.__wrBtnDetail`, &detail))
	}
	return found, detail
}

// jsStrArr []string → JS 数组字面量。
func jsStrArr(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	parts := make([]string, len(ss))
	for i, s := range ss {
		b, _ := json.Marshal(s)
		parts[i] = string(b)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
