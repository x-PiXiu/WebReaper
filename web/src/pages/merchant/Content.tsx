import { useState } from 'react'
import { Card, Typography, Button, Input, Select, Space, message, Empty, Tag, Row, Col, Spin } from 'antd'
import { FileTextOutlined, FileSearchOutlined, ClearOutlined, EditOutlined, ThunderboltOutlined, ExportOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { getToken } from '../../store/auth'
import type { Brand, Keyword, OptimizedContent } from '../../types/api'

const { Text, Paragraph } = Typography
const { TextArea } = Input

// 评分 → 颜色（沿用项目 token，双主题自适应）
function scoreColor(s: number): string {
  if (s >= 80) return 'var(--wr-success)'
  if (s >= 65) return 'var(--wr-accent)'
  if (s >= 50) return 'var(--wr-warning)'
  return 'var(--wr-danger)'
}
// 评分 → 等级文案
function scoreLevel(s: number): string {
  if (s >= 80) return 'A 优秀'
  if (s >= 65) return 'B 良好'
  if (s >= 50) return 'C 及格'
  return 'D 待优化'
}

// GEO 5 维度配置（标签 + 取值 key）
const DIMENSIONS: { label: string; key: keyof OptimizedContent['score'] }[] = [
  { label: '权威性', key: 'authority' },
  { label: '具体性', key: 'specificity' },
  { label: '结构化', key: 'structure' },
  { label: '独特性', key: 'uniqueness' },
  { label: '时效性', key: 'recency' },
]

// 目标 AI 引擎偏好（GEO 优化侧差异：按引擎偏好调整内容格式，提高被引用概率）
const ENGINE_OPTIONS = [
  { value: '', label: '通用（不指定）' },
  { value: 'chatgpt', label: 'ChatGPT' },
  { value: 'perplexity', label: 'Perplexity' },
  { value: 'kimi', label: 'Kimi' },
  { value: 'doubao', label: '豆包' },
]

export default function Content() {
  const queryClient = useQueryClient()
  const [selectedBrand, setSelectedBrand] = useState<string | undefined>()
  const [originalText, setOriginalText] = useState('')
  const [optimizing, setOptimizing] = useState(false)
  const [result, setResult] = useState<OptimizedContent | null>(null)
  const [genKeywords, setGenKeywords] = useState<string[]>([])
  const [targetEngine, setTargetEngine] = useState<string>('') // 目标 AI 引擎偏好
  const [generating, setGenerating] = useState(false)
  const [drafting, setDrafting] = useState(false)

  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })
  const { data: keywords = [] } = useQuery({
    queryKey: ['geo-keywords', selectedBrand],
    queryFn: () => businessApi.listKeywords(selectedBrand!),
    enabled: !!selectedBrand,
  })
  const { data: contents = [] } = useQuery({
    queryKey: ['geo-contents', selectedBrand],
    queryFn: () => businessApi.listContents(selectedBrand!),
    enabled: !!selectedBrand,
  })

  // 内容优化（有原始内容时调用）
  const handleOptimize = async () => {
    if (!selectedBrand || !originalText.trim() || genKeywords.length === 0) {
      message.warning('请选择品牌、关键词和原始内容')
      return
    }
    setOptimizing(true)
    setResult(null)
    try {
      const res = await businessApi.optimizeContent({
        brand_id: selectedBrand,
        keyword: genKeywords[0],
        original_text: originalText,
        target_engine: targetEngine || undefined,
      })
      setResult(res)
      message.success('优化完成')
      queryClient.invalidateQueries({ queryKey: ['geo-contents', selectedBrand] })
    } catch {
    } finally {
      setOptimizing(false)
    }
  }

  // 从零生成内容（SSE 流式：用户实时看到文章逐字输出）
  const handleGenerate = async () => {
    if (!selectedBrand || genKeywords.length === 0) {
      message.warning('请选择品牌并至少选择一个关键词')
      return
    }
    setGenerating(true)
    setResult(null)
    let accText = ''
    setResult({ optimized_text: '', score: { total: 0, authority: 0, specificity: 0, structure: 0, uniqueness: 0, recency: 0 } } as OptimizedContent)

    try {
      const brandInfo = brands.find((b: Brand) => b.id === selectedBrand)
      const token = getToken()
      const res = await fetch(`/api/v1/geo/brands/${selectedBrand}/contents/generate-stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: JSON.stringify({
          keywords: genKeywords,
          brand_info: brandInfo ? `${brandInfo.name}：${brandInfo.positioning || ''}` : '',
          target_engine: targetEngine || undefined,
        }),
      })
      if (res.status === 401) { message.error('登录已过期'); setGenerating(false); return }
      if (!res.ok) throw new Error(`HTTP ${res.status}`)

      const reader = res.body?.getReader()
      const dec = new TextDecoder()
      let buf = ''
      while (reader) {
        const { done, value } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() || ''
        for (const ln of lines) {
          if (!ln.startsWith('data: ')) continue
          try {
            const e = JSON.parse(ln.slice(6))
            if (e.type === 'text-delta' && e.textDelta) {
              accText += e.textDelta
              setResult({ optimized_text: accText, score: { total: 0, authority: 0, specificity: 0, structure: 0, uniqueness: 0, recency: 0 } } as OptimizedContent)
            } else if (e.type === 'result' && e.data) {
              setResult(e.data)
            } else if (e.type === 'error') {
              message.error(e.error || '生成失败')
            }
          } catch {}
        }
      }
      const modeLabel = genKeywords.length > 1 ? `（${genKeywords.length} 个关键词组合）` : ''
      message.success(`内容生成成功${modeLabel}`)
      queryClient.invalidateQueries({ queryKey: ['geo-contents', selectedBrand] })
    } catch (e) {
      message.error('生成失败：' + ((e as Error)?.message || ''))
    } finally {
      setGenerating(false)
    }
  }

  // 内容状态流转：发布到公开站 / 下线
  const handleSetStatus = async (c: OptimizedContent, status: 'draft' | 'published') => {
    try {
      await businessApi.setContentStatus(selectedBrand!, c.id, status)
      if (status === 'published') {
        // 收录预期管理：发布后 IndexNow 立即通知，引擎爬取+引用约 1-2 周
        message.success(`「${c.title || c.id}」已发布到公开站（已通知搜索引擎收录，预计 1-2 周生效，届时可复测提及率）`, 5)
      } else {
        message.success('已下线')
      }
      queryClient.invalidateQueries({ queryKey: ['geo-contents', selectedBrand] })
      if (result?.id === c.id) setResult({ ...result, status })
    } catch (e) {
      message.error('状态变更失败：' + ((e as Error)?.message || ''))
    }
  }

  const score = result?.score

  // AI 生成原始素材（填入编辑区，用户可编辑后再优化）
  const handleGenerateDraft = async () => {
    if (!selectedBrand || genKeywords.length === 0) {
      message.warning('请先选择品牌和关键词')
      return
    }
    setDrafting(true)
    try {
      const brandInfo = brands.find((b: Brand) => b.id === selectedBrand)
      const res = await businessApi.generateContent(selectedBrand, {
        keywords: [genKeywords[0]],
        brand_info: brandInfo ? `${brandInfo.name}：${brandInfo.positioning || ''}` : '',
        target_engine: targetEngine || undefined,
      })
      setOriginalText(res.optimized_text || '')
      message.success('素材已生成，你可以编辑后点击优化')
    } catch (e) {
      message.error('生成素材失败：' + ((e as Error)?.message || ''))
    } finally {
      setDrafting(false)
    }
  }

  const busy = optimizing || generating
  const hasContent = originalText.trim().length > 0

  // 统一操作
  const handleAction = () => {
    if (hasContent) {
      handleOptimize()
    } else {
      handleGenerate()
    }
  }

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* ① Hero 区 */}
        <div className="wr-page-header">
          <h1>内容工作台</h1>
          <p>AI 原创生成与 GEO 优化，提升品牌在 AI 搜索引擎中的可见度</p>
        </div>

        {/* ② 品牌上下文条 */}
        <Card className="wr-glass-card" styles={{ body: { padding: '16px 20px' } }} style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, flex: '1 1 280px' }}>
              <Text strong style={{ whiteSpace: 'nowrap' }}>当前品牌</Text>
              <Select
                style={{ maxWidth: 320, minWidth: 200, flex: 1 }}
                placeholder="选择要创作/优化的品牌"
                value={selectedBrand}
                onChange={(v) => { setSelectedBrand(v); setGenKeywords([]) }}
                options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
              />
            </div>
            {selectedBrand && (
              <Space size={24}>
                <ContextStat icon={<FileSearchOutlined />} value={keywords.length} label="关键词" />
                <ContextStat icon={<FileTextOutlined />} value={contents.length} label="历史内容" />
              </Space>
            )}
          </div>
        </Card>

        {/* ③ 主工作区：左输入 / 右结果 */}
        <Row gutter={16}>
          {/* 左：输入面板 */}
          <Col xs={24} lg={12}>
            <Card styles={{ body: { padding: 24 } }}>
              <div className="wr-stagger">
                {/* 关键词选择 */}
                <div style={{ marginBottom: 20 }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                    <Text strong>关键词</Text>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {genKeywords.length > 0 ? `已选 ${genKeywords.length} 个` : '可多选组合'}
                    </Text>
                  </div>
                  <Select
                    mode="multiple"
                    style={{ width: '100%' }}
                    placeholder="选择关键词（可多选，多选=组合成一篇深度文）"
                    value={genKeywords}
                    onChange={setGenKeywords}
                    options={keywords.map((k: Keyword) => ({ value: k.term, label: k.term }))}
                    disabled={!selectedBrand}
                    maxTagCount={5}
                  />
                </div>

                {/* 目标引擎偏好选择 */}
                <div style={{ marginBottom: 20 }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                    <Text strong>目标 AI 引擎</Text>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      按引擎偏好优化格式，提高被引用概率
                    </Text>
                  </div>
                  <Select
                    style={{ width: '100%' }}
                    value={targetEngine || undefined}
                    onChange={(v) => setTargetEngine(v || '')}
                    options={ENGINE_OPTIONS}
                    allowClear
                    placeholder="选择目标引擎（空=通用优化）"
                  />
                </div>

                {/* 内容编辑区——美化版 */}
                <div style={{ marginBottom: 20 }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                    <Space size={6}>
                      <Text strong>
                        {hasContent ? '原始内容' : '内容编辑区'}
                      </Text>
                      {hasContent ? (
                        <Tag color="blue" style={{ margin: 0, fontSize: 11 }}>优化模式</Tag>
                      ) : (
                        <Tag color="green" style={{ margin: 0, fontSize: 11 }}>原创生成</Tag>
                      )}
                    </Space>
                    <Space size={8}>
                      <Button
                        size="small"
                        type="link"
                        loading={drafting}
                        disabled={!selectedBrand || genKeywords.length === 0}
                        onClick={handleGenerateDraft}
                      >
                        AI 生成素材
                      </Button>
                      {hasContent && (
                        <Button
                          size="small"
                          type="text"
                          icon={<ClearOutlined />}
                          onClick={() => setOriginalText('')}
                        >
                          清空
                        </Button>
                      )}
                    </Space>
                  </div>

                  {/* 模式提示条 */}
                  <div style={{
                    marginBottom: 8, padding: '6px 12px', borderRadius: 8,
                    background: hasContent ? 'var(--wr-primary-bg)' : 'var(--wr-bg-elevated)',
                    display: 'flex', alignItems: 'center', gap: 6,
                  }}>
                    {hasContent ? (
                      <>
                        <EditOutlined style={{ fontSize: 13, color: 'var(--wr-primary)' }} />
                        <Text style={{ fontSize: 12, color: 'var(--wr-primary)' }}>
                          已填入内容，点击下方按钮将进行 GEO 优化
                        </Text>
                      </>
                    ) : (
                      <>
                        <ThunderboltOutlined style={{ fontSize: 13, color: 'var(--wr-success)' }} />
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          编辑区留空，点击下方按钮让 AI 从零原创生成
                        </Text>
                      </>
                    )}
                  </div>

                  <TextArea
                    rows={8}
                    placeholder={drafting
                      ? 'AI 正在生成素材...'
                      : '留空 = AI 根据关键词原创生成\n贴入内容 = AI 围绕关键词优化已有内容\n\n你也可以点右上角「AI 生成素材」让 AI 先写草稿，编辑后再优化'
                    }
                    value={originalText}
                    onChange={(e) => setOriginalText(e.target.value)}
                    style={{ fontSize: 14, lineHeight: 1.8, resize: 'vertical' }}
                    showCount
                    maxLength={20000}
                  />
                </div>

                {/* 统一操作按钮 */}
                <Button
                  type="primary"
                  size="large"
                  block
                  loading={busy}
                  disabled={genKeywords.length === 0}
                  onClick={handleAction}
                  icon={hasContent ? <EditOutlined /> : <ThunderboltOutlined />}
                >
                  {busy
                    ? 'AI 创作中...'
                    : hasContent
                    ? '开始 GEO 优化'
                    : `生成内容${genKeywords.length > 1 ? `（${genKeywords.length} 词组合）` : ''}`}
                </Button>
              </div>
            </Card>
          </Col>

          {/* 右：结果面板（GEO 评分仪表盘）*/}
          <Col xs={24} lg={12}>
            <Card title="优化结果" styles={{ body: { padding: 24 } }} style={{ minHeight: '100%' }}>
              {busy ? (
                <div style={{ textAlign: 'center', padding: '60px 20px' }}>
                  <Spin size="large" />
                  <Paragraph type="secondary" style={{ marginTop: 16, marginBottom: 0 }}>
                    {generating ? 'AI 正在创作内容，文章会逐字显示...' : 'AI 正在优化内容并评分...'}
                  </Paragraph>
                </div>
              ) : result ? (
                <>
                  {/* GEO 评分仪表盘 */}
                  {score && score.total > 0 && (
                    <div style={{ marginBottom: 20 }}>
                      <Row gutter={16} align="middle">
                        <Col flex="auto">
                          <ScoreRing total={score.total} />
                        </Col>
                        <Col flex="auto">
                          <Space direction="vertical" size={10} style={{ width: '100%' }}>
                            {DIMENSIONS.map((d) => (
                              <ScoreBar key={d.key} label={d.label} value={score[d.key]} />
                            ))}
                          </Space>
                        </Col>
                      </Row>
                    </div>
                  )}

                  {/* 前后对比反馈（仅优化模式：后端返回 score_before + recommendations） */}
                  {result.score_before && (
                    <div style={{ marginBottom: 20 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
                        <Text strong style={{ fontSize: 14 }}>优化前后对比</Text>
                        <Tag color="blue" style={{ margin: 0, fontSize: 11 }}>
                          {result.score_before.total?.toFixed(0)} → {score?.total?.toFixed(0)}
                        </Tag>
                      </div>
                      <Space direction="vertical" size={8} style={{ width: '100%' }}>
                        {DIMENSIONS.map((d) => {
                          const before = result.score_before?.[d.key] ?? 0
                          const after = score?.[d.key] ?? 0
                          const diff = after - before
                          const diffColor = diff > 5 ? 'var(--wr-success)' : diff < -5 ? 'var(--wr-danger)' : 'var(--wr-text-muted)'
                          return (
                            <div key={d.key}>
                              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, marginBottom: 4 }}>
                                <Text type="secondary">{d.label}</Text>
                                <Text style={{ fontSize: 12 }}>
                                  <Text type="secondary" style={{ fontSize: 12 }}>{before.toFixed(0)}</Text>
                                  <Text strong style={{ color: diffColor, fontSize: 12 }}> → {after.toFixed(0)}</Text>
                                </Text>
                              </div>
                              {/* 双条对比：before 灰条 + after 彩条 */}
                              <div style={{ position: 'relative', height: 6, background: 'var(--wr-bg-elevated)', borderRadius: 3, overflow: 'hidden' }}>
                                <div style={{
                                  position: 'absolute', top: 0, left: 0, bottom: 0,
                                  width: `${Math.max(0, Math.min(100, before))}%`,
                                  background: 'var(--wr-text-muted)',
                                  opacity: 0.35,
                                }} />
                                <div style={{
                                  position: 'absolute', top: 0, left: 0, bottom: 0,
                                  width: `${Math.max(0, Math.min(100, after))}%`,
                                  background: scoreColor(after),
                                  borderRadius: 3,
                                  transition: 'width 600ms cubic-bezier(0.2, 0, 0, 1)',
                                }} />
                              </div>
                            </div>
                          )
                        })}
                      </Space>

                      {/* 改进建议 */}
                      {result.recommendations && result.recommendations.length > 0 && (
                        <div style={{ marginTop: 16, padding: '12px 16px', background: 'var(--wr-primary-bg)', borderRadius: 10 }}>
                          <Text strong style={{ fontSize: 13, color: 'var(--wr-primary)' }}>改进建议</Text>
                          <ul style={{ margin: '8px 0 0', paddingLeft: 18, fontSize: 13, lineHeight: 1.8, color: 'var(--wr-text-secondary)' }}>
                            {result.recommendations.map((r, i) => (
                              <li key={i}>{r}</li>
                            ))}
                          </ul>
                        </div>
                      )}
                    </div>
                  )}

                  <Paragraph style={{ whiteSpace: 'pre-wrap', lineHeight: 1.8, margin: 0 }}>
                    {result.optimized_text}
                  </Paragraph>

                  {/* 公开链接（AI 引擎可爬取的公开文章页——发布为 published 后生效） */}
                  {result.id && (
                    <div style={{ marginTop: 16, padding: '10px 14px', background: 'var(--wr-bg-elevated)', borderRadius: 8, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                      {result.status === 'published' ? (
                        <>
                          <Tag color="success" style={{ margin: 0 }}>已发布</Tag>
                          <Text code style={{ fontSize: 12 }}>/public/articles/{result.id}</Text>
                          <Button size="small" type="link" icon={<ExportOutlined />} href={`/public/articles/${result.id}`} target="_blank" style={{ fontSize: 12 }}>
                            查看公开页
                          </Button>
                          <Button size="small" type="text" danger style={{ fontSize: 12 }} onClick={() => handleSetStatus(result, 'draft')}>
                            下线
                          </Button>
                        </>
                      ) : (
                        <>
                          <Text type="secondary" style={{ fontSize: 12 }}>草稿——发布后 AI 引擎可爬取此内容</Text>
                          <Button size="small" type="primary" style={{ fontSize: 12 }} onClick={() => handleSetStatus(result, 'published')}>
                            发布到公开站
                          </Button>
                        </>
                      )}
                    </div>
                  )}
                </>
              ) : (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="选择关键词后点击生成，结果会显示在这里"
                  style={{ padding: 60 }}
                />
              )}
            </Card>
          </Col>
        </Row>

        {/* ④ 历史内容卡片网格 */}
        {selectedBrand && contents.length > 0 && (
          <Card title="历史内容" style={{ marginTop: 16 }}>
            <Row gutter={[16, 16]} className="wr-stagger">
              {contents.map((c: OptimizedContent) => {
                const total = c.score?.total || 0
                return (
                  <Col xs={24} sm={12} lg={8} key={c.id}>
                    <Card
                      size="small"
                      hoverable
                      style={{ height: '100%' }}
                      styles={{ body: { padding: 16, height: '100%', display: 'flex', flexDirection: 'column', gap: 10 } }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <Space size={8}>
                          <Tag color={scoreColor(total)} style={{ margin: 0, fontWeight: 600 }}>
                            GEO {total.toFixed(0)}
                          </Tag>
                          <Text type="secondary" style={{ fontSize: 12 }}>{scoreLevel(total)}</Text>
                          {c.status === 'published' ? (
                            <Tag color="success" style={{ margin: 0, fontSize: 11 }}>已发布</Tag>
                          ) : (
                            <Tag style={{ margin: 0, fontSize: 11 }}>草稿</Tag>
                          )}
                        </Space>
                        <Text type="secondary" style={{ fontSize: 12 }}>v{c.version}</Text>
                      </div>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {new Date(c.created_at).toLocaleString()}
                      </Text>
                      <Text strong ellipsis style={{ fontSize: 13 }}>
                        {c.title || '(无标题)'}
                      </Text>
                      <Paragraph
                        ellipsis={{ rows: 2 }}
                        style={{ margin: 0, color: 'var(--wr-text-secondary)', fontSize: 13, lineHeight: 1.6 }}
                      >
                        {c.optimized_text}
                      </Paragraph>
                      <div style={{ display: 'flex', gap: 4, marginTop: 'auto', paddingTop: 4 }}>
                        {c.status === 'published' ? (
                          <>
                            <Button size="small" type="link" icon={<ExportOutlined />} href={`/public/articles/${c.id}`} target="_blank" style={{ fontSize: 12 }}>
                              公开页
                            </Button>
                            <Button size="small" type="text" danger style={{ fontSize: 12 }} onClick={() => handleSetStatus(c, 'draft')}>
                              下线
                            </Button>
                          </>
                        ) : (
                          <Button size="small" type="primary" ghost style={{ fontSize: 12 }} onClick={() => handleSetStatus(c, 'published')}>
                            发布到公开站
                          </Button>
                        )}
                      </div>
                    </Card>
                  </Col>
                )
              })}
            </Row>
          </Card>
        )}
      </div>
    </div>
  )
}

/* ===== 纯展示子组件（谦卑对象：只负责渲染，无业务逻辑）===== */

// 上下文条小统计
function ContextStat({ icon, value, label }: { icon: React.ReactNode; value: number; label: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <span style={{ color: 'var(--wr-text-muted)', fontSize: 16 }}>{icon}</span>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
        <Text strong style={{ fontSize: 18 }}>{value}</Text>
        <Text type="secondary" style={{ fontSize: 12 }}>{label}</Text>
      </div>
    </div>
  )
}

// GEO 总分进度环（自绘 SVG，颜色随分数变化）
function ScoreRing({ total }: { total: number }) {
  const color = scoreColor(total)
  const radius = 52
  const circ = 2 * Math.PI * radius
  const pct = Math.max(0, Math.min(100, total)) / 100
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}>
      <div style={{ position: 'relative', width: 128, height: 128 }}>
        <svg width="128" height="128" viewBox="0 0 128 128" style={{ transform: 'rotate(-90deg)' }}>
          <circle cx="64" cy="64" r={radius} fill="none" stroke="var(--wr-bg-elevated)" strokeWidth="10" />
          <circle
            cx="64" cy="64" r={radius} fill="none" stroke={color} strokeWidth="10" strokeLinecap="round"
            strokeDasharray={circ}
            strokeDashoffset={circ * (1 - pct)}
            style={{ transition: 'stroke-dashoffset 600ms cubic-bezier(0.2, 0, 0, 1), stroke 300ms ease' }}
          />
        </svg>
        <div style={{
          position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column',
          alignItems: 'center', justifyContent: 'center',
        }}>
          <span className="wr-metric-value" style={{ fontSize: 34, color }}>{total.toFixed(0)}</span>
          <Text type="secondary" style={{ fontSize: 12 }}>GEO 总分</Text>
        </div>
      </div>
      <Tag color={color} style={{ fontWeight: 600 }}>{scoreLevel(total)}</Tag>
    </div>
  )
}

// 单维度评分横条
function ScoreBar({ label, value }: { label: string; value: number }) {
  const color = scoreColor(value)
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13, marginBottom: 4 }}>
        <Text type="secondary">{label}</Text>
        <Text strong style={{ color }}>{value.toFixed(0)}</Text>
      </div>
      <div style={{ height: 6, background: 'var(--wr-bg-elevated)', borderRadius: 3, overflow: 'hidden' }}>
        <div style={{
          height: '100%',
          width: `${Math.max(0, Math.min(100, value))}%`,
          background: color,
          borderRadius: 3,
          transition: 'width 600ms cubic-bezier(0.2, 0, 0, 1), background 300ms ease',
        }} />
      </div>
    </div>
  )
}
