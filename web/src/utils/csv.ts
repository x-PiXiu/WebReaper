// CSV 导出工具：字段转义（v3 P2——此前直接 join(',')，含逗号/引号的关键词会破坏格式，
// = + - @ 开头的字段构成表格公式注入）。

/** CSV 单元格转义：公式前缀防护（= + - @）+ 引号包裹（含逗号/引号/换行时）。 */
export function csvCell(v: unknown): string {
  let s = v == null ? '' : String(v)
  if (/^[=+\-@\t\r]/.test(s)) {
    s = `'${s}` // 防 Excel/WPS 公式注入
  }
  if (/[",\n\r]/.test(s)) {
    s = `"${s.replace(/"/g, '""')}"`
  }
  return s
}

/** 一行字段 → CSV 行（逐格转义后拼接）。 */
export function csvRow(cells: unknown[]): string {
  return cells.map(csvCell).join(',')
}
