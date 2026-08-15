import { Spin } from 'antd'

// 页面级加载态（全站统一约定）。
//
// 此前四种混用：居中大 Spin / Card loading 骨架 / App 级 Skeleton / 表格 loading。
// 统一口径：
//   - 页面首屏数据加载 → <PageLoading />（本组件，居中 + 语境文案）
//   - 路由级代码分割 → App.tsx 的 PageFallback（Skeleton）
//   - 页面局部刷新 → 局部区域 <Spin size="small" /> 或表格自身 loading prop
export default function PageLoading({ tip = '加载中...' }: { tip?: string }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', minHeight: 400, gap: 12 }}>
      <Spin size="large" />
      {tip && (
        <span style={{ fontSize: 13, color: 'var(--wr-text-muted)' }}>{tip}</span>
      )}
    </div>
  )
}
