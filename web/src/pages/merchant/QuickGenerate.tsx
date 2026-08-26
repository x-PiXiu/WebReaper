import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Input, Tag, Typography } from 'antd'
import { message } from '../../utils/antdApp'
import {
  FileImageOutlined, VideoCameraOutlined, AudioOutlined,
  CheckCircleOutlined, ExportOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../api/business'
import type { GenerationTask, MediaAsset } from '../../types/api'
import { useBrandContext } from '../../hooks/useBrands'
import { GENERATION_TASKS_KEY } from '../../hooks/useGenerationTasks'
import {
  WizardShell, PhonePreview, MaterialDropzone, SystemChoiceBadge, TemplatePickGrid, CapabilityBanner,
  type WizardStepDef,
} from '../../components/wizard'

const { TextArea } = Input
const { Text } = Typography

const QUICK_STEPS: WizardStepDef[] = [
  {
    key: 'template',
    label: '选模板',
    title: '选择生成场景',
    tip: '选一个最接近需求的模板，系统会自动配置时长与质量',
    nextLabel: '下一步：上传素材',
  },
  {
    key: 'create',
    label: '生成',
    title: '上传素材并描述',
    tip: '传素材、写一句话，端点和模型由系统自动选择',
    nextLabel: '开始生成',
  },
]

const MATERIAL_LABELS: Record<string, string> = {
  image: '图片',
  video: '视频',
  audio: '音频',
}

function countByType(assets: MediaAsset[], ids: string[]) {
  const counts = { image: 0, video: 0, audio: 0 }
  for (const a of assets) {
    if (!ids.includes(a.id)) continue
    const t = a.type as keyof typeof counts
    if (t in counts) counts[t]++
  }
  return counts
}

function requiredCounts(required: string[]) {
  const c: Record<string, number> = {}
  for (const t of required) c[t] = (c[t] || 0) + 1
  return c
}

function missingMaterials(required: string[], selectedIds: string[], assets: MediaAsset[]) {
  const have = countByType(assets, selectedIds)
  const need = requiredCounts(required)
  const missing: string[] = []
  for (const [type, count] of Object.entries(need)) {
    const gap = count - (have[type as keyof typeof have] || 0)
    for (let i = 0; i < gap; i++) missing.push(type)
  }
  return missing
}

function materialIcon(type: string) {
  switch (type) {
    case 'video': return <VideoCameraOutlined />
    case 'audio': return <AudioOutlined />
    default: return <FileImageOutlined />
  }
}

/**
 * 快速生成（09 统一 submit）：选模板 → 传素材+文案 → 系统自动选端点/模型
 */
export default function QuickGenerate() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { brandId } = useBrandContext()

  const [step, setStep] = useState(0)
  const [selectedTemplate, setSelectedTemplate] = useState('')
  const [text, setText] = useState('')
  const [selectedMaterials, setSelectedMaterials] = useState<string[]>([])
  const [duration, setDuration] = useState(0)
  const [quality, setQuality] = useState('720p')
  const [submitResult, setSubmitResult] = useState<GenerationTask | null>(null)

  const { data: templates = [], isLoading: templatesLoading } = useQuery({
    queryKey: ['generation-templates'],
    queryFn: () => businessApi.listTemplates(),
  })

  const { data: assets = [], refetch: refetchAssets } = useQuery({
    queryKey: ['assets'],
    queryFn: () => businessApi.listAssets(),
  })

  const currentTemplate = useMemo(
    () => templates.find(t => t.id === selectedTemplate) || null,
    [templates, selectedTemplate],
  )

  useEffect(() => {
    if (!currentTemplate?.default_params) return
    const d = currentTemplate.default_params.duration
    const q = currentTemplate.default_params.resolution
    if (typeof d === 'number' && d > 0) setDuration(d)
    if (typeof q === 'string' && q) setQuality(q)
  }, [currentTemplate?.id])

  const missing = useMemo(
    () => missingMaterials(currentTemplate?.required_materials || [], selectedMaterials, assets),
    [currentTemplate, selectedMaterials, assets],
  )

  const uploadMutation = useMutation({
    mutationFn: (file: File) => businessApi.uploadAsset(file),
    onSuccess: (data) => {
      message.success('素材已上传')
      setSelectedMaterials(prev => [...prev, data.id])
      refetchAssets()
    },
  })

  const submitMutation = useMutation({
    mutationFn: () => {
      if (!brandId) throw new Error('请先选择品牌')
      return businessApi.submitGeneration({
        brand_id: brandId,
        text,
        materials: selectedMaterials,
        template: selectedTemplate || undefined,
        duration: duration > 0 ? duration : undefined,
        quality,
      })
    },
    onSuccess: (task) => {
      setSubmitResult(task)
      message.success('任务已提交')
      queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : String(err || '')
      if (/已停用|停用|disabled/i.test(msg)) {
        message.warning('请联系管理员在后台启用文生视频能力，或先上传一张参考图使用图生视频')
      }
    },
  })

  const toggleAsset = (id: string) => {
    setSelectedMaterials(prev =>
      prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id],
    )
  }

  const handleNext = () => {
    if (step === 0) {
      setStep(1)
      return
    }
    if (submitResult) {
      navigate('/m/compose/tools?tab=media')
      return
    }
    if (!text.trim()) {
      message.warning('请填写描述')
      return
    }
    if (missing.length > 0) {
      message.warning(`还需上传：${missing.map(m => MATERIAL_LABELS[m] || m).join('、')}`)
      return
    }
    submitMutation.mutate()
  }

  const handleBack = () => {
    if (step === 0) navigate('/m/compose')
    else setStep(0)
  }

  const canNext = step === 0
    ? true
    : !!text.trim() && missing.length === 0 && !!brandId

  const nextHint = step === 1 && !brandId
    ? '请先在账号人设选择品牌'
    : step === 1 && !text.trim()
      ? '请填写描述'
      : step === 1 && missing.length > 0
        ? `还需：${missing.map(m => MATERIAL_LABELS[m] || m).join('、')}`
        : undefined

  const footerNextLabel = step === 1
    ? (submitResult ? '查看任务列表' : submitMutation.isPending ? '提交中…' : '开始生成')
    : undefined

  const alerts = (
    <>
      {!brandId && (
        <Alert
          type="warning" showIcon className="wz-draft-banner"
          message={<>请先在 <Link to="/m/brands">账号人设</Link> 选择品牌</>}
        />
      )}
      <CapabilityBanner required={['text2video', 'img2video', 'reference2video', 'lip_sync', 'tts']} />
      {submitResult && (
        <>
          <SystemChoiceBadge subType={submitResult.sub_type} model={submitResult.model} />
          <Alert
            type="success" showIcon icon={<CheckCircleOutlined />}
            className="wz-draft-banner"
            message={`任务已创建（${submitResult.id.slice(0, 8)}…），可在媒体工具查看进度`}
          />
        </>
      )}
    </>
  )

  return (
    <WizardShell
      breadcrumb="快速生成"
      steps={QUICK_STEPS}
      stepIndex={step}
      maxReachableStep={1}
      onStepChange={setStep}
      preview={
        <PhonePreview
          script={text}
          stepKey={step === 0 ? 'source' : 'produce'}
          topic={currentTemplate?.name || '自由生成'}
          estimatedSeconds={duration || Math.ceil(text.length / 4)}
        />
      }
      onBack={handleBack}
      onNext={handleNext}
      nextDisabled={step === 1 && !canNext && !submitResult}
      nextHint={nextHint}
      nextLoading={submitMutation.isPending}
      nextLabel={footerNextLabel}
      alerts={alerts}
    >
      {step === 0 && (
        <div className="ip-stagger">
          <TemplatePickGrid
            templates={templates}
            selectedId={selectedTemplate}
            onSelect={setSelectedTemplate}
            loading={templatesLoading}
          />
          {currentTemplate && (
            <Alert
              type="info" showIcon
              message={currentTemplate.description}
              description={
                currentTemplate.required_materials?.length
                  ? `下一步需准备：${currentTemplate.required_materials.map(m => MATERIAL_LABELS[m] || m).join('、')}`
                  : '此模板可不传素材，仅输入描述即可'
              }
            />
          )}
        </div>
      )}

      {step === 1 && (
        <div className="ip-form-stack ip-stagger">
          {currentTemplate && (
            <div className="wz-ready-tags">
              <Tag color="blue">{currentTemplate.icon} {currentTemplate.name}</Tag>
              {duration > 0 && <Tag>约 {duration} 秒</Tag>}
              <Tag>{quality}</Tag>
            </div>
          )}

          {missing.length > 0 && (
            <Alert
              type="warning" showIcon
              message={`还需上传 ${missing.length} 个素材`}
              description={missing.map(m => MATERIAL_LABELS[m] || m).join('、')}
            />
          )}

          <label>描述你想要的内容</label>
          <TextArea
            rows={4}
            showCount
            maxLength={500}
            placeholder="如：一个现代化的品牌宣传视频，展示我们的 Logo 和产品亮点"
            value={text}
            onChange={e => setText(e.target.value)}
          />

          <label>上传素材</label>
          <MaterialDropzone
            accept="image/*,video/*,audio/*"
            hint="支持图片、视频、音频，可多次上传"
            loading={uploadMutation.isPending}
            onUpload={async (file) => { await uploadMutation.mutateAsync(file) }}
          />

          {assets.length > 0 && (
            <>
              <label>从素材库选择</label>
              <div className="wz-asset-picks">
                {assets.slice(0, 24).map(a => (
                  <button
                    key={a.id}
                    type="button"
                    className={`wz-asset-pick${selectedMaterials.includes(a.id) ? ' is-on' : ''}`}
                    onClick={() => toggleAsset(a.id)}
                  >
                    <span className="wz-asset-pick-icon">{materialIcon(a.type)}</span>
                    <span className="wz-asset-pick-name">{a.name || a.id.slice(0, 8)}</span>
                    <span className="wz-asset-pick-type">{MATERIAL_LABELS[a.type] || a.type}</span>
                  </button>
                ))}
              </div>
            </>
          )}

          {selectedMaterials.length > 0 && (
            <div className="wz-selected-materials">
              <Text type="secondary" style={{ fontSize: 12 }}>已选 {selectedMaterials.length} 个素材</Text>
              <div className="wz-ready-tags">
                {selectedMaterials.map(id => {
                  const a = assets.find(x => x.id === id)
                  return (
                    <Tag
                      key={id}
                      closable
                      onClose={() => setSelectedMaterials(prev => prev.filter(x => x !== id))}
                    >
                      {materialIcon(a?.type || 'image')} {a?.name || id.slice(0, 8)}
                    </Tag>
                  )
                })}
              </div>
            </div>
          )}

          {submitResult && (
            <div className="wz-produce-actions">
              <Button icon={<ExportOutlined />} onClick={() => navigate('/m/compose/tools?tab=media')}>
                查看任务列表
              </Button>
            </div>
          )}
        </div>
      )}
    </WizardShell>
  )
}
