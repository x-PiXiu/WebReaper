import { useMemo, useState } from 'react'
import { Alert, Button, Modal, Typography } from 'antd'
import { PlusOutlined, UserOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { ViduSubject } from '../../utils/subjectTask'
import { SubjectPickCard } from './SubjectCard'
import { CREATIVE_CDN } from '../../config/creativeCdn'
import { MODAL_W, modalBodyScroll } from '../../ui/modalFit'

const { Text } = Typography

/** 官方主体占位（23 §2.3；GET /subjects?ownership=system 就绪后替换） */
const OFFICIAL_SHOWCASE = [
  { id: 'official-1', name: '数字人-女-坐', portraitUrl: CREATIVE_CDN.pipeline.copy, tag: '公共' },
  { id: 'official-2', name: '数字人-男-站', portraitUrl: CREATIVE_CDN.pipeline.voice, tag: '公共' },
  { id: 'official-3', name: '数字人-女-站', portraitUrl: CREATIVE_CDN.pipeline.mic, tag: '影视' },
] as const

type Props = {
  subjects: ViduSubject[]
  value?: string
  onChange: (serverId: string) => void
  highlightServerId?: string
  createHref?: string
  className?: string
  emptyHint?: string
}

/**
 * 分身选择器（23 号计划 §4.1）：触发条 + 弹窗双区（官方 + 我的）。
 * 官方区暂不可选（后端代理未就绪）；个人区分身即选即用。
 */
export function SubjectPicker({
  subjects,
  value,
  onChange,
  highlightServerId,
  createHref = '/m/compose/avatar?create=1',
  className,
  emptyHint = '还没有可用的数字分身',
}: Props) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const selected = useMemo(
    () => subjects.find((s) => s.serverId === value),
    [subjects, value],
  )
  const flashId = highlightServerId && highlightServerId !== value ? highlightServerId : undefined

  const pick = (serverId: string) => {
    onChange(serverId)
    setOpen(false)
  }

  return (
    <div className={['wz-subject-picker', className].filter(Boolean).join(' ')}>
      <button
        type="button"
        className={`wz-subject-trigger${selected ? ' has-value' : ''}${flashId ? ' is-highlight' : ''}`}
        onClick={() => setOpen(true)}
      >
        {selected?.portraitUrl ? (
          <span className="wz-subject-trigger-thumb" style={{ backgroundImage: `url(${selected.portraitUrl})` }} />
        ) : (
          <span className="wz-subject-trigger-thumb wz-subject-trigger-thumb--empty">
            <UserOutlined />
          </span>
        )}
        <span className="wz-subject-trigger-copy">
          <strong>{selected?.name || '选择数字分身'}</strong>
          <em>{selected ? '点击更换 · 官方区 + 我的分身' : '打开选择器：官方主体 / 我的分身'}</em>
        </span>
        <span className="wz-subject-trigger-action">选择</span>
      </button>

      <Modal
        open={open}
        onCancel={() => setOpen(false)}
        title="选择数字分身"
        width={MODAL_W.xl}
        footer={null}
        destroyOnHidden
        styles={{ body: modalBodyScroll.body }}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 14 }}
          message="官方主体即选即用；个人分身跨视频形象一致"
          description="官方列表待服务端代理就绪后可点选；当前请从「我的分身」选择，或去创建。"
        />

        <section className="wz-subject-modal-section">
          <header className="wz-subject-modal-head">
            <h4>官方主体</h4>
            <Text type="secondary">即将开放</Text>
          </header>
          <div className="wz-subject-modal-grid">
            {OFFICIAL_SHOWCASE.map((card) => (
              <div key={card.id} className="wz-subject-official is-disabled" title="官方主体接口尚未就绪">
                <span className="wz-subject-official-thumb" style={{ backgroundImage: `url(${card.portraitUrl})` }} />
                <strong>{card.name}</strong>
                <em>{card.tag}</em>
              </div>
            ))}
          </div>
        </section>

        <section className="wz-subject-modal-section">
          <header className="wz-subject-modal-head">
            <h4>我的分身</h4>
            <Button
              type="link"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => navigate(createHref)}
            >
              去创建
            </Button>
          </header>
          {subjects.length === 0 ? (
            <div className="wz-subject-empty">
              <Text type="secondary">{emptyHint}</Text>
              <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => navigate(createHref)}>
                去创建数字分身
              </Button>
            </div>
          ) : (
            <div className="wz-subject-picks wz-subject-picks--cards">
              {subjects.map((s) => (
                <SubjectPickCard
                  key={s.taskId}
                  subject={s}
                  active={value === s.serverId}
                  highlight={flashId === s.serverId}
                  onClick={() => s.serverId && pick(s.serverId)}
                />
              ))}
            </div>
          )}
        </section>
      </Modal>
    </div>
  )
}
