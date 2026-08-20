import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { MOCK_WORKS_SEED, type WorkItem } from '../mock/ipAssets'

type WorksState = {
  works: WorkItem[]
  upsertWork: (work: WorkItem) => void
  markPublished: (id: string, platform: string) => void
  resetDemo: () => void
}

export const useWorksStore = create<WorksState>()(
  persist(
    (set) => ({
      works: MOCK_WORKS_SEED,
      upsertWork: (work) =>
        set((s) => {
          const i = s.works.findIndex((w) => w.id === work.id)
          if (i >= 0) {
            const next = [...s.works]
            next[i] = { ...next[i], ...work }
            return { works: next }
          }
          return { works: [work, ...s.works] }
        }),
      markPublished: (id, platform) =>
        set((s) => ({
          works: s.works.map((w) =>
            w.id === id
              ? {
                  ...w,
                  status: 'published' as const,
                  platform,
                  publishedAt: new Date().toISOString(),
                  views: w.views || Math.floor(800 + Math.random() * 1200),
                  likes: w.likes || Math.floor(40 + Math.random() * 120),
                  comments: w.comments || Math.floor(5 + Math.random() * 30),
                  leads: w.leads || Math.floor(2 + Math.random() * 12),
                }
              : w,
          ),
        })),
      resetDemo: () => set({ works: MOCK_WORKS_SEED }),
    }),
    { name: 'ip-works-v1' },
  ),
)
