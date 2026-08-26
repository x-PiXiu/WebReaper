/**
 * 统一生成接口错误友好化（对照 Docs/Plans/16 BE-GEN-*）。
 * 拦截器与页面 catch 共用——把后端原始 msg 译成可操作的提示。
 */
export function friendlyGenerationError(raw: string | undefined | null): string {
  const msg = (raw || '').trim()
  if (!msg) return '生成失败，请稍后重试'

  if (/params_json|Data too long|1406/i.test(msg)) {
    return '素材过大，任务无法保存。请压缩图片/视频后再试（建议单文件小于 2MB）'
  }
  if (/viduq1.*参考|参考生图需|需 1-7 张/i.test(msg)) {
    return '当前默认模型需要参考图。请上传 1–7 张参考图，或在高级生成中选择 viduq2 纯文生图'
  }
  if (/已停用|未启用|disabled/i.test(msg)) {
    return '该生成能力未启用。请联系管理员在后台开启对应模式，或先上传参考图改用图生视频'
  }
  if (/详情解析失败|unexpected end of JSON|XHR 执行失败|详情无播放地址/i.test(msg)) {
    return '抖音详情拉取失败（常见原因：爬虫账号未登录/Cookie 过期，或平台风控）。请改用「上传视频」提取文案，或请管理员在后台检查抖音爬虫账号'
  }
  if (/分享链解析失败|找不到视频|暂不支持该链接/i.test(msg)) {
    return '分享链接无法解析。请确认含 https://v.douyin.com/… 的完整口令，或改用上传视频'
  }
  if (/超出本地内联上限|8MB/i.test(msg)) {
    return '素材超过 8MB 本地内联上限，请压缩后上传或使用更短的片段'
  }
  if (/积分不足|credits/i.test(msg)) {
    return '生成积分不足，请稍后重试或联系管理员'
  }

  return msg
}

/** 本地联调：大文件易触发 BE params_json 超长；上传前轻量提示 */
export const WARN_MATERIAL_BYTES = 2 * 1024 * 1024
export const BLOCK_MATERIAL_BYTES = 8 * 1024 * 1024

export function checkMaterialFileSize(file: File): { ok: boolean; warning?: string; error?: string } {
  if (file.size > BLOCK_MATERIAL_BYTES) {
    return {
      ok: false,
      error: `文件约 ${(file.size / 1024 / 1024).toFixed(1)}MB，超过本地生成上限（8MB）。请压缩后再上传`,
    }
  }
  if (file.size > WARN_MATERIAL_BYTES) {
    return {
      ok: true,
      warning: `文件约 ${(file.size / 1024 / 1024).toFixed(1)}MB，本地联调可能失败。建议压缩到 2MB 以内`,
    }
  }
  return { ok: true }
}
