// Package main 是内嵌的招聘网站示例服务。
//
// 它提供一个结构化的招聘列表页 HTML，让 colly 爬虫能真实发起 HTTP 请求、
// 真实解析 DOM、真实提取字段。HTML 结构与真实招聘网站一致。
//
// 用途：
//   1. 验证 generic-web 爬虫的真实抓取能力（colly 真实 HTTP + CSS 选择器）
//   2. 提供零法律风险的采集目标（不爬真实网站）
//   3. 前端"示例招聘站"一键采集体验
//
// 启动：go run ./cmd/mocksite（默认端口 8088）
package main

import (
	"fmt"
	"net/http"
)

// jobsPageHTML 是模拟的招聘列表页，结构与真实招聘网站一致。
// CSS class 命名与 generic-web 爬虫的默认选择器配置对应。
const jobsPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="UTF-8"><title>招聘列表 - 示例招聘站</title></head>
<body>
<h1>招聘职位列表</h1>
<div class="job-list">

  <div class="job-item">
    <h3 class="position">Go 后端工程师</h3>
    <span class="company">字节跳动</span>
    <span class="salary">25-40K·15薪</span>
    <ul class="requirements">
      <li>3 年以上 Go 开发经验，熟悉 goroutine/channel 并发编程</li>
      <li>熟悉 Gin/GORM 等 Web 框架，有微服务架构经验</li>
      <li>了解 MySQL/Redis，有性能优化经验</li>
    </ul>
    <a class="detail-link" href="/job/1">查看详情</a>
  </div>

  <div class="job-item">
    <h3 class="position">高级 Java 工程师</h3>
    <span class="company">美团</span>
    <span class="salary">30-50K·16薪</span>
    <ul class="requirements">
      <li>5 年以上 Java 开发经验，扎实的基础知识</li>
      <li>熟悉 Spring Cloud 微服务生态，有分布式系统设计经验</li>
      <li>熟悉消息队列（Kafka/RocketMQ）和缓存中间件</li>
    </ul>
    <a class="detail-link" href="/job/2">查看详情</a>
  </div>

  <div class="job-item">
    <h3 class="position">前端开发工程师</h3>
    <span class="company">腾讯</span>
    <span class="salary">20-35K·14薪</span>
    <ul class="requirements">
      <li>3 年以上前端开发经验，精通 React 或 Vue</li>
      <li>熟悉 TypeScript，了解前端工程化和构建工具</li>
      <li>有可视化（Echarts/D3）经验者优先</li>
    </ul>
    <a class="detail-link" href="/job/3">查看详情</a>
  </div>

  <div class="job-item">
    <h3 class="position">Python 算法工程师</h3>
    <span class="company">阿里巴巴</span>
    <span class="salary">35-60K·16薪</span>
    <ul class="requirements">
      <li>硕士及以上学历，计算机/数学相关专业</li>
      <li>熟悉 PyTorch/TensorFlow，有 NLP 或 CV 项目经验</li>
      <li>熟悉大语言模型微调（LoRA/P-Tuning）优先</li>
    </ul>
    <a class="detail-link" href="/job/4">查看详情</a>
  </div>

  <div class="job-item">
    <h3 class="position">DevOps 工程师</h3>
    <span class="company">华为</span>
    <span class="salary">22-38K·14薪</span>
    <ul class="requirements">
      <li>3 年以上运维/DevOps 经验，熟悉 Kubernetes/Docker</li>
      <li>熟悉 CI/CD 流水线（Jenkins/GitLab CI）</li>
      <li>熟悉 Prometheus/Grafana 监控体系</li>
    </ul>
    <a class="detail-link" href="/job/5">查看详情</a>
  </div>

</div>
</body>
</html>`

func main() {
	port := ":8088"
	http.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, jobsPageHTML)
	})
	// 首页重定向到 /jobs
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/jobs", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	fmt.Printf("示例招聘站已启动：http://localhost%s/jobs\n", port)
	fmt.Println("采集目标 URL: http://localhost:8088/jobs")
	fmt.Println("CSS 选择器: .job-item / .position / .company / .requirements li / .salary")
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Printf("启动失败: %v\n", err)
	}
}
