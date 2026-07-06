import { useState, useRef, useCallback } from 'react'
import { Input, Button, Typography, Space, Tag, Card, Alert } from 'antd'
import { PlayCircleOutlined, StopOutlined, RobotOutlined } from '@ant-design/icons'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { executeTaskStream, type TaskStreamEvent } from '../api/taskStream'

const { Text } = Typography
const { TextArea } = Input

// 工具调用记录
interface ToolRecord {
  name: string
  args?: string
  result?: string
  status: 'calling' | 'done' | 'error'
}

// 通用任务执行页面：给 Agent 任意任务，流式看它自主完成。
export default function TaskExecute() {
  const [task, setTask] = useState('')
  const [running, setRunning] = useState(false)
  // 渲染分块：Agent 回复被拆成文本块（中间穿插工具调用）
  const [textBlocks, setTextBlocks] = useState<string[]>([''])
  const [tools, setTools] = useState<ToolRecord[]>([])
  const [error, setError] = useState('')
  const stopRef = useRef<(() => void) | null>(null)

  const handleRun = useCallback(() => {
    if (!task.trim() || running) return
    setError('')
    setTextBlocks([''])
    setTools([])
    setRunning(true)

    let currentText = ''
    const stop = executeTaskStream(
      task,
      [], // 空=全部工具
      '', // 空=默认提示词
      (evt: TaskStreamEvent) => {
        switch (evt.type) {
          case 'text-delta':
            currentText += evt.text || ''
            setTextBlocks((prev) => {
              const next = [...prev]
              next[next.length - 1] = currentText
              return next
            })
            break
          case 'tool-call':
            // 工具调用开始：固化当前文本块，开始新块；记录工具调用
            if (currentText) {
              setTextBlocks((prev) => [...prev, ''])
              currentText = ''
            }
            setTools((prev) => [...prev, { name: evt.tool_name || '?', args: evt.tool_args, status: 'calling' }])
            break
          case 'tool-result':
            setTools((prev) => prev.map((t, i) =>
              i === prev.length - 1 ? { ...t, result: evt.tool_result, status: 'done' as const } : t,
            ))
            break
          case 'finish':
            setRunning(false)
            break
          case 'error':
            setError(evt.error || '未知错误')
            setRunning(false)
            break
        }
      },
      (err: Error) => {
        setError(err.message)
        setRunning(false)
      },
    )
    stopRef.current = stop
  }, [task, running])

  const handleStop = useCallback(() => {
    stopRef.current?.()
    setRunning(false)
  }, [])

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '24px 16px' }}>
      <Card>
        <Space direction='vertical' size='middle' style={{ width: '100%' }}>
          <div>
            <RobotOutlined style={{ marginRight: 8 }} />
            <Text strong>通用任务执行</Text>
            <Text type='secondary' style={{ marginLeft: 8, fontSize: 12 }}>
              给 Agent 任意任务，它自主规划、调工具，直到完成
            </Text>
          </div>

          {/* 任务输入 */}
          <TextArea
            value={task}
            onChange={(e) => setTask(e.target.value)}
            placeholder='输入任务，例如：&#10;- 采集 trpc-agent-go 的 GitHub README 并总结其核心模块&#10;- 为 Gin 框架生成完整的面试题&#10;- 说明 Go 的 goroutine 和 channel'
            autoSize={{ minRows: 3, maxRows: 6 }}
            disabled={running}
            onPressEnter={(e) => { if (e.ctrlKey) handleRun() }}
          />
          <Space>
            <Button
              type='primary'
              icon={<PlayCircleOutlined />}
              onClick={handleRun}
              disabled={!task.trim() || running}
              loading={running}
            >
              {running ? '执行中...' : '执行任务'}
            </Button>
            {running && (
              <Button danger icon={<StopOutlined />} onClick={handleStop}>
                停止
              </Button>
            )}
            <Text type='secondary' style={{ fontSize: 12 }}>Ctrl+Enter 快速执行</Text>
          </Space>

          {error && <Alert type='error' message={error} closable />}

          {/* 执行结果 */}
          {(textBlocks.some((b) => b) || tools.length > 0) && (
            <div style={{ borderTop: '1px solid #f0f0f0', paddingTop: 16 }}>
              {textBlocks.map((block, i) => (
                block.trim() ? (
                  <ReactMarkdown key={i} remarkPlugins={[remarkGfm]}>{block}</ReactMarkdown>
                ) : null
              ))}
              {tools.map((t, i) => (
                <Card key={i} size='small' style={{ margin: '8px 0', background: '#fafafa' }}>
                  <Space>
                    <Tag color={t.status === 'done' ? 'green' : t.status === 'error' ? 'red' : 'blue'}>
                      {t.status === 'calling' ? '调用中' : t.status === 'done' ? '完成' : '错误'}
                    </Tag>
                    <Text strong>{t.name}</Text>
                  </Space>
                  {t.result && (
                    <pre style={{ margin: '8px 0 0', fontSize: 12, color: '#666', maxHeight: 120, overflow: 'auto' }}>
                      {t.result.slice(0, 500)}
                    </pre>
                  )}
                </Card>
              ))}
              {running && <Text type='secondary'>Agent 思考中...</Text>}
            </div>
          )}
        </Space>
      </Card>
    </div>
  )
}
