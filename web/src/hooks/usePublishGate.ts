// 发布到公开站的质量门槛（前端唯一的业务规则实现）。
//
// 此前该规则在 Content.tsx 写了两遍（结果面板 + 历史卡片）且按钮形态不一致——
// 典型的"业务逻辑泄漏进 UI 组件"。收敛于此，纯函数、可单测。

export interface PublishGate {
  /** 评分过低，禁止发布（需先优化） */
  blocked: boolean
  /** 评分偏低，需二次确认 */
  needConfirm: boolean
  /** 给用户看的解释文案 */
  hint: string
}

/** 评分门槛：>0 且 <30 禁发；30-50 需确认；≥50 或未评分直接发。 */
export function publishGate(score?: number): PublishGate {
  const s = score ?? 0
  if (s > 0 && s < 30) {
    return { blocked: true, needConfirm: false, hint: `GEO 评分 ${s.toFixed(0)} 过低（需 ≥30），请优化后再发布` }
  }
  if (s > 0 && s < 50) {
    return { blocked: false, needConfirm: true, hint: `评分 ${s.toFixed(0)} 偏低，确定发布？` }
  }
  return { blocked: false, needConfirm: false, hint: '' }
}
