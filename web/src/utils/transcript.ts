/** 提取结果分行：优先 raw_text_lines，否则按换行切分 raw_text */
export function transcriptLines(rawText: string, rawTextLines?: string[]): string[] {
  if (rawTextLines?.length) return rawTextLines.filter((l) => l.trim())
  return rawText.split(/\n+/).map((l) => l.trim()).filter(Boolean)
}
