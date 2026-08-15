import { create } from 'zustand'
import { persist } from 'zustand/middleware'

// 全局品牌上下文（商户端的"第一导航维度"）。
//
// 设计动机：品牌贯穿 关键词→内容→分发→监测 全旅程，此前五个页面各自 useState
// 存品牌选择，跨页即丢（用户每换一页都要重选）。收敛为全局单例后：
//   - 页面 Select 读写同一状态，跳转天然携带上下文
//   - Content→Distribution 的 ?brandId= query 参数特例可删除（deep-link 兼容保留）
//   - 创建品牌成功后写入，下一页（关键词/内容）自动预选
//
// 持久化到 localStorage；登出时由布局清理（见 MainLayout handleLogout）。

interface BrandContextState {
  currentBrandId: string | null
  setCurrentBrand: (id: string | null) => void
}

export const useBrandStore = create<BrandContextState>()(
  persist(
    (set) => ({
      currentBrandId: null,
      setCurrentBrand: (id) => set({ currentBrandId: id }),
    }),
    { name: 'webreaper-current-brand' }, // localStorage key
  ),
)
