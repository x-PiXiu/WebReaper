import { useCallback, useState } from 'react'

type TextAreaElement = HTMLTextAreaElement

/** 跟踪 TextArea 光标位置，供停顿标记等工具栏在插入点写入 */
export function useTextAreaCaret() {
  const [caret, setCaret] = useState<number | undefined>(undefined)

  const syncCaret = useCallback((e: React.SyntheticEvent<TextAreaElement>) => {
    setCaret(e.currentTarget.selectionStart)
  }, [])

  return {
    caret,
    setCaret,
    bind: {
      onSelect: syncCaret,
      onClick: syncCaret,
      onKeyUp: syncCaret,
    },
  }
}
