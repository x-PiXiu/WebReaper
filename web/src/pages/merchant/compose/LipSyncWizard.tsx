import { useEffect, useMemo, useState } from 'react'
import { useSearchParams, useLocation } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Alert, Button, Input, Radio, Segmented, Space, Steps, Tag, Typography, Upload, message,
} from 'antd'
import {
  LinkOutlined, UploadOutlined, SoundOutlined, VideoCameraOutlined, UserOutlined,
  RocketOutlined, CheckCircleOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import type { GenerationTask } from '../../../types/api'
import VoicePicker from '../../../components/VoicePicker'
import { useComposeDraft } from '../../../store/composeDraft'

const { Text, Paragraph } = Typography
const { TextArea } = Input

/** 正常语速 ≈4 字/秒（时长估算提示——D2） */
const estSeconds = (text: string) => Math.ceil((text || '').length / 4)

function taskParams(t: GenerationTask): Record<string, any> {
  if (t.params && typeof t.params === 'object') return t.params as Record<string, any>
  if (typeof t.params === 'string' && t.params) {
    try { return JSON.parse(t.params) } catch { return {} }
  }
  return {}
}

/** 轮询任务到终态（向导链式提交的步骤间等待） */
async function waitTask(id: string, onTick?: () => void, timeoutMs = 10 * 60 * 1000): Promise<GenerationTask> {
  const start = Date.now()
  for (;;) {
    onTick?.()
    const t = await businessApi.listGenerationTasks().then(r => r.tasks.find(x => x.id === id))
    if (t && (t.state === 'success' || t.state === 'failed' || t.state === 'cancelled')) {
      if (t.state !== 'success') throw new Error(t.err_msg || '任务失败')
      return t
    }
    if (Date.now() - start > timeoutMs) throw new Error('任务超时（10 分钟）')
    await new Promise(r => setTimeout(r, 5000))
  }
}

/**
 * 拍同款口播视频向导（08 计划 D2 五步）：
 * ① 文案来源（分享链提取/上传提取/手写）→ ② 文案确认（双产出+时长估算）
 * → ③ 出镜方式（真人视频/数字分身）→ ④ 音色 → ⑤ 成片（TTS→对口型，分身先参考生）。
 */
export default function LipSyncWizard() {
  // 08 R7：草稿持久化——刷新/关闭页面后可恢复向导进度
  const draft = useComposeDraft()
  const queryClient = useQueryClient()
  const [searchParams] = useSearchParams()
  const location = useLocation()
  const presetSubjectId = searchParams.get('subject') || location.state?.subjectId || ''
  const presetState = location.state as { rawText?: string; title?: string; method?: string } | null

  // 初始化优先级：URL 预填 > 草稿恢复 > 默认值
  const hasDraft = (draft.wizardStep ?? 0) > 0
  const [step, setStep] = useState(presetState?.rawText ? 1 : (draft.wizardStep || 0))

  // ① 文案来源
  const [shareUrl, setShareUrl] = useState('')
  const [extracting, setExtracting] = useState(false)
  // ② 文案
  const [, setRawText] = useState(presetState?.rawText || draft.wizardCleanText || '')
  const [cleanText, setCleanText] = useState(draft.wizardCleanText || '')
  const [script, setScript] = useState(presetState?.rawText || draft.wizardScript || '')
  const [topic, setTopic] = useState(draft.wizardTopic || '')
  const [rewriting, setRewriting] = useState(false)
  // ③ 出镜
  const [presence, setPresence] = useState<'real' | 'avatar'>(
    presetSubjectId ? 'avatar' : (draft.wizardPresence || 'real')
  )
  const [realVideoUrl, setRealVideoUrl] = useState(draft.wizardRealVideoUrl || '')
  const [intent, setIntent] = useState(draft.wizardIntent || '')
  // ④ 音色
  const [voiceId, setVoiceId] = useState(draft.wizardVoiceId || '')
  // ⑤ 成片
  const [producing, setProducing] = useState(false)
  const [stage, setStage] = useState('')
  const [resultUrl, setResultUrl] = useState(draft.wizardResultUrl || '')
  const [error, setError] = useState('')
  // 链路任务 ID（R7：恢复后可从断点续跑）
  const [ttsTaskId, setTtsTaskId] = useState(draft.wizardTtsTaskId || '')
  const [refTaskId, setRefTaskId] = useState(draft.wizardRefTaskId || '')
  const [lipsyncTaskId, setLipsyncTaskId] = useState(draft.wizardLipsyncTaskId || '')
  const [subjectServerId, setSubjectServerId] = useState(presetSubjectId || draft.wizardSubjectId || '')

  // 草稿同步：关键状态变更写回持久化（R7——刷新不丢进度）
  useEffect(() => {
    draft.patch({
      track: 'lipsync',
      wizardStep: step,
      wizardPresence: presence,
      wizardTopic: topic,
      wizardScript: script,
      wizardCleanText: cleanText,
      wizardVoiceId: voiceId,
      wizardRealVideoUrl: realVideoUrl,
      wizardSubjectId: subjectServerId,
      wizardIntent: intent,
      wizardTtsTaskId: ttsTaskId,
      wizardRefTaskId: refTaskId,
      wizardLipsyncTaskId: lipsyncTaskId,
      wizardResultUrl: resultUrl,
    })
  }, [step, presence, topic, script, cleanText, voiceId, realVideoUrl, subjectServerId, intent, ttsTaskId, refTaskId, lipsyncTaskId, resultUrl])

  // 初始化时若有预填原文（灵感广场提取跳转），自动触发一次 AI 润色（静默）
  const [initRewriting, setInitRewriting] = useState(false)
  useEffect(() => {
    if (presetState?.rawText && !initRewriting && cleanText === '') {
      const prefilled = presetState.rawText
      setInitRewriting(true)
      businessApi.rewriteScript({ raw_text: prefilled, topic: topic || '' })
        .then(rw => { setCleanText(rw.clean); setScript(rw.rewrite || rw.clean) })
        .catch(() => { setCleanText(prefilled) })
        .finally(() => setInitRewriting(false))
    }
  }, [])

  // 我的音色 + 主体库（出镜选择）
  const { data: tasks = [] } = useQuery({
    queryKey: ['generation-tasks'],
    queryFn: () => businessApi.listGenerationTasks().then(r => r.tasks),
  })
  const myVoices = useMemo(() => {
    const ids = new Set<string>()
    for (const t of tasks) {
      if (t.sub_type !== 'voice_clone' || t.state !== 'success') continue
      const vid = taskParams(t).voice_id
      if (typeof vid === 'string' && vid) ids.add(vid)
    }
    return Array.from(ids)
  }, [tasks])
  const subjects = useMemo(() => tasks
    .filter(t => t.sub_type === 'subject' && t.state === 'success')
    .map(t => ({
      id: t.id,
      serverId: t.provider_task_id,
      name: taskParams(t).name || t.id.slice(0, 8),
      hasVideo: Array.isArray(taskParams(t).videos) && taskParams(t).videos.length > 0,
    })), [tasks])

  // ① 提取
  const doExtract = async (payload: { share_url?: string; asset_url?: string }) => {
    setExtracting(true); setError('')
    try {
      const r = await businessApi.extractTranscript(payload)
      setRawText(r.raw_text)
      setScript('')
      const rw = await businessApi.rewriteScript({ raw_text: r.raw_text, topic: topic || '口播获客' })
      setCleanText(rw.clean)
      setScript(rw.rewrite || rw.clean)
      message.success('提取完成，文案已生成——可编辑')
      setStep(1)
    } catch (e: any) {
      setError(e?.response?.data?.msg || e?.message || '提取失败')
    } finally { setExtracting(false) }
  }

  // ② 润色（手写路径 / 对编辑结果再润色）
  const doRewrite = async () => {
    if (!script.trim()) { message.warning('请先输入文案'); return }
    setRewriting(true)
    try {
      const rw = await businessApi.rewriteScript({ raw_text: script, topic: topic || '口播获客' })
      setRawText(script)
      setCleanText(rw.clean)
      setScript(rw.rewrite || rw.clean)
      message.success('已润色')
    } catch { /* 拦截器已提示 */ } finally { setRewriting(false) }
  }

  // ⑤ 成片：TTS → （分身：参考生）→ 对口型
  const produce = async () => {
    if (!script.trim()) { message.warning('文案为空'); return }
    if (!voiceId) { message.warning('请选择音色'); return }
    setProducing(true); setError(''); setResultUrl(''); setStage('提交语音合成…')
    try {
      // ① TTS：文案 → 试听/驱动音频
      const tts = await businessApi.submitGenerationTask({
        sub_type: 'tts', model: '',
        params: { text: script, voice_setting_voice_id: voiceId },
      })
      setTtsTaskId(tts.id)
      queryClient.invalidateQueries({ queryKey: ['generation-tasks'] })
      const ttsDone = await waitTask(tts.id, () => setStage('语音合成中…'))
      const audioUrl = ttsDone.creations?.[0]?.stored_url || ttsDone.creations?.[0]?.url || ''

      // ② 出镜画面
      let videoUrl = realVideoUrl
      if (presence === 'avatar') {
        if (!subjectServerId) throw new Error('请选择数字分身')
        setStage('生成数字分身画面（参考生视频）…')
        const prompt = intent.trim() || '人物面对镜头自然口播'
        const ref = await businessApi.submitGenerationTask({
          sub_type: 'reference2video', model: '', // D3 模型自动切换
          params: { subjects: [{ name: '主角', server_id: subjectServerId }], prompt },
        })
        setRefTaskId(ref.id)
        queryClient.invalidateQueries({ queryKey: ['generation-tasks'] })
        const refDone = await waitTask(ref.id)
        videoUrl = refDone.creations?.[0]?.stored_url || refDone.creations?.[0]?.url || ''
      } else if (!videoUrl) {
        throw new Error('请上传出镜视频')
      }
      if (!audioUrl) throw new Error('语音产物缺失（可重试）')
      if (!videoUrl) throw new Error('分身画面产物缺失（可重试）')

      // ③ 对口型
      setStage('对口型合成成片…')
      const lipsync = await businessApi.submitGenerationTask({
        sub_type: 'lip_sync', model: '',
        params: { video_url: videoUrl, audio_url: audioUrl },
      })
      setLipsyncTaskId(lipsync.id)
      queryClient.invalidateQueries({ queryKey: ['generation-tasks'] })
      const done = await waitTask(lipsync.id)
      setResultUrl(done.creations?.[0]?.stored_url || done.creations?.[0]?.url || '')
      message.success('成片完成')
      queryClient.invalidateQueries({ queryKey: ['generation-tasks'] })
    } catch (e: any) {
      setError(e?.response?.data?.msg || e?.message || '成片失败')
    } finally { setProducing(false); setStage('') }
  }

  const scriptSec = estSeconds(script)

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Wizard</p>
          <h1>拍同款口播视频</h1>
          <p className="ip-page-lead">参考爆款说话内容 → 确认文案 → 选谁来出镜 → 配音色 → 一键成片</p>
        </div>
      </div>

      <Steps
        current={step}
        onChange={setStep}
        size="small"
        style={{ marginBottom: 20, maxWidth: 760 }}
        items={[
          { title: '文案来源' }, { title: '确认文案' }, { title: '出镜方式' }, { title: '音色' }, { title: '成片' },
        ]}
      />

      {error && <Alert type="error" showIcon style={{ marginBottom: 14 }} message={error} />}
      {hasDraft && step > 0 && !resultUrl && (
        <Alert
          type="info" showIcon closable style={{ marginBottom: 14 }}
          message="已恢复上次进度（草稿自动保存）"
        />
      )}

      {step === 0 && (
        <Space direction="vertical" size={14} style={{ width: '100%', maxWidth: 640 }}>
          <Alert type="info" showIcon message="粘贴爆款视频分享链接，提取它说了什么作为你的文案底稿；也可以跳过直接手写" />
          <Input.Search
            size="large" enterButton={<><LinkOutlined /> 提取文案</>}
            placeholder="粘贴抖音/快手分享链接（如 https://v.douyin.com/xxxx）"
            value={shareUrl} onChange={e => setShareUrl(e.target.value)}
            loading={extracting}
            onSearch={() => shareUrl.trim() && doExtract({ share_url: shareUrl.trim() })}
          />
          <Space size={12}>
            <Upload
              accept="audio/*,video/*" showUploadList={false}
              customRequest={async ({ file, onSuccess, onError }) => {
                try {
                  const r = await businessApi.uploadAsset(file as File)
                  await doExtract({ asset_url: r.url })
                  onSuccess?.(r)
                } catch (e) { onError?.(e as Error) }
              }}
            >
              <Button icon={<UploadOutlined />} loading={extracting}>上传音/视频提取</Button>
            </Upload>
            <Button type="dashed" onClick={() => setStep(1)}>跳过，手写文案</Button>
          </Space>
        </Space>
      )}

      {step === 1 && (
        <Space direction="vertical" size={12} style={{ width: '100%', maxWidth: 720 }}>
          <Input
            placeholder="一句话主题（AI 润色/改写围绕它，如：酸菜鱼餐馆新菜品）"
            value={topic} onChange={e => setTopic(e.target.value)} maxLength={100}
          />
          <TextArea
            rows={8} showCount
            value={script} onChange={e => setScript(e.target.value)}
            placeholder="输入或提取口播文案…"
            style={{ fontSize: 14 }}
          />
          <Space size={12} wrap>
            <Tag color={scriptSec > 0 ? 'blue' : 'default'}>约 {scriptSec} 秒语音（≈4字/秒）</Tag>
            <Button loading={rewriting} onClick={doRewrite} disabled={!script.trim()}>AI 润色/改写</Button>
            {cleanText && (
              <Button type="dashed" onClick={() => { setScript(cleanText); message.info('已切换为清洗版原文') }}>用原文</Button>
            )}
            <Button type="primary" disabled={!script.trim()} onClick={() => setStep(2)}>下一步：选谁出镜</Button>
          </Space>
        </Space>
      )}

      {step === 2 && (
        <Space direction="vertical" size={14} style={{ width: '100%', maxWidth: 680 }}>
          <Segmented
            value={presence}
            onChange={v => setPresence(v as 'real' | 'avatar')}
            options={[
              { value: 'real', label: '真人出镜（自己拍的视频）', icon: <VideoCameraOutlined /> },
              { value: 'avatar', label: '数字分身（AI 生成画面）', icon: <UserOutlined /> },
            ]}
          />
          {presence === 'real' ? (
            <>
              <Alert type="info" showIcon message="上传一段你自己出镜、不说话的视频——成片里你会对着文案开口说话（正脸、光线稳定效果最好）" />
              <Upload
                accept="video/mp4,video/quicktime,x-msvideo" maxCount={1}
                customRequest={async ({ file, onSuccess, onError }) => {
                  try {
                    const r = await businessApi.uploadAsset(file as File)
                    setRealVideoUrl(r.url)
                    onSuccess?.(r)
                  } catch (e) { onError?.(e as Error) }
                }}
                onRemove={() => setRealVideoUrl('')}
              >
                <Button icon={<UploadOutlined />}>{realVideoUrl ? '重新上传' : '上传出镜视频（mp4/mov/avi）'}</Button>
              </Upload>
              <Text type="secondary" style={{ fontSize: 12 }}>
                时长参考：文案约 {scriptSec} 秒，出镜视频时长最好相近（差异大 Vidu 可能截断或延长）
              </Text>
            </>
          ) : (
            <>
              <Alert type="info" showIcon message="选择资产库里的数字分身（没有可先去资产库创建）；一句话描述场景，AI 生成画面后再对口型" />
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                {subjects.length === 0 ? (
                  <Text type="warning">还没有数字分身——<a href="/m/assets" target="_blank" rel="noreferrer">去资产库创建</a></Text>
                ) : subjects.map(s => (
                  <Radio.Button
                    key={s.id} checked={subjectServerId === s.serverId}
                    onClick={() => setSubjectServerId(s.serverId)}
                  >
                    {s.name}{s.hasVideo ? '（视频分身）' : ''}
                  </Radio.Button>
                ))}
              </Space>
              <Input
                placeholder="一句话场景意图（如：在厨房里边做菜边对镜头讲解）"
                value={intent} onChange={e => setIntent(e.target.value)} maxLength={200}
              />
            </>
          )}
          <Button
            type="primary"
            disabled={presence === 'real' ? !realVideoUrl : !subjectServerId}
            onClick={() => setStep(3)}
          >下一步：配音色</Button>
        </Space>
      )}

      {step === 3 && (
        <Space direction="vertical" size={14} style={{ width: '100%', maxWidth: 560 }}>
          <Text strong><SoundOutlined /> 选择口播音色（可试听）</Text>
          <VoicePicker value={voiceId} onChange={setVoiceId} myVoices={myVoices} style={{ maxWidth: 420 }} />
          <Text type="secondary" style={{ fontSize: 12 }}>
            想用自己的声音？<a href="/m/compose/tools?tab=media" target="_blank" rel="noreferrer">去声音克隆</a>（7 天内在语音合成中调用一次即永久保留）
          </Text>
          <Button type="primary" disabled={!voiceId} onClick={() => setStep(4)}>下一步：生成成片</Button>
        </Space>
      )}

      {step === 4 && (
        <Space direction="vertical" size={14} style={{ width: '100%', maxWidth: 720 }}>
          <Paragraph>
            就绪检查：<Tag color="green">文案 {script.length} 字</Tag>
            <Tag color="green">{presence === 'real' ? '真人出镜' : '数字分身'}</Tag>
            <Tag color="green">音色 {voiceId}</Tag>
          </Paragraph>
          <Button type="primary" size="large" icon={<RocketOutlined />} loading={producing} onClick={produce}>
            {producing ? (stage || '生成中…') : '一键成片（语音 → 画面 → 对口型）'}
          </Button>
          {resultUrl && (
            <div>
              <Alert type="success" showIcon icon={<CheckCircleOutlined />} message="成片完成" style={{ marginBottom: 10 }} />
              <video src={resultUrl} controls style={{ width: '100%', maxWidth: 420, borderRadius: 12 }} />
              <div style={{ marginTop: 8 }}>
                <Button href={resultUrl} target="_blank" download>下载成片</Button>
              </div>
            </div>
          )}
        </Space>
      )}
    </div>
  )
}
