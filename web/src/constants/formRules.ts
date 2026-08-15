// 行业建议清单（F1-4：品牌表单行业字段"下拉建议+可输入"——驱动行业看板填写率，
// 留空则全行业检索）。AutoComplete 数据源，用户可自由输入清单外行业。
export const INDUSTRY_OPTIONS = [
  // 本地生活
  '餐饮', '美业/美容美发', '家装/装修', '教育培训', '健身/运动', '酒店/民宿',
  '汽车服务', '母婴/亲子', '医疗/口腔', '零售/便利店', '娱乐/休闲',
  // 线上业务
  'SaaS/软件工具', '电商/零售', '跨境电商', '在线教育', '游戏', '自媒体/内容',
  '金融/理财', '医疗健康', '出行/旅游', 'AI/大模型', '企业服务',
]

/** 输入校验共享规则（F3-3）：拒绝乱码/控制字符 + 长度上限。 */
export const CLEAN_TEXT_VALIDATOR = {
  validator: (_: unknown, v: string) => {
    const s = (v || '').trim()
    if (/[\uFFFD\u0000-\u0008\u000B\u000C\u000E-\u001F]/.test(s)) {
      return Promise.reject(new Error('包含非法字符（乱码/控制字符），请检查输入法或粘贴内容'))
    }
    return Promise.resolve()
  },
}

/** 官网地址规则：online 必填阻断 + 填写时必须是 http(s) URL。 */
export const websiteRules = (isOnline: boolean) => [
  {
    required: isOnline,
    message: '线上品牌必填官网地址——官网是 AI 引用你的核心信源（NAP 注入内容与结构化数据）',
  },
  {
    validator: (_: unknown, v: string) => {
      const s = (v || '').trim()
      if (!s) return Promise.resolve()
      if (!/^https?:\/\/\S+\.\S+/.test(s)) {
        return Promise.reject(new Error('请输入完整网址（以 http:// 或 https:// 开头）'))
      }
      return Promise.resolve()
    },
  },
  CLEAN_TEXT_VALIDATOR,
]
