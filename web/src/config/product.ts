/** 产品定位：垂直 IP 营销拓客智能体（服务各行业老板） */
export const PRODUCT = {
  name: '获客智能体',
  nameEn: 'IP Growth Agent',
  tagline: '帮老板用账号 IP 持续获客',
  promise: '对标爆款 · 口播成片 · 一键分发 · 看清线索',
  audience: '各行业老板与运营负责人',
} as const

/** 老板向四步闭环 */
export const GROWTH_STAGES = [
  {
    key: 'persona',
    label: '建人设',
    desc: '定行业、卖点与数字分身素材',
    path: '/m/brands',
  },
  {
    key: 'create',
    label: '出内容',
    desc: '发视频或发图文，两条获客线',
    path: '/m/compose',
  },
  {
    key: 'publish',
    label: '发出去',
    desc: '绑定账号，一键发到平台',
    path: '/m/distribution',
  },
  {
    key: 'measure',
    label: '看线索',
    desc: '播放、互动与获客效果复盘',
    path: '/m/analytics',
  },
] as const

export type FlowStepDef = {
  key: string
  label: string
  title: string
  tip: string
  nextLabel: string
}

/** 发视频：三步引导（大厂步骤式） */
export const VIDEO_FLOW_STEPS: FlowStepDef[] = [
  {
    key: 'script',
    label: '写脚本',
    title: '写好口播文案',
    tip: '右侧实时预览成片效果；可用 AI 按主题生成或润色',
    nextLabel: '下一步 · 配素材',
  },
  {
    key: 'assets',
    label: '配素材',
    title: '配好声音与形象',
    tip: '选配音、数字人形象与封面，预览区同步更新',
    nextLabel: '下一步 · 出成片',
  },
  {
    key: 'produce',
    label: '出成片',
    title: '生成并发布短视频',
    tip: '确认预览后生成口播成片，再一键发到平台',
    nextLabel: '去发布视频',
  },
]

/** 发图文：三步引导 */
export const GRAPHIC_FLOW_STEPS: FlowStepDef[] = [
  {
    key: 'script',
    label: '写图文',
    title: '写好种草文案',
    tip: '右侧预览笔记卡片；可用 AI 生成或差异化改写',
    nextLabel: '下一步 · 配图',
  },
  {
    key: 'assets',
    label: '配图',
    title: '配好封面与配图',
    tip: '上传或 AI 生成配图，预览区按图文帖展示',
    nextLabel: '下一步 · 发布',
  },
  {
    key: 'produce',
    label: '发布',
    title: '确认并发布图文',
    tip: '核对标题、正文与配图后发到图文渠道',
    nextLabel: '去发布图文',
  },
]

/** @deprecated 兼容旧模块总览 */
export const VIDEO_TRACK_STAGES = [
  { key: 'benchmark', label: '① 找对标', tip: '拆爆款视频结构，拿出口播原文', moduleKeys: ['benchmark'] },
  { key: 'copy', label: '② 写口播稿', tip: '文案工作室（口播）+ 标题话题', moduleKeys: ['copy', 'titles'] },
  { key: 'produce', label: '③ 做成片', tip: '配音、数字人、剪辑、视频封面', moduleKeys: ['voice', 'avatar', 'edit', 'cover'] },
  { key: 'publish', label: '④ 发视频', tip: '发布到短视频平台获客', moduleKeys: ['publish-video'] },
] as const

export const GRAPHIC_TRACK_STAGES = [
  { key: 'benchmark', label: '① 找对标', tip: '拆爆款笔记/文章结构，沉淀可改写原文', moduleKeys: ['benchmark'] },
  { key: 'copy', label: '② 写图文', tip: '文案工作室（种草/长文）+ 标题话题', moduleKeys: ['copy', 'titles'] },
  { key: 'produce', label: '③ 配图面', tip: '配图与图文封面', moduleKeys: ['images', 'cover-graphic'] },
  { key: 'publish', label: '④ 发图文', tip: '发布到图文/种草平台获客', moduleKeys: ['publish-graphic'] },
] as const

export const COMPOSE_STAGE_GROUPS = VIDEO_TRACK_STAGES
