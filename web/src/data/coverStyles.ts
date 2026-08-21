/** 封面风格色板（非演示业务数据） */
export type CoverStyle = {
  id: string
  name: string
  accent: string
  mood: string
}

export const COVER_STYLES: CoverStyle[] = [
  { id: 'teal', name: '舞台青', accent: '#2dd4bf', mood: '清爽获客' },
  { id: 'gold', name: '暖金', accent: '#d4a574', mood: '品质感' },
  { id: 'ink', name: '墨黑', accent: '#64748b', mood: '沉稳口播' },
  { id: 'rose', name: '珊瑚', accent: '#fb7185', mood: '种草活力' },
]
