import { Button, Space } from 'antd'
import { CloudUploadOutlined } from '@ant-design/icons'
import type { WizardStep } from './wizardModel'

type Props = {
  step: WizardStep
  onPrev: () => void
  onNext: () => void
  onSaveDraft: () => void
  onClear: () => void
  onPublish: () => void
  savingDraft?: boolean
  publishing?: boolean
  canPublish?: boolean
}

/** 发布向导底栏（窄屏吸底） */
export function PublishWizardFooter({
  step,
  onPrev,
  onNext,
  onSaveDraft,
  onClear,
  onPublish,
  savingDraft,
  publishing,
  canPublish,
}: Props) {
  return (
    <footer className="pub-wizard-footer">
      <Space>
        <Button icon={<CloudUploadOutlined />} loading={savingDraft} onClick={onSaveDraft}>
          保存草稿
        </Button>
        <Button type="text" onClick={onClear}>清空</Button>
      </Space>
      <Space>
        <Button disabled={step <= 1} onClick={onPrev}>上一步</Button>
        {step < 5 ? (
          <Button type="primary" className="ip-btn-primary" onClick={onNext}>下一步</Button>
        ) : (
          <Button
            type="primary"
            className="ip-btn-primary"
            loading={publishing}
            disabled={!canPublish}
            onClick={onPublish}
          >
            发布
          </Button>
        )}
      </Space>
    </footer>
  )
}
