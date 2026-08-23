import { CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined } from '@ant-design/icons'
import type { PipelineStage } from './types'

type Props = {
  stages: PipelineStage[]
  className?: string
}

function StageIcon({ status }: { status: PipelineStage['status'] }) {
  if (status === 'done') return <CheckCircleOutlined className="wz-pipeline-icon is-done" />
  if (status === 'active') return <LoadingOutlined spin className="wz-pipeline-icon is-active" />
  if (status === 'error') return <CloseCircleOutlined className="wz-pipeline-icon is-error" />
  return <span className="wz-pipeline-dot" />
}

/** 多段生成链路进度（TTS → 参考生 → 对口型） */
export function PipelineProgress({ stages, className = '' }: Props) {
  return (
    <div className={`wz-pipeline ${className}`.trim()} role="list" aria-label="生成进度">
      {stages.map((s, i) => (
        <div key={s.key} className={`wz-pipeline-item is-${s.status}`} role="listitem">
          <div className="wz-pipeline-track">
            <StageIcon status={s.status} />
            {i < stages.length - 1 && <span className={`wz-pipeline-line is-${s.status}`} />}
          </div>
          <div className="wz-pipeline-copy">
            <strong>{s.label}</strong>
            <span>
              {s.status === 'done' ? '已完成'
                : s.status === 'active' ? '进行中…'
                : s.status === 'error' ? '失败'
                : '等待中'}
            </span>
          </div>
        </div>
      ))}
    </div>
  )
}
