import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Button, Input, Radio, Select, Space, Steps, Tag, Typography, message, Progress,
} from 'antd'
import {
  ArrowLeftOutlined, ArrowRightOutlined, CheckOutlined, RocketOutlined, SendOutlined,
} from '@ant-design/icons'
import { MOCK_AVATARS, MOCK_VOICES, MOCK_STORYBOARDS, MOCK_COVERS } from '../../../mock/ipAssets'
import { useWorksStore } from '../../../store/works'
import { useBrands } from '../../../hooks/useBrands'

const { Text, Title, Paragraph } = Typography
const { TextArea } = Input

const STEPS = [
  { title: '选题', desc: '定人设与钩子' },
  { title: '分镜', desc: '编排镜头' },
  { title: '配音', desc: '选音色' },
  { title: '合成', desc: '生成成片' },
  { title: '封面', desc: '定封面' },
  { title: '发布', desc: '入库 / 分发' },
]

/**
 * 成片分步向导（演示）：选题 → 分镜 → 配音 → 合成 → 封面 → 发布入作品库。
 * embedded：嵌在内容合成 Tabs 内时隐藏外层页头。
 */
export default function ComposeWizard({ embedded }: { embedded?: boolean }) {
  const navigate = useNavigate()
  const upsertWork = useWorksStore((s) => s.upsertWork)
  const markPublished = useWorksStore((s) => s.markPublished)
  const { data: brands = [] } = useBrands()

  const [step, setStep] = useState(0)
  const [topic, setTopic] = useState('')
  const [hook, setHook] = useState('')
  const [personaId, setPersonaId] = useState(MOCK_AVATARS[0]?.id)
  const [boardIds, setBoardIds] = useState<string[]>([MOCK_STORYBOARDS[0].id, MOCK_STORYBOARDS[1].id])
  const [voiceId, setVoiceId] = useState(MOCK_VOICES[0]?.id)
  const [coverId, setCoverId] = useState(MOCK_COVERS[0]?.id)
  const [brandId, setBrandId] = useState<string>()
  const [composing, setComposing] = useState(false)
  const [composePct, setComposePct] = useState(0)
  const [composed, setComposed] = useState(false)
  const [workId, setWorkId] = useState<string>()
  const [platform, setPlatform] = useState('抖音')

  const cover = MOCK_COVERS.find((c) => c.id === coverId)
  const canNext = useMemo(() => {
    if (step === 0) return topic.trim().length >= 4 && !!personaId
    if (step === 1) return boardIds.length > 0
    if (step === 2) return !!voiceId
    if (step === 3) return composed
    if (step === 4) return !!coverId
    return true
  }, [step, topic, personaId, boardIds, voiceId, composed, coverId])

  const runCompose = async () => {
    setComposing(true)
    setComposePct(0)
    setComposed(false)
    for (let p = 0; p <= 100; p += 8) {
      await new Promise((r) => setTimeout(r, 90))
      setComposePct(p)
    }
    const id = `wk-${Date.now()}`
    const accent = cover?.accent || '#0f766e'
    upsertWork({
      id,
      title: topic.trim(),
      coverAccent: accent,
      status: 'ready',
      createdAt: new Date().toISOString(),
      durationSec: boardIds.length * 8 + 6,
      views: 0,
      likes: 0,
      comments: 0,
      leads: 0,
      brandId,
      source: 'wizard',
    })
    setWorkId(id)
    setComposed(true)
    setComposing(false)
    message.success('成片已生成（演示），可继续选封面')
  }

  const finishToLibrary = (publish: boolean) => {
    if (!workId) {
      message.warning('请先完成合成')
      return
    }
    if (publish) {
      markPublished(workId, platform)
      message.success('已写入作品库（演示发布）。真实分发请去发布中心')
      navigate('/m/works')
      return
    }
    message.success('已保存到我的作品（演示）')
    navigate('/m/works')
  }

  return (
    <div className={embedded ? undefined : 'wr-page-content ip-page'}>
      {!embedded && (
        <div className="ip-page-hero">
          <div>
            <p className="ip-kicker">Wizard</p>
            <h1>成片向导</h1>
            <p className="ip-lead">分步打造账号 IP 成片（演示）——真实生成请用「写文章 / 做视频图片」</p>
          </div>
        </div>
      )}
      {embedded && (
        <Paragraph type="secondary" style={{ marginBottom: 12 }}>
          演示流程：分镜 / 配音 / 合成不会调用真实生成 API。正式创作请切到「做视频图片」或「写文章」。
        </Paragraph>
      )}

      <div className="ip-wizard-shell">
        <Steps
          current={step}
          size="small"
          className="ip-wizard-steps"
          items={STEPS.map((s) => ({ title: s.title, description: s.desc }))}
        />

        <div className="ip-wizard-body ip-rise">
          {step === 0 && (
            <section className="ip-wizard-panel">
              <Title level={4}>选题与人设</Title>
              <Paragraph type="secondary">先写清钩子，再锁定形象——后面步骤都会围绕它展开。</Paragraph>
              <div className="ip-form-stack">
                <label>关联人设档案（可选）</label>
                <Select
                  allowClear
                  placeholder="选择品牌 / 人设"
                  value={brandId}
                  onChange={setBrandId}
                  options={brands.map((b) => ({ value: b.id, label: b.name }))}
                />
                <label>作品主题</label>
                <Input
                  size="large"
                  placeholder="例如：三天打造个人 IP 开场片"
                  value={topic}
                  onChange={(e) => setTopic(e.target.value)}
                  maxLength={48}
                  showCount
                />
                <label>开场钩子</label>
                <TextArea
                  rows={3}
                  placeholder="例如：还在靠发朋友圈获客？试试这条账号打法……"
                  value={hook}
                  onChange={(e) => setHook(e.target.value)}
                />
                <label>选用形象</label>
                <div className="ip-pick-grid">
                  {MOCK_AVATARS.map((a) => (
                    <button
                      key={a.id}
                      type="button"
                      className={`ip-pick-card${personaId === a.id ? ' is-active' : ''}`}
                      onClick={() => setPersonaId(a.id)}
                    >
                      <span className="ip-pick-swatch" style={{ background: a.cover }} />
                      <strong>{a.name}</strong>
                      <span>{a.style}</span>
                    </button>
                  ))}
                </div>
              </div>
            </section>
          )}

          {step === 1 && (
            <section className="ip-wizard-panel">
              <Title level={4}>编排分镜</Title>
              <Paragraph type="secondary">勾选镜头顺序（演示版按点击顺序入列）。</Paragraph>
              <div className="ip-pick-grid">
                {MOCK_STORYBOARDS.map((s) => {
                  const on = boardIds.includes(s.id)
                  return (
                    <button
                      key={s.id}
                      type="button"
                      className={`ip-pick-card${on ? ' is-active' : ''}`}
                      onClick={() =>
                        setBoardIds((prev) =>
                          prev.includes(s.id) ? prev.filter((x) => x !== s.id) : [...prev, s.id],
                        )}
                    >
                      <Tag>{s.ratio}</Tag>
                      <strong>{s.title}</strong>
                      <span>{s.scene} · {s.durationSec}s</span>
                    </button>
                  )
                })}
              </div>
              <Text type="secondary">已选 {boardIds.length} 镜</Text>
            </section>
          )}

          {step === 2 && (
            <section className="ip-wizard-panel">
              <Title level={4}>配音音色</Title>
              <Radio.Group
                value={voiceId}
                onChange={(e) => setVoiceId(e.target.value)}
                className="ip-voice-list"
              >
                {MOCK_VOICES.map((v) => (
                  <Radio.Button key={v.id} value={v.id} className="ip-voice-item">
                    <strong>{v.name}</strong>
                    <span>{v.tone} · {v.lang}</span>
                  </Radio.Button>
                ))}
              </Radio.Group>
            </section>
          )}

          {step === 3 && (
            <section className="ip-wizard-panel">
              <Title level={4}>合成成片</Title>
              <Paragraph type="secondary">演示合成进度。真实渲染将对接后端生成管线。</Paragraph>
              <div className="ip-compose-stage">
                <div
                  className="ip-compose-preview"
                  style={{ background: cover ? `linear-gradient(160deg,#0b0b10,${cover.accent}66)` : undefined }}
                >
                  <Text strong style={{ fontSize: 18 }}>{topic || '未命名作品'}</Text>
                  <Text type="secondary">{hook || '等待合成…'}</Text>
                </div>
                {(composing || composed) && (
                  <Progress percent={composePct} status={composed ? 'success' : 'active'} />
                )}
                <Button
                  type="primary"
                  size="large"
                  className="ip-btn-primary"
                  icon={<RocketOutlined />}
                  loading={composing}
                  onClick={runCompose}
                  disabled={composed}
                >
                  {composed ? '已合成' : '开始合成'}
                </Button>
              </div>
            </section>
          )}

          {step === 4 && (
            <section className="ip-wizard-panel">
              <Title level={4}>选择封面</Title>
              <div className="ip-pick-grid">
                {MOCK_COVERS.map((c) => (
                  <button
                    key={c.id}
                    type="button"
                    className={`ip-pick-card${coverId === c.id ? ' is-active' : ''}`}
                    onClick={() => setCoverId(c.id)}
                  >
                    <span
                      className="ip-pick-swatch"
                      style={{ background: `linear-gradient(160deg,#111,${c.accent})` }}
                    />
                    <strong>{c.name}</strong>
                    <span>{c.mood} · {c.ratio}</span>
                  </button>
                ))}
              </div>
            </section>
          )}

          {step === 5 && (
            <section className="ip-wizard-panel">
              <Title level={4}>入库与发布</Title>
              <Paragraph type="secondary">
                保存即进入「我的作品」。「发布」仅为演示状态；真实账号分发请走发布中心。
              </Paragraph>
              <div className="ip-form-stack">
                <label>目标平台（演示）</label>
                <Select
                  value={platform}
                  onChange={setPlatform}
                  options={[
                    { value: '抖音', label: '抖音' },
                    { value: '视频号', label: '视频号' },
                    { value: '快手', label: '快手' },
                  ]}
                />
              </div>
              <Space wrap style={{ marginTop: 20 }}>
                <Button size="large" icon={<CheckOutlined />} onClick={() => finishToLibrary(false)}>
                  仅保存到作品库
                </Button>
                <Button
                  type="primary"
                  size="large"
                  className="ip-btn-primary"
                  icon={<SendOutlined />}
                  onClick={() => finishToLibrary(true)}
                >
                  发布并入库
                </Button>
                <Button type="link" onClick={() => navigate('/m/distribution')}>
                  去发布中心绑定账号
                </Button>
              </Space>
            </section>
          )}
        </div>

        <div className="ip-wizard-footer">
          <Button
            icon={<ArrowLeftOutlined />}
            disabled={step === 0 || composing}
            onClick={() => setStep((s) => Math.max(0, s - 1))}
          >
            上一步
          </Button>
          {step < STEPS.length - 1 ? (
            <Button
              type="primary"
              className="ip-btn-primary"
              icon={<ArrowRightOutlined />}
              disabled={!canNext || composing}
              onClick={() => {
                if (step === 3 && !composed) {
                  message.warning('请先完成合成')
                  return
                }
                setStep((s) => s + 1)
              }}
            >
              下一步
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  )
}
