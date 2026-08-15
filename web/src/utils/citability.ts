// 可引用度检测（v3 P2：对齐 GEO"可引用素材结构"四项原则的展示层启发式检查）。
// 检测三个可机读信号：结论前置 / 小标题分段 / 数据标注来源——
// 用于生成结果的即时反馈，引导用户开启"引用友好"结构开关（非精确评分，仅结构提示）。

export interface CitabilityReport {
  score: number // 0-100（三信号命中数映射）
  conclusionFirst: boolean // 首段含结论信号（推荐/最佳/综上 等）
  hasSubheadings: boolean // 有 ## / ### 小标题
  dataCited: boolean // 有数据+来源标注信号（年份报告/据…数据/来源： 等）
  hints: string[] // 未命中项的改进提示
}

/** 检测一段 markdown 文本的可引用结构（纯函数）。 */
export function citabilityOf(text: string): CitabilityReport {
  const t = (text || '').trim()
  const head = t.slice(0, 260) // 结论前置只看开头两段

  const conclusionFirst = /综上|总之|结论是|推荐|首选|最佳|最值得|答案是|建议选/.test(head)
  const hasSubheadings = /^#{2,4}\s+\S/m.test(t)
  const dataCited = /\d{4}\s*年[^。]{0,20}(报告|数据|统计|调研|白皮书)|据[^。，\n]{2,18}(报告|数据|统计|官方)|来源[：:]/.test(t)

  const hits = [conclusionFirst, hasSubheadings, dataCited].filter(Boolean).length
  const hints: string[] = []
  if (!conclusionFirst) hints.push('结论前置：开头直接给出核心答案（AI 摘录优先取结论段）')
  if (!hasSubheadings) hints.push('小标题分段：用 ##/### 组织段落（小标题即语义索引）')
  if (!dataCited) hints.push('数据标注来源：关键数字标注可验证出处（如「据 2025 年行业报告」）')

  return {
    score: Math.round((hits / 3) * 100),
    conclusionFirst,
    hasSubheadings,
    dataCited,
    hints,
  }
}
