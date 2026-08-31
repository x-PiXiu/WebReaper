import { useMemo, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Select, Tag } from 'antd'
import { PlayCircleOutlined, PauseCircleOutlined } from '@ant-design/icons'
import type { GenerationVoice } from '../types/api'
import { businessApi } from '../api/business'

/** 全局唯一试听播放器（切换选项自动停上一段） */
let previewAudio: HTMLAudioElement | null = null
function stopPreview() {
  if (previewAudio) {
    previewAudio.pause()
    previewAudio = null
  }
}

/**
 * 音色选择器：官方音色库（/generation/voices，按语言分组）+ 我的音色（声音克隆产物）。
 * 选中值为 voice_id 字符串（TTS 的 voice_setting_voice_id / 主体与数字人的 voice_id）。
 */
export default function VoicePicker({
  value, onChange, myVoices = [], placeholder, style,
}: {
  value?: string
  onChange?: (v: string) => void
  myVoices?: string[]
  placeholder?: string
  style?: React.CSSProperties
}) {
  const playingRef = useRef<string>('')

  const { data: voices = [] } = useQuery({
    queryKey: ['generation-voices'],
    queryFn: () => businessApi.listGenerationVoices().then(r => r.voices),
    staleTime: 24 * 60 * 60 * 1000, // 静态参考数据——当天内不重拉
  })

  // 分组：平台精选音色 → 各语言官方音色 → 我的音色
  const groups = useMemo(() => {
    const platform: GenerationVoice[] = []
    const byLang = new Map<string, GenerationVoice[]>()
    for (const v of voices) {
      // platform scope = 管理后台创建的官方复刻音色（置顶显示）
      if (v.scope === 'platform') {
        platform.push(v)
        continue
      }
      // recommend = 精选推荐（口播常用音色）
      const lang = v.recommend ? '★ 精选推荐' : (v.language || '其他')
      if (!byLang.has(lang)) byLang.set(lang, [])
      byLang.get(lang)!.push(v)
    }
    // 平台精选音色置顶
    if (platform.length > 0) {
      byLang.set('🎤 平台精选', platform)
    }
    // 精选推荐组排在最前
    const sorted = new Map<string, GenerationVoice[]>()
    const recommendKey = '★ 精选推荐'
    if (byLang.has(recommendKey)) sorted.set(recommendKey, byLang.get(recommendKey)!)
    for (const [k, v] of byLang) {
      if (k !== recommendKey) sorted.set(k, v)
    }
    return sorted
  }, [voices])

  const togglePreview = (e: React.MouseEvent, v: GenerationVoice) => {
    e.stopPropagation()
    e.preventDefault()
    if (playingRef.current === v.voice_id && previewAudio) {
      stopPreview()
      playingRef.current = ''
      return
    }
    stopPreview()
    previewAudio = new Audio(v.sample_url)
    playingRef.current = v.voice_id
    previewAudio.onended = () => { playingRef.current = ''; stopPreview() }
    previewAudio.play().catch(() => { playingRef.current = '' })
  }

  // antd 分组 options：附加字段 voice 供 optionRender 试听用（结构对齐 DefaultOptionType）
  const officialOptions: any[] = Array.from(groups.entries()).map(([lang, list]) => ({
    label: `${lang}（${list.length}）`,
    options: list.map(v => ({
      value: v.voice_id,
      label: `${v.name}（${v.voice_id}）`,
      voice: v,
    })),
  }))

  const myOptions: any[] = myVoices.length > 0 ? [{
    label: `我的音色（声音克隆，${myVoices.length}）`,
    options: myVoices.map(id => ({ value: id, label: id, voice: null })),
  }] : []
  return (
    <Select
      style={{ width: '100%', ...style }}
      showSearch
      allowClear
      value={value || undefined}
      onChange={v => onChange?.(v || '')}
      placeholder={placeholder || '搜索并选择音色（支持名称/ID）'}
      filterOption={(input, option) => String(option?.value ?? '').toLowerCase().includes(input.toLowerCase())
        || String(option?.label ?? '').toLowerCase().includes(input.toLowerCase())}
      filterSort={(a, b) => String(a?.value ?? '').localeCompare(String(b?.value ?? ''))}
      options={[...myOptions, ...officialOptions]}
      optionRender={(option) => {
        const v = (option as any).voice as GenerationVoice | null
        return (
          <span style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{option.label}</span>
            {v && (
              <span
                onClick={e => togglePreview(e, v)}
                style={{ color: 'var(--wr-primary, #1677ff)', flexShrink: 0, fontSize: 18, lineHeight: 1 }}
                title="试听示例"
              >
                {playingRef.current === v.voice_id && previewAudio ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
              </span>
            )}
            {!v && <Tag style={{ margin: 0 }} title="克隆音色：7 天内在语音合成中调用一次即永久保留">克隆</Tag>}
            {v?.scope === 'platform' && <Tag color="orange" style={{ margin: 0 }} title="管理后台创建的平台精选音色">平台</Tag>}
          </span>
        )
      }}
      onDropdownVisibleChange={(open) => { if (!open) stopPreview() }}
    />
  )
}
