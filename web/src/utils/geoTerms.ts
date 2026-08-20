// 术语翻译层（傻瓜化改造核心）：全站统一的"GEO 黑话 → 商户语言"映射。
//
// 设计动机：目标用户是餐馆/美业/教培老板，不是 SEO 专家。
// "意图分类""低置信度""可引用结构正交组合"这类工程术语对他们和天书无异。
// 此前每个页面各自发明解释文案——口径漂移且无法集中治理。
// 所有面向商户的文案一律从这里取词；GEO 专业词（提及率等核心指标）保留，
// 但搭配人话副标题，不让用户靠猜。

/** 关键词意图 → 商户语言（词库表 / 筛选器 / 添加弹窗共用） */
export function intentLabel(intent?: string): string {
  const map: Record<string, string> = {
    informational: '想了解的',
    transactional: '准备下单的',
    local: '找附近的',
  }
  return (intent && map[intent]) || intent || '—'
}

/** 问题词意图 → 商户语言（提问词挖掘结果标签——与词库意图统一口径） */
export function questionIntentLabel(intent?: string): string {
  const map: Record<string, string> = {
    informational: '想了解的',
    comparative: '在做比较的',
    recommendational: '求推荐的',
  }
  return (intent && map[intent]) || intent || ''
}

/** 采样不足的置信度提示 → "数据积累中"（不向商户暴露统计学术语） */
export function lowSampleHint(sampleCount?: number): string {
  const n = sampleCount || 0
  return `数据积累中：已采样 ${n} 次，结果可能波动，建议过几天再复测`
}

/** 提及率一句话解释（tooltip 常用） */
export const MENTION_RATE_TIP =
  'AI 推荐度 = 问 AI 的次数里，它提到你的比例。比如问 10 次"装修公司哪家好"，AI 提到你 3 次 = 30%'

/** 收录一句话解释 */
export const INDEXED_TIP =
  '收录 = 这篇内容已经进入搜索引擎的数据库，AI 才能搜到并引用它（一般需要 1-2 周）'

/** 引擎显示名（engine_name → 商户友好的平台名，全站统一口径） */
const ENGINE_LABEL_MAP: Record<string, string> = {
  default: '默认引擎',
  chatgpt: 'ChatGPT',
  kimi: 'Kimi',
  doubao: '豆包',
  deepseek: 'DeepSeek',
  qwen: '通义千问',
  ernie: '文心一言',
  yuanbao: '腾讯元宝',
  perplexity: 'Perplexity',
}

export function engineLabel(name?: string): string {
  const key = (name || 'default').toLowerCase()
  return ENGINE_LABEL_MAP[key] || name || '默认引擎'
}

/** 相对时间（"3 天前"——历史列表/摘要通用） */
export function timeAgo(iso?: string): string {
  if (!iso) return ''
  const s = (Date.now() - new Date(iso).getTime()) / 1000
  if (s < 60) return '刚刚'
  if (s < 3600) return `${Math.floor(s / 60)} 分钟前`
  if (s < 86400) return `${Math.floor(s / 3600)} 小时前`
  return `${Math.floor(s / 86400)} 天前`
}
