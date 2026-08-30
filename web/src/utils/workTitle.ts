/** 作品标题兜底：后端个别记录存在编码乱码，剔除不可读字符后有效内容不足则返回占位 */
export function cleanWorkTitle(title: string | undefined): string {
  if (!title) return '未命名作品'
  const cleaned = title
    .replace(/[\u0000-\u001f\u007f\ufffd]/g, '')
    .replace(/^[\s#]+/, '')
    .trim()
  // 有效字符 = 中文/字母/数字；乱码（不可读符号）占比过高时整个标题不可信
  const core = (cleaned.match(/[\u4e00-\u9fa5a-zA-Z0-9]/g) || []).length
  if (core < 2) return '未命名作品'
  if (core / cleaned.length < 0.6) return '未命名作品'
  return cleaned
}
