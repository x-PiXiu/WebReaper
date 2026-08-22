import type { ReactNode } from 'react'

type Props = {
  kicker?: string
  title: string
  lead?: string
  actions?: ReactNode
  className?: string
}

/** 全站统一页面标题区（衬线大标题 + 描述 + 右侧操作） */
export function PageHeader({ kicker, title, lead, actions, className = '' }: Props) {
  return (
    <header className={`ip-page-hero wr-page-header-unified ${className}`.trim()}>
      <div>
        {kicker && <p className="ip-kicker">{kicker}</p>}
        <h1>{title}</h1>
        {lead && <p className="ip-lead">{lead}</p>}
      </div>
      {actions && <div className="wr-page-header-actions">{actions}</div>}
    </header>
  )
}
