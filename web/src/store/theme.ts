import { create } from 'zustand'
import { persist } from 'zustand/middleware'

// 主题状态管理（暗/亮切换）。
// persist 到 localStorage，刷新不丢。
type ThemeMode = 'dark' | 'light'

interface ThemeState {
  mode: ThemeMode
  toggle: () => void
  setMode: (mode: ThemeMode) => void
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      mode: 'dark', // 默认暗色（项目设计调性）
      toggle: () => set((s) => ({ mode: s.mode === 'dark' ? 'light' : 'dark' })),
      setMode: (mode) => set({ mode }),
    }),
    { name: 'webreaper-theme' },
  ),
)

// 非组件场景读取当前主题
export const getThemeMode = (): ThemeMode => useThemeStore.getState().mode
