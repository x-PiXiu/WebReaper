import { Input } from 'antd'
import type { TextAreaProps } from 'antd/es/input'
import { PauseMarkerToolbar } from './PauseMarkerToolbar'
import { useTextAreaCaret } from '../../hooks/useTextAreaCaret'

const { TextArea } = Input

type Props = Omit<TextAreaProps, 'onChange' | 'value'> & {
  value: string
  onChange: (value: string) => void
  showPauseToolbar?: boolean
}

/** 口播文案输入：TextArea + 光标感知停顿工具栏 */
export function PauseScriptEditor({
  value,
  onChange,
  showPauseToolbar = true,
  ...textareaProps
}: Props) {
  const { caret, setCaret, bind } = useTextAreaCaret()

  return (
    <>
      <TextArea
        value={value}
        onChange={(e) => {
          onChange(e.target.value)
          setCaret(e.target.selectionStart)
        }}
        {...bind}
        {...textareaProps}
      />
      {showPauseToolbar && (
        <PauseMarkerToolbar
          text={value}
          onChange={onChange}
          caret={caret}
          onCaretChange={setCaret}
        />
      )}
    </>
  )
}
