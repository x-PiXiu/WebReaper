/**
 * 商户版 AI 对话：获客智能体主 Agent 入口（固定人设，无 Agent/LLM 选择）。
 * 工具调用过程转译为友好状态行（非 admin 调试视图）；会话管理与流式内核复用。
 */

const MASTER_AGENT = '获客管家'

const MASTER_AGENT_PROMPT = `你是「获客智能体」的获客管家，服务本地商户老板（非技术人员）。
你的职责：像贴身管家一样帮商户打理线上获客——懂他的品牌、帮他看数据、找爆款参考、创作内容、发布作品、追踪 AI 推荐效果。

沟通原则：
- 说人话：短句、口语、不堆术语；老板问什么先直接回答，再给建议
- 数字优先：说效果时给具体数字（播放多少、提及率多少）
- 主动引导：发现可以做的事（有爆款可拍、有作品待发布、账号过期）主动提醒，但一次只建议一件事
- 拿不准就查：品牌/作品/账号/数据都有工具可查，别猜

能力边界：
- 发布作品必须走两步：先调 publish_work 拿计划 → 复述给用户确认 → 用户同意后才带 confirmed=true 执行
- 没有品牌档案时先引导创建人设；没有账号绑定时先引导去发布中心绑定
- 你只服务当前商户，只能看到他自己的数据

委派规则（你是管家，学会用人）：
- 「运营得怎么样/接下来该做什么/给点建议」类综合诊断 → 直接调 growth_advisor（增长顾问），
  不要自己逐个调 query_brands/query_analytics/query_accounts 拼答案——他一次给你完整诊断
- 简单事实查询（我有哪些作品/账号绑定没/数据多少）→ 自己调对应查询工具，快
- 找爆款参考 → 自己调 discover_hot_videos`

const TOOL_LABELS: Record<string, string> = {
  query_brands: '📋 查看人设档案',
  discover_hot_videos: '🔥 搜索同行爆款',
  list_works: '📁 查看作品库',
  query_analytics: '📊 查询作品数据',
  trigger_monitor: '🔍 发起 AI 监测',
  publish_work: '📤 发布操作',
  query_accounts: '🔗 查看账号绑定',
  query_knowledge: '📚 查阅品牌知识库',
  generate_content: '✨ 生成内容',
  api_crawler: '🌐 抓取网页',
  growth_advisor: '🧭 增长诊断',
}

import { useState, useRef, useEffect, useCallback, useMemo } from 'react'
import { Input, Button, Spin, Typography, Space, Modal } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useLocation } from 'react-router-dom'
import LazyChatMarkdown from '../../components/markdown/LazyChatMarkdown'
import { CheckCircleOutlined } from '@ant-design/icons'
import { getToken, useAuthStore } from '../../store/auth'
import { businessApi } from '../../api/business'
import type { ChatMessage } from '../../types/api'
import { ChatQuickActions } from '../../components/chat/ChatQuickActions'
import { AdvisorResultCard } from '../../components/chat/AdvisorResultCard'
import { GenerationTaskCard, parseGenerateContentResult } from '../../components/chat/GenerationTaskCard'

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
  const label = TOOL_LABELS[tool.name] || `🔧 ${tool.name}`
  // 发布计划 → 硬确认卡片（UI 级强制：确认走 REST，与对话链路分离，模型无法伪造）
  const planId = !isRunning && tool.name === 'publish_work'
    ? (String(tool.result || '').match(/plan_id=([a-z0-9.-]+)/) || [])[1] : undefined
  if (planId) return <PublishConfirmCard planId={planId} raw={String(tool.result || '')} />
  if (!isRunning && tool.name === 'growth_advisor') {
    return <AdvisorResultCard raw={String(tool.result || '')} />
  }
  if (!isRunning && tool.name === 'generate_content') {
    const parsed = parseGenerateContentResult(String(tool.result || ''))
    if (parsed.taskId) {
      return <GenerationTaskCard taskId={parsed.taskId} subType={parsed.subType} initialStatus={parsed.status} />
    }
  }
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 8,
      padding: '6px 12px', margin: '6px 0', borderRadius: 10,
      background: 'rgba(94,234,212,0.06)', border: '1px solid rgba(94,234,212,0.15)',
      fontSize: 13, color: isRunning ? 'var(--wr-accent)' : 'var(--wr-text-secondary)',
    }}>
      {isRunning ? <Spin size="small" /> : <CheckCircleOutlined style={{ color: 'var(--wr-success)' }} />}
      <span>{label}{isRunning ? '…' : ''}</span>
    </div>
  )
}

// 发布确认卡片：计划摘要 + 立即确认/定时发布/取消（pending 10 分钟有效）
function PublishConfirmCard({ planId, raw }: { planId: string; raw: string }) {
  const [state, setState] = useState<'pending' | 'done' | 'cancelled' | 'error'>('pending')
  const [jobURL, setJobURL] = useState<string>()
  const [scheduledAt, setScheduledAt] = useState<string>('')
  const [confirming, setConfirming] = useState(false)

  const summary = raw.split('请向用户复述')[0].replace('发布计划已生成（plan_id=' + planId + '）：', '').trim()

  const doConfirm = async () => {
    setConfirming(true)
    try {
      const job = await businessApi.confirmPublishPlan(planId, scheduledAt || undefined)
      setJobURL(job?.external_url)
      setState('done')
    } catch (e: any) {
      setState('error')
    } finally {
      setConfirming(false)
    }
  }
  const doCancel = async () => {
    await businessApi.cancelPublishPlan(planId).catch(() => {})
    setState('cancelled')
  }

  return (
    <div style={{
      margin: '8px 0', padding: 14, borderRadius: 12,
      background: 'rgba(212,165,116,0.08)', border: '1px solid rgba(212,165,116,0.3)',
    }}>
      <Text strong style={{ fontSize: 13 }}>📤 发布确认</Text>
      <div style={{ fontSize: 13, margin: '8px 0', whiteSpace: 'pre-wrap' }}>{summary}</div>
      {state === 'pending' && (
        <Space wrap>
          <Button size="small" type="primary" loading={confirming} onClick={doConfirm}>确认发布</Button>
          <input type="datetime-local" value={scheduledAt} onChange={(e) => setScheduledAt(e.target.value)}
            style={{ fontSize: 12, padding: '2px 6px', borderRadius: 6, border: '1px solid var(--wr-border)', background: 'var(--wr-bg-elevated)', color: 'inherit' }} />
          {scheduledAt && <Text type="secondary" style={{ fontSize: 12 }}>将定时发布</Text>}
          <Button size="small" type="text" danger onClick={doCancel}>取消</Button>
        </Space>
      )}
      {state === 'done' && (
        <Space>
          <CheckCircleOutlined style={{ color: 'var(--wr-success)' }} />
          <Text style={{ fontSize: 13 }}>{scheduledAt ? '已设定时发布' : '发布任务已创建'}</Text>
          {jobURL && <a href={jobURL} target="_blank" rel="noopener noreferrer" style={{ fontSize: 12 }}>打开发布页</a>}
        </Space>
      )}
      {state === 'cancelled' && <Text type="secondary" style={{ fontSize: 13 }}>已取消</Text>}
      {state === 'error' && <Text type="danger" style={{ fontSize: 13 }}>确认失败（计划可能已过期）——请重新发起</Text>}
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
  const [loadedConvMsgs, setLoadedConvMsgs] = useState<Set<string>>(new Set()) // 已加载消息的会话
  const endRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)
  const { username } = useAuthStore()

  // 引擎名单：聊天界面动态切换模型（覆盖 Agent 默认）——仅 name/provider/model，不含厂商密钥

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
        agentName: MASTER_AGENT,
        messages: [userMsg, aMsg],
        createdAt: Date.now(),
        updatedAt: Date.now(),
      }
      setConversations(prev => [newConv, ...prev])
      setCurrentConvId(convId)
      setLoadedConvMsgs(prev => new Set(prev).add(convId!)) // 新会话无需再加载
      // 后端持久化：创建会话
      businessApi.createConversation({ id: convId!, title: newConv.title, agent_name: MASTER_AGENT }).catch(() => {})
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
    if (!isNewConv) {
      setConversations(prev => prev.map(c => c.id === convId ? { ...c, agentName: MASTER_AGENT } : c))
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
          system_message: MASTER_AGENT_PROMPT,
          tools: [], // 空数组=后端使用全部已启用的工具（toolRegistry.All()）
          use_tools: true, // 启用工具模式（ReAct Agent + 爬虫工具调用）
          llm_config_name: '', // 聊天界面可临时覆盖 Agent 默认 LLM
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

              // 防御性收尾：流结束（finish/error）时仍在转圈的工具块自动完成——
              // SSE 中间层（代理缓冲/断连）可能丢尾部 tool-result 事件，不收尾会永远转
              if (e.type === 'finish' || e.type === 'error') {
                for (const b of blocks) {
                  if (b.type === 'tool' && b.tool && !b.tool.result) b.tool.result = '(已完成)'
                }
                for (const t of newTools) { if (!t.result) t.result = '(已完成)' }
                for (const t of accTools) { if (!t.result) t.result = '(已完成)' }
              }

              msgs[msgs.length - 1] = { ...oldMsg, content: newContent, tools: newTools, blocks }
              return msgs
            })
          } catch {}
        }
      }
      // 断连兜底：连接结束但未收到 finish（代理截断丢尾部事件）——未完成工具块收尾
      for (const t of accTools) { if (!t.result) t.result = '(已完成)' }
      patchConvMessages(convIdRef, cmsgs => {
        const msgs = [...cmsgs]
        const lastMsg = msgs[msgs.length - 1]
        if (lastMsg?.blocks) {
          msgs[msgs.length - 1] = {
            ...lastMsg,
            blocks: lastMsg.blocks.map(b => b.tool && !b.tool.result ? { ...b, tool: { ...b.tool, result: '(已完成)' } } : b),
          }
        }
        return msgs
      })

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
            <Text strong style={{ fontSize: 14 }}>🤖 {MASTER_AGENT}</Text>
            <Text type="secondary" style={{ fontSize: 12 }}>对话即可：查数据 · 找爆款 · 做内容 · 发作品</Text>
          </Space>
        </div>

        {/* 对话内容 */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '16px 0' }}>
          {messages.length === 0 ? (
            <div style={{ textAlign: 'center', paddingTop: 60 }}>
              <div style={{ width: 56, height: 56, borderRadius: 14, background: 'linear-gradient(135deg, var(--wr-primary), var(--wr-accent))', margin: '0 auto 20px', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 26, color: '#fff', boxShadow: '0 0 32px var(--wr-primary-bg)' }}>{'🤖'}</div>
              <Text style={{ fontSize: 20, fontWeight: 600, display: 'block', marginBottom: 8 }}>与获客管家对话</Text>
              <Text type="secondary" style={{ fontSize: 15 }}>回车发送 · Shift+回车换行 · 工具自动调用</Text>
            </div>
          ) : (
            <div style={{ maxWidth: 1080, margin: '0 auto', padding: '0 24px' }}>
              {messages.map(m => {
                const isLA = m.role === 'assistant' && m.id === messages[messages.length - 1]?.id && streaming
                const isUser = m.role === 'user'
                return (
                  <div key={m.id} style={{ marginBottom: 20, display: 'flex', justifyContent: isUser ? 'flex-end' : 'flex-start', alignItems: 'flex-start' }}>
                    {!isUser && <div style={{ width: 36, height: 36, borderRadius: 8, flexShrink: 0, marginRight: 12, background: 'linear-gradient(135deg, var(--wr-primary), var(--wr-accent))', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 14, fontWeight: 700, color: '#fff' }}>{'🤖'}</div>}
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
          <div style={{ maxWidth: 1080, margin: '0 auto 8px' }}>
            <ChatQuickActions disabled={streaming} />
          </div>
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
