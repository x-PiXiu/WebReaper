/** 向导步骤定义（与 product.FlowStepDef 对齐） */
export type WizardStepDef = {
  key: string
  label: string
  title: string
  tip: string
  nextLabel: string
}

export type PipelineStageStatus = 'pending' | 'active' | 'done' | 'error'

export type PipelineStage = {
  key: string
  label: string
  status: PipelineStageStatus
}
