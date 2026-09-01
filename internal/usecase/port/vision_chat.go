package port

import "context"

// VisionChat 图片多模态对话（32号 P2 第二批：图片机审）。
// 实现：MiMo mimo-v2.5（OpenAI vision 兼容格式，2026-09-01 实测可用——
// image_url 内容准确识别二维码图）。
type VisionChat interface {
	// ChatWithImage 单轮图片问答。imageURL 支持 http(s) 公网地址与本站托管地址
	//（私网/相对路径由适配器下载转 data URL——外部厂商拉不到 localhost）。
	ChatWithImage(ctx context.Context, prompt, imageURL string) (string, error)
}
