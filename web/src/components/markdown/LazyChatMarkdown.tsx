import { lazy, Suspense } from 'react'
import { Spin } from 'antd'

const ChatMarkdown = lazy(() => import('./ChatMarkdown'))

export default function LazyChatMarkdown({ content }: { content: string }) {
  return (
    <Suspense fallback={<Spin size="small" />}>
      <ChatMarkdown content={content} />
    </Suspense>
  )
}
