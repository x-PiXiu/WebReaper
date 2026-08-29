/** 文案工作室 — 推荐模板（前端示意，点击填入编辑器） */
export type CopyTemplate = {
  id: string
  title: string
  desc: string
  tag: string
  track: 'video' | 'graphic' | 'shared'
  body: string
}

export const COPY_TEMPLATES: CopyTemplate[] = [
  {
    id: 'tpl-oral-hook',
    title: '口播开场钩子',
    desc: '3 秒抓住注意力，适合短视频开头',
    tag: '口播',
    track: 'video',
    body: '你有没有发现，身边很多人做了同样的事，结果却差很多？\n今天我把踩过的坑和真正有效的方法说清楚——看完你就能马上用。\n',
  },
  {
    id: 'tpl-oral-sell',
    title: '口播卖点三段',
    desc: '痛点 → 方案 → 行动号召',
    tag: '口播',
    track: 'video',
    body: '很多人卡在「知道重要，但不知道怎么做」。\n我们的做法很简单：先把核心需求讲明白，再给一套可落地的步骤。\n如果你也想少走弯路，评论区扣「1」，我把清单发你。\n',
  },
  {
    id: 'tpl-oral-story',
    title: '门店故事口播',
    desc: '烟火气叙事，适合本地生活',
    tag: '口播',
    track: 'video',
    body: '这间店开了好几年，最让我印象深的不是装修，是每天早上的那股认真劲儿。\n今天带你看看我们怎么把一件小事做到顾客愿意再来。\n最后告诉你怎么预约 / 到店，别错过。\n',
  },
  {
    id: 'tpl-xhs-note',
    title: '种草笔记骨架',
    desc: '小红书风格：标题感 + 分段 + CTA',
    tag: '图文',
    track: 'graphic',
    body: '✨ 真的被种草到了｜亲测有用\n\n先说结论：适合想要「少踩坑、快上手」的人。\n\n✅ 亮点一：……\n✅ 亮点二：……\n✅ 亮点三：……\n\n💬 我的使用感受：……\n\n👇 想同款的，评论区扣关键词，我发你详细清单～\n',
  },
  {
    id: 'tpl-xhs-compare',
    title: '对比种草',
    desc: '前后对比，突出差异化卖点',
    tag: '图文',
    track: 'graphic',
    body: '之前我也纠结很久……\n\n❌ 旧方案：费时、效果不稳定\n✅ 现在这样：步骤更清晰，反馈更快\n\n适合人群：……\n不适合：……\n\n总结一句话：……\n',
  },
  {
    id: 'tpl-shared-faq',
    title: '答疑话术',
    desc: '高频问题整理，口播/图文通用',
    tag: '通用',
    track: 'shared',
    body: 'Q1：适不适合新手？\nA：……\n\nQ2：多久能看到效果？\nA：……\n\nQ3：怎么开始？\nA：三步——①…… ②…… ③……\n',
  },
]
