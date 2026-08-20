import { useState, useRef, useEffect, useCallback, useMemo } from 'react'
import { Input, Button, Spin, Typography, Select, Space, Tag, Popover, Switch, Modal, message as antdMessage } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useLocation } from 'react-router-dom'
import LazyChatMarkdown from '../../components/markdown/LazyChatMarkdown'
import { getToken, useAuthStore } from '../../store/auth'
import { businessApi } from '../../api/business'
import type { ChatMessage, AgentConfig, EngineOption, ToolView } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input

// formatConvTime 格式化会话最后活动时间（精确到秒）。
// 智能显示：今天 → HH:MM:SS；昨天 → "昨天 HH:MM"；本年 → MM-DD HH:MM；更早 → YYYY-MM-DD。
function formatConvTime(ts: number): string {
  const d = new Date(ts)
  const now = new Date()
  const pad = (n: number) => n < 10 ? '0' + n : '' + n
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`
  const hms = `${hm}:${pad(d.getSeconds())}`

  const isSameDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)

  if (isSameDay(d, now)) return hms                // 今天：精确到秒
  if (isSameDay(d, yesterday)) return `昨天 ${hm}`  // 昨天
  if (d.getFullYear() === now.getFullYear()) {
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${hm}` // 本年：月日时分
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` // 更早：完整日期
}

interface ToolRecord { name: string; args: string; result?: string }

// 渲染块：消息内容被拆分为有序的块（工具/思考/正文交替出现）
interface RenderBlock {
  type: 'tool' | 'think' | 'text'
  tool?: ToolRecord
  content?: string  // think 或 text 的内容
}

// 统一的渲染块——所有事件（工具调用/思考/正文）按时间顺序存在这里
interface RenderBlock {
  type: 'tool' | 'think' | 'text'
  tool?: ToolRecord
  content?: string
  closed?: boolean // think 块专用：true=已闭合（</think>已到达），false=还在流式中
}

interface RichMessage extends ChatMessage {
  id: string
  tools?: ToolRecord[]      // 保留向后兼容
  blocks?: RenderBlock[]    // 新：按时间顺序的完整事件流
}

// 会话结构（前端内存态：messages 在选中会话时按需从后端加载）
interface Conversation {
  id: string
  title: string
  agentName?: string
  messages: RichMessage[]   // 当前会话的消息（选中时从后端加载；流式中实时更新）
  createdAt: number
  updatedAt: number         // 最后活动时间（发消息时本地更新，列表刷新时从后端同步）
}

// <think> 解析

// 思考块：流式中展开，闭合后自动收起
// active = 这个 think 块是否还在流式增长（它后面没有更多块了，且整体还在 streaming）
function ThinkBlock({ content, active }: { content: string; active?: boolean }) {
  const [open, setOpen] = useState(true)
  const wasActive = useRef(active)

  useEffect(() => {
    // active 从 true → false 意味着后面来了新块（或 streaming 结束），think 块完成了
    if (wasActive.current && !active) {
      const timer = setTimeout(() => setOpen(false), 300) // 延迟 300ms 收起，让用户看到思考完成
      return () => clearTimeout(timer)
    }
    wasActive.current = active
  }, [active])

  return (
    <div style={{ marginBottom: 8 }}>
      <div
        onClick={() => setOpen(!open)}
        style={{
          display: 'inline-flex', alignItems: 'center', gap: 4, cursor: 'pointer',
          fontSize: 12, color: 'var(--wr-text-muted)', fontStyle: 'italic',
          padding: '2px 8px', borderRadius: 6,
          background: active ? 'var(--wr-primary-bg)' : 'transparent',
          transition: 'background 150ms',
        }}
      >
        <span style={{ fontSize: 10 }}>{open ? '▾' : '▸'}</span>
        {active && open ? <Spin size="small" style={{ transform: 'scale(0.7)' }} /> : null}
        <span>{active ? 'Thinking...' : 'Thought process'}</span>
      </div>
      {open && (
        <pre style={{
          whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontSize: 12,
          color: 'var(--wr-text-muted)', margin: '4px 0 0', padding: '8px 12px',
          background: 'var(--wr-bg-elevated)', borderRadius: 8,
          borderLeft: '2px solid var(--wr-primary)',
        }}>{content}</pre>
      )}
    </div>
  )
}

// 工具调用块：可点击收起/展开，结果到达后自动收起
function ToolCallBlock({ tool }: { tool: ToolRecord }) {
  const isRunning = !tool.result
  // 调用中展开，结果到达后收起（用户可点击重新展开）
  const [open, setOpen] = useState(true)
  const wasRunning = useRef(isRunning)

  useEffect(() => {
    // isRunning 从 true → false 意味着结果到了
    if (wasRunning.current && !isRunning) {
      const timer = setTimeout(() => setOpen(false), 500) // 延迟 500ms 让用户看到结果
      return () => clearTimeout(timer)
    }
    wasRunning.current = isRunning
  }, [isRunning])

  return (
    <div style={{
      marginBottom: 8, borderRadius: 8,
      border: '1px solid var(--wr-border)',
      background: isRunning ? 'rgba(34, 211, 238, 0.06)' : 'transparent',
      overflow: 'hidden',
      transition: 'background 200ms',
    }}>
      {/* 头部：可点击展开/收起 */}
      <div
        onClick={() => setOpen(!open)}
        style={{
          display: 'flex', alignItems: 'center', gap: 6,
          padding: '6px 10px', fontSize: 12, cursor: 'pointer',
        }}
      >
        <span style={{ fontSize: 10, color: 'var(--wr-text-muted)' }}>{open ? '▾' : '▸'}</span>
        <Tag color="cyan" style={{ margin: 0, fontSize: 11 }}>{tool.name}</Tag>
        {isRunning ? (
          <span style={{ fontSize: 11, color: 'var(--wr-accent)', display: 'flex', alignItems: 'center', gap: 4 }}>
            <Spin size="small" style={{ transform: 'scale(0.7)' }} />
            调用中...
          </span>
        ) : (
          <span style={{ fontSize: 11, color: 'var(--wr-success)' }}>✓ 已完成</span>
        )}
      </div>
      {/* 参数 + 结果（可展开/收起） */}
      {open && (tool.args || tool.result) && (
        <div style={{ padding: '0 10px 8px', fontSize: 11 }}>
          {tool.args && (
            <div style={{ marginBottom: tool.result ? 6 : 0 }}>
              <span style={{ color: 'var(--wr-text-muted)' }}>参数: </span>
              <code style={{ color: 'var(--wr-accent)', fontSize: 11 }}>{tool.args}</code>
            </div>
          )}
          {tool.result && (
            <div>
              <span style={{ color: 'var(--wr-text-muted)' }}>结果: </span>
              <pre style={{
                whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                margin: '2px 0 0', padding: 6,
                background: 'rgba(0,0,0,0.15)', borderRadius: 6,
                fontSize: 11, color: 'var(--wr-text-secondary)',
                maxHeight: 150, overflowY: 'auto',
              }}>{tool.result}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// 从 SSE 事件流重建消息时，blocks 是实时维护的（不再从 content+tools 事后拼装）
// 但从 localStorage 加载历史消息时，blocks 可能为空（旧格式），需要从 content 回退解析
function parseBlocks(content: string, _tools: ToolRecord[] | undefined, savedBlocks?: RenderBlock[]): RenderBlock[] {
  // 优先使用实时记录的 blocks（最准确的时间线）
  if (savedBlocks && savedBlocks.length > 0) return savedBlocks

  // 回退：从 content 中解析 <think> 和正文（工具不单独插入，因为旧格式无法还原顺序）
  const blocks: RenderBlock[] = []
  if (content) {
    const re = /<think>([\s\S]*?)<\/think>/g
    let lastIdx = 0
    let m
    while ((m = re.exec(content)) !== null) {
      const textBefore = content.slice(lastIdx, m.index).trim()
      if (textBefore) blocks.push({ type: 'text', content: textBefore })
      blocks.push({ type: 'think', content: m[1].trim(), closed: true }) // 已闭合
      lastIdx = m.index + m[0].length
    }
    let remaining = content.slice(lastIdx).trim()
    const openIdx = remaining.indexOf('<think>')
    if (openIdx >= 0) {
      const textBefore = remaining.slice(0, openIdx).trim()
      if (textBefore) blocks.push({ type: 'text', content: textBefore })
      const activeThink = remaining.slice(openIdx + 7).trim()
      if (activeThink) blocks.push({ type: 'think', content: activeThink, closed: false }) // 未闭合
      remaining = ''
    }
    if (remaining) blocks.push({ type: 'text', content: remaining })
  }
  return blocks
}

function MessageContent({ content, tools, savedBlocks, isStreaming }: { content: string; tools?: ToolRecord[]; savedBlocks?: RenderBlock[]; isStreaming?: boolean }) {
  const blocks = parseBlocks(content, tools, savedBlocks)
  const hasContent = blocks.length > 0 || content

  if (!hasContent) return <Spin size="small" />

  // 找最后一个块来判断 streaming 状态

  return (
    <div style={{ lineHeight: 1.7, fontSize: 15 }}>
      {blocks.map((block, i) => {
        if (block.type === 'tool' && block.tool) {
          return <ToolCallBlock key={`t${i}`} tool={block.tool} />
        }
        if (block.type === 'think' && block.content) {
          // active = 未闭合 且 在流式中（closed 字段在实时 blocks 中设置）
          // 回退路径中，已闭合的 think 从正则解析出，天然 closed=true
          const isActive = isStreaming && !block.closed
          return <ThinkBlock key={`k${i}`} content={block.content} active={isActive} />
        }
        if (block.type === 'text' && block.content) {
          return <div key={`x${i}`} style={{ marginBottom: 8 }}><LazyChatMarkdown content={block.content} /></div>
        }
        return null
      })}
      {isStreaming && !content && (!tools || tools.length === 0) && <Spin size="small" />}
    </div>
  )
}

export default function Chat() {
  const queryClient = useQueryClient()
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [currentConvId, setCurrentConvId] = useState<string | null>(null)
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedAgent, setSelectedAgent] = useState<string | undefined>(undefined)
  const [selectedLLM, setSelectedLLM] = useState<string | undefined>(undefined) // 聊天界面临时覆盖 LLM（空则用 Agent 默认）
  const [loadedConvMsgs, setLoadedConvMsgs] = useState<Set<string>>(new Set()) // 已加载消息的会话
  const endRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)
  const { username } = useAuthStore()

  const { data: agentConfigs = [] } = useQuery({ queryKey: ['agent-configs'], queryFn: () => businessApi.listAgentConfigs() })
  const { data: tools = [] } = useQuery({ queryKey: ['tools'], queryFn: () => businessApi.listTools() })
  // 引擎名单：聊天界面动态切换模型（覆盖 Agent 默认）——仅 name/provider/model，不含厂商密钥
  const { data: llmConfigs = [] } = useQuery({ queryKey: ['geo-engines'], queryFn: () => businessApi.listEngines() })
  const currentAgent = agentConfigs.find((a: AgentConfig) => a.name === selectedAgent)

  // 拉取会话列表（后端持久化，按当前用户隔离）
  const { data: serverConvs = [] } = useQuery({ queryKey: ['conversations'], queryFn: () => businessApi.listConversations() })

  // 同步后端会话列表到本地状态（保留已加载的 messages，避免重复请求）
  useEffect(() => {
    setConversations(prev => {
      const prevMap = new Map(prev.map(c => [c.id, c]))
      return serverConvs.map(sc => {
        const exist = prevMap.get(sc.id)
        const serverUpdated = new Date(sc.updated_at || sc.created_at || Date.now()).getTime()
        if (exist) {
          // 保留已加载的消息；updatedAt 取本地与后端的较大值
          // （发送消息后到列表刷新前，本地的 updatedAt 比后端更及时）
          return { ...exist, updatedAt: Math.max(exist.updatedAt || 0, serverUpdated) }
        }
        return {
          id: sc.id, title: sc.title, agentName: sc.agent_name, messages: [],
          createdAt: new Date(sc.created_at || Date.now()).getTime(),
          updatedAt: serverUpdated,
        }
      })
    })
  }, [serverConvs])

  // 当前会话
  const currentConv = conversations.find(c => c.id === currentConvId)
  const messages = currentConv?.messages || []

  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages])

  // 切换会话时同步 Agent 选择
  useEffect(() => {
    if (currentConv?.agentName) setSelectedAgent(currentConv.agentName)
  }, [currentConvId])

  // 选中会话时按需加载消息（只在未加载过时拉取）
  useEffect(() => {
    if (!currentConvId || loadedConvMsgs.has(currentConvId)) return
    businessApi.getMessages(currentConvId).then(recs => {
      const richMsgs: RichMessage[] = recs.map(r => ({
        id: r.id, role: r.role as 'user' | 'assistant', content: r.content,
        tools: r.tool_calls ? safeParseTools(r.tool_calls) : [],
      }))
      setConversations(prev => prev.map(c => c.id === currentConvId ? { ...c, messages: richMsgs } : c))
      setLoadedConvMsgs(prev => new Set(prev).add(currentConvId))
    }).catch(() => {
      // 加载失败标记为已加载，避免反复重试
      setLoadedConvMsgs(prev => new Set(prev).add(currentConvId))
    })
  }, [currentConvId, loadedConvMsgs])

  const newConversation = () => {
    setCurrentConvId(null)
    setInput('')
    setError(null)
  }

  // 工具调用 JSON 安全反序列化（历史消息还原）
  function safeParseTools(json: string): ToolRecord[] {
    try { return JSON.parse(json) || [] } catch { return [] }
  }

  // 停止生成（abort）：保留已流式输出的部分内容
  const stopGeneration = useCallback(() => {
    if (abortRef.current) { abortRef.current.abort(); abortRef.current = null }
    setStreaming(false)
  }, [])

  // 更新指定会话的消息（供流式回调使用）
  const patchConvMessages = (convId: string, updater: (msgs: RichMessage[]) => RichMessage[]) => {
    setConversations(prev => prev.map(c => c.id === convId ? { ...c, messages: updater(c.messages) } : c))
  }

  // GEO 助手：商户端注入品牌数据摘要（榜豆式——让 AI 能解读你的 GEO 状态）
  const location = useLocation()
  const isMerchantChat = location.pathname.startsWith('/m')
  const { data: geoBrands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands().catch(() => []),
    enabled: isMerchantChat,
  })
  const { data: geoMonitor = [] } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults().catch(() => []),
    enabled: isMerchantChat,
  })
  const geoSummary = useMemo(() => {
    if (!isMerchantChat || geoBrands.length === 0) return ''
    const parts: string[] = []
    geoBrands.forEach((b: any) => {
      const list = geoMonitor.filter((r: any) => r.brand_id === b.id)
      const latest = [...list].sort((a: any, c: any) => new Date(c.probed_at).getTime() - new Date(a.probed_at).getTime())[0]
      if (!latest) return
      const senti = latest.sentiment === 'positive' ? '正面' : latest.sentiment === 'negative' ? '负面' : '中性'
      parts.push(`${b.name}：可见度 ${Math.round((latest.mention_rate || 0) * 100)}%、情感${senti}${latest.avg_position ? `、位次#${latest.avg_position}` : ''}${latest.self_source_count ? `、被引用${latest.self_source_count}次` : ''}`)
    })
    if (parts.length === 0) return ''
    return `【账号 IP 摘要·仅供参考】\n${parts.join('\n')}`
  }, [isMerchantChat, geoBrands, geoMonitor])

  const GEO_QUICK_QUESTIONS = [
    { label: '我的账号最近表现如何？', q: '根据上面的账号 IP 摘要，我最近在内容与曝光上的表现如何？有什么亮点和问题？' },
    { label: '哪个竞品更值得盯？', q: '根据上面的账号 IP 摘要，哪个竞品对我的威胁最大？我应该怎么应对？' },
    { label: '下一步该做什么？', q: '根据上面的账号 IP 摘要，给我 3 条具体可执行的下一步建议（合成、发布或数据复盘）。' },
  ]

  const doSend = async (text: string) => {
    if (!text.trim() || streaming) return
    const userMsg: RichMessage = { id: `u${Date.now()}`, role: 'user', content: text }
    const aMsg: RichMessage = { id: `a${Date.now()}`, role: 'assistant', content: '', tools: [] }

    // 创建或更新会话
    let convId = currentConvId
    let isNewConv = false
    if (!convId) {
      convId = `conv${Date.now()}`
      isNewConv = true
      const newConv: Conversation = {
        id: convId,
        title: text.slice(0, 30),
        agentName: selectedAgent,
        messages: [userMsg, aMsg],
        createdAt: Date.now(),
        updatedAt: Date.now(),
      }
      setConversations(prev => [newConv, ...prev])
      setCurrentConvId(convId)
      setLoadedConvMsgs(prev => new Set(prev).add(convId!)) // 新会话无需再加载
      // 后端持久化：创建会话
      businessApi.createConversation({ id: convId!, title: newConv.title, agent_name: selectedAgent }).catch(() => {})
    } else {
      patchConvMessages(convId, msgs => [...msgs, userMsg, aMsg])
      // 更新会话的最后活动时间（侧边栏显示用）
      setConversations(prev => prev.map(c => c.id === convId ? { ...c, updatedAt: Date.now() } : c))
    }

    // 后端持久化：保存 user 消息
    businessApi.saveMessage(convId, { id: userMsg.id, role: 'user', content: userMsg.content }).catch(() => {})

    const hist = [...messages, userMsg]
    setInput(''); setStreaming(true); setError(null)

    // 记录会话使用的 Agent（新会话已在 create 时传，旧会话这里更新本地即可）
    if (selectedAgent && !isNewConv) {
      setConversations(prev => prev.map(c => c.id === convId ? { ...c, agentName: selectedAgent } : c))
    }

    // abort 控制器（支持"停止生成"）
    const controller = new AbortController()
    abortRef.current = controller

    // 本地累积变量：流式过程中实时记录 assistant 消息的最终状态。
    // 关键修复：不依赖 conversationsRef（useEffect 异步同步，末尾内容会丢失），
    // 而是用同步的本地变量，流式结束后直接读它，确保完整内容入库。
    // 声明在 try 外，catch 块（AbortError）也能访问已累积的部分内容。
    let accContent = ''
    let accTools: ToolRecord[] = []

    try {
      const token = getToken()
      const res = await fetch('/api/v1/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: JSON.stringify({
          conversation_id: convId, // 会话隔离的关键：后端据此区分 sessionID，根治记忆串台
          messages: hist.map(({ id: _, tools: _t, blocks: _b, ...r }) => r),
          system_message: currentAgent?.system_prompt || '',
          tools: [], // 空数组=后端使用全部已启用的工具（toolRegistry.All()）
          use_tools: true, // 启用工具模式（ReAct Agent + 爬虫工具调用）
          llm_config_name: selectedLLM || currentAgent?.llm_config_name || '', // 聊天界面可临时覆盖 Agent 默认 LLM
        }),
        signal: controller.signal,
      })
      // 401 处理：清登录态并跳登录（复用 client.ts 拦截器逻辑）
      if (res.status === 401) {
        useAuthStore.getState().clearAuth()
        window.location.href = '/login'
        return
      }
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const reader = res.body?.getReader(); const dec = new TextDecoder(); let buf = ''
      const convIdRef = convId

      while (reader) {
        const { done, value } = await reader.read(); if (done) break
        buf += dec.decode(value, { stream: true }); const lines = buf.split('\n'); buf = lines.pop() || ''
        for (const ln of lines) {
          if (!ln.startsWith('data: ')) continue
          try {
            const e = JSON.parse(ln.slice(6))
            patchConvMessages(convIdRef, cmsgs => {
              const msgs = [...cmsgs]
              const oldMsg = msgs[msgs.length - 1]
              const blocks: RenderBlock[] = oldMsg.blocks ? oldMsg.blocks.map(b => ({ ...b, tool: b.tool ? { ...b.tool } : undefined })) : []
              let newContent = oldMsg.content
              const newTools = oldMsg.tools ? [...oldMsg.tools] : []

              if (e.type === 'text-delta' && e.text) {
                newContent = oldMsg.content + e.text
                // 同步累积到本地变量（用于流式结束后入库）
                accContent = accContent + e.text
                let textToAdd = e.text

                // 检测 <think> 开始
                if (textToAdd.includes('<think>')) {
                  const thinkParts = textToAdd.split('<think>')
                  if (thinkParts[0].trim()) {
                    const last = blocks[blocks.length - 1]
                    if (last && last.type === 'text') {
                      last.content = (last.content || '') + thinkParts[0]
                    } else {
                      blocks.push({ type: 'text', content: thinkParts[0] })
                    }
                  }
                  const afterThink = thinkParts[1] || ''
                  if (afterThink.includes('</think>')) {
                    const closedParts = afterThink.split('</think>')
                    blocks.push({ type: 'think', content: closedParts[0].trim(), closed: true })
                    if (closedParts[1] && closedParts[1].trim()) {
                      blocks.push({ type: 'text', content: closedParts[1] })
                    }
                  } else {
                    blocks.push({ type: 'think', content: afterThink.trim(), closed: false })
                  }
                  textToAdd = ''
                }

                // 检测 </think> 闭合
                if (textToAdd.includes('</think>')) {
                  for (let bi = blocks.length - 1; bi >= 0; bi--) {
                    if (blocks[bi].type === 'think' && !blocks[bi].closed) {
                      const parts = textToAdd.split('</think>')
                      blocks[bi].content = ((blocks[bi].content || '') + parts[0]).trim()
                      blocks[bi].closed = true
                      if (parts[1] && parts[1].trim()) {
                        blocks.push({ type: 'text', content: parts[1] })
                      }
                      textToAdd = ''
                      break
                    }
                  }
                }

                // 剩余的普通文字追加到最后一个 text 块
                if (textToAdd) {
                  const last = blocks[blocks.length - 1]
                  if (last && last.type === 'text') {
                    last.content = (last.content || '') + textToAdd
                  } else if (last && last.type === 'think' && !last.closed) {
                    last.content = (last.content || '') + textToAdd
                  } else {
                    blocks.push({ type: 'text', content: textToAdd })
                  }
                }
              } else if (e.type === 'tool-call') {
                const newTool: ToolRecord = { name: e.tool_name, args: e.tool_args }
                newTools.push(newTool)
                accTools.push(newTool) // 同步累积
                blocks.push({ type: 'tool', tool: newTool })
              } else if (e.type === 'tool-result') {
                for (let bi = blocks.length - 1; bi >= 0; bi--) {
                  if (blocks[bi].type === 'tool' && blocks[bi].tool && !blocks[bi].tool!.result) {
                    blocks[bi].tool!.result = e.tool_result
                    // 同步更新本地累积的 tools
                    for (let ti = accTools.length - 1; ti >= 0; ti--) {
                      if (!accTools[ti].result) { accTools[ti].result = e.tool_result; break }
                    }
                    break
                  }
                }
              } else if (e.type === 'error') setError(e.error || null)

              msgs[msgs.length - 1] = { ...oldMsg, content: newContent, tools: newTools, blocks }
              return msgs
            })
          } catch {}
        }
      }
      // 流式正常结束：保存 assistant 消息到后端
      // 关键修复：从本地累积变量读取（同步、完整），而非 conversationsRef（异步、可能漏末尾）
      if (accContent || accTools.length > 0) {
        businessApi.saveMessage(convIdRef, {
          id: aMsg.id, role: 'assistant', content: accContent,
          tool_calls: accTools.length > 0 ? JSON.stringify(accTools) : '',
        }).catch(() => {})
      }
    } catch (e) {
      const err = e as Error
      // AbortError：用户主动停止，保留已生成内容（不当作错误）
      if (err.name === 'AbortError') {
        // 从本地累积变量读取已生成的部分内容
        if (accContent) {
          businessApi.saveMessage(convId!, {
            id: aMsg.id, role: 'assistant', content: accContent,
            tool_calls: accTools.length > 0 ? JSON.stringify(accTools) : '',
          }).catch(() => {})
        }
      } else {
        setError(err.message || null)
      }
    } finally {
      setStreaming(false)
      abortRef.current = null
      // 刷新会话列表（更新时间排序）
      queryClient.invalidateQueries({ queryKey: ['conversations'] })
    }
  }

  // 回车发送，shift+回车换行
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); doSend(input) }
  }

  // GEO 快捷问：摘要+问题一起发送（榜豆式——AI 有上下文才能解读）
  const askGeo = (q: string) => {
    if (streaming) return
    doSend(geoSummary ? `${geoSummary}\n\n${q}` : q)
  }

  const deleteConv = async (id: string) => {
    setConversations(prev => prev.filter(c => c.id !== id))
    setLoadedConvMsgs(prev => { const n = new Set(prev); n.delete(id); return n })
    if (currentConvId === id) setCurrentConvId(null)
    try { await businessApi.deleteConversation(id) } catch {}
    queryClient.invalidateQueries({ queryKey: ['conversations'] })
  }

  // 会话重命名（P2-9-11：标题为首句截断，用户可手动修正）
  const [renameTarget, setRenameTarget] = useState<{ id: string; title: string } | null>(null)
  const openRename = (id: string, title: string) => setRenameTarget({ id, title })
  const doRename = async () => {
    if (!renameTarget || !renameTarget.title.trim()) return
    try {
      await businessApi.renameConversation(renameTarget.id, renameTarget.title.trim())
      setConversations(prev => prev.map(c => c.id === renameTarget.id ? { ...c, title: renameTarget.title.trim() } : c))
      setRenameTarget(null)
    } catch { /* 拦截器已提示 */ }
  }

  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 56px)' }}>
      {/* 左侧会话列表 */}
      <div style={{
        width: 220, flexShrink: 0, borderRight: '1px solid var(--wr-border)',
        display: 'flex', flexDirection: 'column',
      }}>
        <div style={{ padding: '8px 12px' }}>
          <Button block onClick={newConversation} type="primary" size="small">+ 新对话</Button>
        </div>
        <div style={{ flex: 1, overflowY: 'auto', padding: '0 8px' }}>
          {conversations.map(c => (
            <div key={c.id} onClick={() => setCurrentConvId(c.id)} style={{
              padding: '8px 12px', borderRadius: 8, marginBottom: 4, cursor: 'pointer',
              background: c.id === currentConvId ? 'var(--wr-primary-bg)' : 'transparent',
              transition: 'background 150ms',
            }} onMouseEnter={e => { if (c.id !== currentConvId) e.currentTarget.style.background = 'var(--wr-bg-hover)' }} onMouseLeave={e => { if (c.id !== currentConvId) e.currentTarget.style.background = 'transparent' }}>
              <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--wr-text-primary)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{c.title}</div>
              <div style={{ fontSize: 12, color: 'var(--wr-text-muted)', display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 2, gap: 6 }}>
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{c.agentName || '默认'}</span>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
                    <span style={{ fontSize: 11, fontVariantNumeric: 'tabular-nums' }} title={new Date(c.updatedAt || c.createdAt).toLocaleString()}>
                      {c.updatedAt ? formatConvTime(c.updatedAt) : ''}
                    </span>
                    {/* 重命名（P2-9-11：标题为首句截断，用户可手动修正） */}
                    <span onClick={(e) => { e.stopPropagation(); openRename(c.id, c.title) }} style={{ cursor: 'pointer', opacity: 0.5 }} title="重命名">改</span>
                    <span onClick={(e) => { e.stopPropagation(); deleteConv(c.id) }} style={{ cursor: 'pointer', opacity: 0.5 }}>×</span>
                  </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* 右侧对话区 */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        {/* 顶栏 */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 24px', borderBottom: '1px solid var(--wr-border)' }}>
          <Space>
            <Text type="secondary" style={{ fontSize: 14 }}>角色：</Text>
            <Select style={{ width: 220 }} placeholder="默认 AI" allowClear value={selectedAgent} onChange={setSelectedAgent} options={agentConfigs.map((a: AgentConfig) => ({ value: a.name, label: a.name }))} />
            <Text type="secondary" style={{ fontSize: 14 }}>模型：</Text>
            <Select
              style={{ width: 200 }}
              placeholder="用 Agent 默认"
              allowClear
              value={selectedLLM}
              onChange={setSelectedLLM}
              options={llmConfigs.map((l: EngineOption) => ({ value: l.name, label: `${l.model}${l.provider ? ' · ' + l.provider : ''}` }))}
              title={selectedLLM ? `当前强制使用 ${selectedLLM}（覆盖 Agent 默认）` : '留空则使用所选 Agent 配置的默认模型'}
            />
            <Popover
              trigger="click"
              placement="bottomLeft"
              title="工具管理"
              content={
                <div style={{ maxWidth: 420 }}>
                  <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>
                    点击开关启用/禁用工具。禁用的工具 Agent 不会调用。
                  </Typography.Text>
                  <div style={{ maxHeight: 360, overflowY: 'auto' }}>
                    {tools.length === 0 ? (
                      <Typography.Text type="secondary">暂无工具</Typography.Text>
                    ) : tools.map((t: ToolView) => (
                      <div key={t.name} style={{
                        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                        padding: '6px 0', borderBottom: '1px solid var(--wr-border)',
                      }}>
                        <div style={{ minWidth: 0, flex: 1, marginRight: 12 }}>
                          <Typography.Text code style={{ fontSize: 12 }}>{t.name}</Typography.Text>
                          {t.description && (
                            <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                              {t.description}
                            </Typography.Text>
                          )}
                        </div>
                        <Switch
                          size="small"
                          checked={t.enabled}
                          onChange={async (checked) => {
                            try {
                              await businessApi.toggleTool(t.name, checked)
                              antdMessage.success(`${checked ? '启用' : '禁用'}：${t.name}`)
                              queryClient.invalidateQueries({ queryKey: ['tools'] })
                            } catch {
                              // axios 拦截器已提示
                            }
                          }}
                        />
                      </div>
                    ))}
                  </div>
                </div>
              }
            >
              <Tag color="purple" style={{ cursor: 'pointer' }}>
                {tools.filter((t: ToolView) => t.enabled).length}/{tools.length} 个工具可用
              </Tag>
            </Popover>
          </Space>
        </div>

        {/* 对话内容 */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '16px 0' }}>
          {messages.length === 0 ? (
            <div style={{ textAlign: 'center', paddingTop: 60 }}>
              <div style={{ width: 56, height: 56, borderRadius: 14, background: 'linear-gradient(135deg, var(--wr-primary), var(--wr-accent))', margin: '0 auto 20px', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 26, color: '#fff', boxShadow: '0 0 32px var(--wr-primary-bg)' }}>{currentAgent ? currentAgent.name[0].toUpperCase() : 'AI'}</div>
              <Text style={{ fontSize: 20, fontWeight: 600, display: 'block', marginBottom: 8 }}>{currentAgent ? `与 ${currentAgent.name} 对话` : '开始对话'}</Text>
              <Text type="secondary" style={{ fontSize: 15 }}>回车发送 · Shift+回车换行 · 工具自动调用</Text>
            </div>
          ) : (
            <div style={{ maxWidth: 1080, margin: '0 auto', padding: '0 24px' }}>
              {messages.map(m => {
                const isLA = m.role === 'assistant' && m.id === messages[messages.length - 1]?.id && streaming
                const isUser = m.role === 'user'
                return (
                  <div key={m.id} style={{ marginBottom: 20, display: 'flex', justifyContent: isUser ? 'flex-end' : 'flex-start', alignItems: 'flex-start' }}>
                    {!isUser && <div style={{ width: 36, height: 36, borderRadius: 8, flexShrink: 0, marginRight: 12, background: 'linear-gradient(135deg, var(--wr-primary), var(--wr-accent))', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 14, fontWeight: 700, color: '#fff' }}>{currentAgent ? currentAgent.name[0].toUpperCase() : 'AI'}</div>}
                    <div style={{ maxWidth: '76%', padding: '12px 18px', borderRadius: isUser ? '18px 18px 4px 18px' : '18px 18px 18px 4px', background: isUser ? 'var(--wr-primary)' : 'var(--wr-bg-elevated)', color: isUser ? '#fff' : 'var(--wr-text-primary)', border: isUser ? 'none' : '1px solid var(--wr-border)' }}>
                      {isUser ? <span style={{ lineHeight: 1.6, fontSize: 15 }}>{m.content}</span> : <MessageContent content={m.content} tools={m.tools} savedBlocks={m.blocks} isStreaming={isLA} />}
                    </div>
                    {isUser && <div style={{ width: 36, height: 36, borderRadius: 8, flexShrink: 0, marginLeft: 12, background: 'linear-gradient(135deg, #22d3ee, #6366f1)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 14, fontWeight: 700, color: '#fff' }}>{(username || '?')[0].toUpperCase()}</div>}
                  </div>
                )
              })}
              <div ref={endRef} />
              {error && <Text type="danger" style={{ display: 'block', textAlign: 'center' }}>{error}</Text>}
            </div>
          )}
        </div>

        {/* 输入区 */}
        <div style={{ flexShrink: 0, padding: '8px 24px', borderTop: '1px solid var(--wr-border)' }}>
          {/* GEO 快捷问（商户端）：注入实时数据摘要，让 AI 解读你的品牌状态 */}
          {isMerchantChat && (
            <div style={{ maxWidth: 1080, margin: '0 auto 8px', display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
              <Text type="secondary" style={{ fontSize: 11, flexShrink: 0 }}>IP 助手：</Text>
              {GEO_QUICK_QUESTIONS.map((item) => (
                <Button
                  key={item.label}
                  size="small"
                  disabled={streaming || !geoSummary}
                  title={geoSummary ? undefined : '暂无摘要数据——先完善人设并发布作品'}
                  onClick={() => askGeo(item.q)}
                  style={{ fontSize: 12 }}
                >
                  {item.label}
                </Button>
              ))}
            </div>
          )}
          <div style={{ display: 'flex', gap: 12, maxWidth: 1080, margin: '0 auto' }}>
            <TextArea value={input} onChange={e => setInput(e.target.value)} placeholder="回车发送，Shift+回车换行..." autoSize={{ minRows: 1, maxRows: 4 }} onKeyDown={handleKeyDown} style={{ borderRadius: 12 }} disabled={streaming} />
            {streaming ? (
              <Button danger onClick={stopGeneration} style={{ height: 'auto', borderRadius: 12, minWidth: 56 }}>停止</Button>
            ) : (
              <Button type="primary" onClick={() => doSend(input)} style={{ height: 'auto', borderRadius: 12, minWidth: 56 }}>发送</Button>
            )}
          </div>
        </div>
      </div>

      {/* 会话重命名弹窗 */}
      <Modal
        title="重命名会话"
        open={!!renameTarget}
        onCancel={() => setRenameTarget(null)}
        onOk={doRename}
        okText="保存"
        width={400}
      >
        <Input
          value={renameTarget?.title || ''}
          onChange={(e) => setRenameTarget(prev => prev ? { ...prev, title: e.target.value } : prev)}
          placeholder="输入会话标题"
          maxLength={60}
          style={{ marginTop: 8 }}
        />
      </Modal>
    </div>
  )
}
