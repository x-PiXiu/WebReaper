/** 居中弹窗宽度档位（适配内容，避免侧边抽屉式窄条） */
export const MODAL_W = {
  sm: 440,
  md: 560,
  lg: 640,
  xl: 760,
  xxl: 880,
} as const

/** 表单/详情类弹窗共用：内容区可滚，footer 固定 */
export const modalBodyScroll = {
  body: {
    maxHeight: 'min(72vh, calc(100vh - 160px))',
    overflowY: 'auto' as const,
  },
}
