import { useMemo, useState } from 'react'
import { Button, Modal, Typography } from 'antd'
import { PlusOutlined, UserOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { ViduSubject } from '../../utils/subjectTask'
import { SubjectPickCard } from './SubjectCard'
import { useOfficialSubjects, assetToViduSubject } from '../../hooks/useSubjectAssets'
import { MODAL_W, modalBodyScroll } from '../../ui/modalFit'

const { Text } = Typography

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
 * 数字人选择器：触发条 + 弹窗双区（官方数字人 + 我的分身）。
 * 官方区读 subject_assets(scope=official) 真实数据，即选即用；
 * 个人分身跨视频形象一致。
 */
export function SubjectPicker({
  subjects,
  value,
  onChange,
  highlightServerId,
  createHref = '/m/compose/avatar?create=1',
  className,
  emptyHint = '还没有可用的数字人——可先用官方数字人，或去创建',
}: Props) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const { subjects: officialAssets } = useOfficialSubjects({ kind: 'person', enabled: open })
  const officialSubjects = useMemo(() => officialAssets.map(assetToViduSubject), [officialAssets])
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
          <strong>{selected?.name || '选择数字人'}</strong>
          <em>{selected ? '点击更换 · 官方数字人 / 我的分身' : '打开选择器：官方数字人 / 我的分身'}</em>
        </span>
        <span className="wz-subject-trigger-action">选择</span>
      </button>

      <Modal
        open={open}
        onCancel={() => setOpen(false)}
        title="选择数字人"
        width={MODAL_W.xl}
        footer={null}
        destroyOnHidden
        styles={{ body: modalBodyScroll.body }}
      >
        <section className="wz-subject-modal-section">
          <header className="wz-subject-modal-head">
            <h4>官方数字人</h4>
            <Text type="secondary">即选即用 · 免创建</Text>
          </header>
          {officialSubjects.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 12 }}>暂无官方数字人</Text>
          ) : (
            <div className="wz-subject-picks wz-subject-picks--cards">
              {officialSubjects.map((s) => (
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
