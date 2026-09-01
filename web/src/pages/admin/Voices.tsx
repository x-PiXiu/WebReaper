import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Alert, Button, Input, Modal, Popconfirm, Segmented, Space, Spin, Switch,
  Table, Tag, Typography, Upload,
} from 'antd'
import { PlusOutlined, UploadOutlined } from '@ant-design/icons'
import { businessApi } from '../../api/business'
import { message } from '../../utils/antdApp'
import type { GenerationVoice } from '../../types/api'

const { Text } = Typography

/**
 * 官方音色管理（04 号音频业务 §2.3）：
 * 列表（scope 三类过滤）+ 创建（上传样本或 URL + 试听文本——服务端 SynthesizeClone
 * 合成试听后写 platform 行）+ 编辑（名称/语言/上下架）+ 删除（软删，仅 platform）。
 */
const VOICE_SCOPES = [
  { value: 'platform', label: '平台精选' },
  { value: 'vidu', label: '官方音色' },
  { value: 'clone', label: '用户克隆' },
]

function AdminVoices({ embedded = false }: { embedded?: boolean }) {
  const queryClient = useQueryClient()
  const [scope, setScope] = useState('platform')
  const [q, setQ] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [audioUrl, setAudioUrl] = useState('')
  const [audioFile, setAudioFile] = useState<File | null>(null)
  const [text, setText] = useState('欢迎来到智宸AI，用一句话介绍你的店铺，让更多客人找到你。')
  const [name, setName] = useState('')
  const [language, setLanguage] = useState('平台精选')
  const [editing, setEditing] = useState<GenerationVoice | null>(null)
  const [editName, setEditName] = useState('')
  const [editLang, setEditLang] = useState('')
  const [editActive, setEditActive] = useState(true)
  // Vidu 克隆 Tab
  const [viduOpen, setViduOpen] = useState(false)
  const [viduVoices, setViduVoices] = useState<GenerationVoice[]>([])
  const [viduLoading, setViduLoading] = useState(false)
  const [viduQ, setViduQ] = useState('')
  const [viduSelected, setViduSelected] = useState<GenerationVoice | null>(null)
  const [viduText, setViduText] = useState('欢迎来到智宸AI，用一句话介绍你的店铺，让更多客人找到你。')
  const [viduName, setViduName] = useState('')
  const [viduCloning, setViduCloning] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-voices', scope],
    queryFn: () => businessApi.adminListVoices({ scope }).then((r) => r.voices),
  })

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-voices'] })
  }

  const openViduTab = async () => {
    setViduOpen(true)
    if (viduVoices.length === 0) {
      setViduLoading(true)
      try {
        const r = await businessApi.adminListViduVoices()
        setViduVoices(r.voices || [])
      } catch { /* 拦截器已提示 */ } finally {
        setViduLoading(false)
      }
    }
  }

  const doViduClone = async () => {
    if (!viduSelected) { message.warning('请选择一个 Vidu 音色作为克隆源'); return }
    if (!viduText.trim()) { message.warning('请填写介绍文本'); return }
    setViduCloning(true)
    try {
      await businessApi.adminCreateVoiceFromVidu({
        vidu_voice_id: viduSelected.voice_id,
        text: viduText.trim(),
        name: viduName.trim() || undefined,
      })
      message.success(`已从「${viduSelected.name}」克隆出平台音色——用户端即显`)
      setViduOpen(false)
      setViduSelected(null)
      setViduName('')
      setScope('platform')
      refresh()
    } catch { /* 拦截器已提示 */ } finally {
      setViduCloning(false)
    }
  }

  const doSetDefault = async (voiceId: string) => {
    try {
      await businessApi.adminSetDefaultVoice(voiceId)
      message.success('已设为平台默认音色——用户不选音色时使用')
      refresh()
    } catch { /* 拦截器已提示 */ }
  }

  const doCreate = async () => {
    if (!audioFile && !audioUrl.trim()) { message.warning('请上传样本音频或填写音频 URL'); return }
    if (!text.trim()) { message.warning('请填写试听文本'); return }
    setCreating(true)
    try {
      if (audioFile) {
        const form = new FormData()
        form.append('audio', audioFile)
        form.append('text', text.trim())
        if (name.trim()) form.append('name', name.trim())
        if (language.trim()) form.append('language', language.trim())
        await businessApi.adminCreateVoice(form)
      } else {
        await businessApi.adminCreateVoiceByUrl({
          audio_url: audioUrl.trim(), text: text.trim(),
          name: name.trim() || undefined, language: language.trim() || undefined,
        })
      }
      message.success('官方音色已创建——用户端 VoicePicker「平台精选」即显')
      setCreateOpen(false)
      setAudioFile(null)
      setAudioUrl('')
      setName('')
      refresh()
    } catch { /* 拦截器已提示 */ } finally {
      setCreating(false)
    }
  }

  const doUpdate = async () => {
    if (!editing) return
    try {
      await businessApi.adminUpdateVoice(editing.voice_id, {
        name: editName.trim() || undefined,
        language: editLang.trim() || undefined,
        status: editActive ? 'active' : 'disabled',
      })
      message.success('已保存')
      setEditing(null)
      refresh()
    } catch { /* 拦截器已提示 */ }
  }

  return (
    <div className={embedded ? '' : 'wr-page-content ip-page'}>
      {!embedded && (
        <div className="ip-page-hero">
          <div>
            <h1>官方音色</h1>
            <p className="ip-lead">运营创建平台音色——用户端仅显示此处创建的内容（白牌化：上游 Vidu 音色仅作克隆参考源）</p>
          </div>
          <Space size={8}>
            <Button size="large" onClick={openViduTab}>
              从 Vidu 音色克隆
            </Button>
            <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
              上传样本创建
            </Button>
          </Space>
        </div>
      )}
      {embedded && (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <Text type="secondary">用户端仅显示此处创建的音色（白牌化）</Text>
          <Space size={8}>
            <Button onClick={openViduTab}>从 Vidu 音色克隆</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
              上传样本创建
            </Button>
          </Space>
        </div>
      )}

      <Alert
        type="info" showIcon style={{ marginBottom: 16 }}
        message="创建流程：上传 10 秒-5 分钟人声样本 + 试听文本 → 系统用小米声音克隆合成试听 → 写入音色库（scope=platform）"
        description="复刻音色的 7 天保留规则同样适用：创建后 7 天内需有合成调用，建议创建后立即在用户端试听确认。"
      />

      <div className="ip-toolbar">
        <Segmented value={scope} onChange={(v) => setScope(v as string)} options={VOICE_SCOPES} />
        <Input.Search
          allowClear
          placeholder="搜索 voice_id / 名称"
          style={{ maxWidth: 260 }}
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      </div>

      <Table<GenerationVoice>
        rowKey="voice_id"
        loading={isLoading}
        dataSource={data ?? []}
        pagination={{ pageSize: 20, showTotal: (t) => `共 ${t} 条` }}
        columns={[
          {
            title: '音色', dataIndex: 'name', render: (_, v) => (
              <Space direction="vertical" size={2}>
                <Text strong>{v.name || v.voice_id.slice(0, 16)}</Text>
                <Text type="secondary" style={{ fontSize: 12 }} copyable={{ text: v.voice_id }}>{v.voice_id}</Text>
              </Space>
            ),
          },
          { title: '语言/分组', dataIndex: 'language', width: 130 },
          {
            title: '类型', dataIndex: 'scope', width: 100, render: (s: string) => (
              <Tag color={s === 'platform' ? 'purple' : s === 'clone' ? 'blue' : 'default'}>
                {VOICE_SCOPES.find((x) => x.value === s)?.label ?? s}
              </Tag>
            ),
          },
          {
            title: '状态', dataIndex: 'status', width: 110, render: (_, v) => (
              <Space size={4}>
                <Tag color={v.status === 'active' ? 'green' : 'default'}>{v.status === 'active' ? '启用' : v.status === 'deleted' ? '已删' : '停用'}</Tag>
                {v.is_default && <Tag color="gold">默认</Tag>}
              </Space>
            ),
          },
          {
            title: '试听', width: 90, render: (_, v) => v.sample_url ? (
              <audio controls preload="none" src={v.sample_url} style={{ height: 32, maxWidth: 200 }} />
            ) : <Text type="secondary">无</Text>,
          },
          {
            title: '操作', width: 200, render: (_, v) => (
              <Space size={4}>
                {v.scope === 'platform' && !v.is_default && (
                  <Popconfirm
                    title="设为平台默认音色？"
                    description="用户不选音色时自动使用此音色"
                    okText="设为默认" cancelText="取消"
                    onConfirm={() => doSetDefault(v.voice_id)}
                  >
                    <Button size="small" type="link">设为默认</Button>
                  </Popconfirm>
                )}
                <Button size="small" onClick={() => {
                  setEditing(v)
                  setEditName(v.name)
                  setEditLang(v.language)
                  setEditActive(v.status !== 'disabled')
                }}>编辑</Button>
                {v.scope === 'platform' && (
                  <Popconfirm
                    title="删除该官方音色？"
                    description="软删（status=deleted），用户端不再展示"
                    okText="删除" okButtonProps={{ danger: true }} cancelText="取消"
                    onConfirm={async () => {
                      try {
                        await businessApi.adminDeleteVoice(v.voice_id)
                        message.success('已删除')
                        refresh()
                      } catch { /* 拦截器已提示 */ }
                    }}
                  >
                    <Button size="small" danger>删除</Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />

      <Modal
        open={createOpen}
        title="创建官方音色"
        okText="创建" cancelText="取消"
        confirmLoading={creating}
        onOk={doCreate}
        onCancel={() => setCreateOpen(false)}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <div>
            <Text strong>样本音频（二选一）</Text>
            <Upload
              maxCount={1}
              accept="audio/mpeg,audio/mp4,audio/wav"
              beforeUpload={(f) => { setAudioFile(f); setAudioUrl(''); return false }}
              onRemove={() => setAudioFile(null)}
            >
              <Button icon={<UploadOutlined />}>{audioFile ? '重新选择' : '上传人声样本（10 秒-5 分钟）'}</Button>
            </Upload>
            <Input
              style={{ marginTop: 8 }}
              placeholder="或填音频 URL（mp3/m4a/wav，≤50MB）"
              value={audioUrl}
              onChange={(e) => { setAudioUrl(e.target.value); setAudioFile(null) }}
            />
          </div>
          <div>
            <Text strong>试听文本（合成后作为该音色的 sample）</Text>
            <Input.TextArea
              rows={3}
              maxLength={1000}
              showCount
              value={text}
              onChange={(e) => setText(e.target.value)}
            />
          </div>
          <Input placeholder="音色名称（如：平台·温柔女声）" value={name} onChange={(e) => setName(e.target.value)} maxLength={64} />
          <Input placeholder="语言/分组（默认：平台精选）" value={language} onChange={(e) => setLanguage(e.target.value)} maxLength={32} />
        </Space>
      </Modal>

      <Modal
        open={!!editing}
        title={`编辑音色 · ${editing?.name ?? ''}`}
        okText="保存" cancelText="取消"
        onOk={doUpdate}
        onCancel={() => setEditing(null)}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Input placeholder="名称" value={editName} onChange={(e) => setEditName(e.target.value)} maxLength={64} />
          <Input placeholder="语言/分组" value={editLang} onChange={(e) => setEditLang(e.target.value)} maxLength={32} />
          <Space>
            <Switch checked={editActive} onChange={setEditActive} />
            <Text>{editActive ? '启用（用户端可见）' : '停用（用户端隐藏）'}</Text>
          </Space>
        </Space>
      </Modal>

      {/* Vidu 音色克隆弹窗（白牌化——用上游音色做种子，产出平台自建音色） */}
      <Modal
        open={viduOpen}
        title="从 Vidu 音色克隆平台音色"
        okText="克隆并发布" cancelText="取消"
        width={640}
        confirmLoading={viduCloning}
        onOk={doViduClone}
        onCancel={() => setViduOpen(false)}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type="info" showIcon
            message="选择一个 Vidu 音色作为克隆源 → 系统用它生成一段介绍词 TTS → 以此音频克隆出平台专属音色"
            description="用户端不会看到任何 Vidu 标识——最终展示的是你命名的平台音色。"
          />
          <Input.Search
            allowClear
            placeholder="搜索 Vidu 音色（名称 / voice_id / 语言）"
            value={viduQ}
            onChange={(e) => setViduQ(e.target.value)}
          />
          <div style={{ maxHeight: 280, overflowY: 'auto', border: '1px solid var(--wr-border)', borderRadius: 8, padding: 8 }}>
            {viduLoading ? (
              <div style={{ textAlign: 'center', padding: 24 }}><Spin /></div>
            ) : (
              (viduQ.trim()
                ? viduVoices.filter((v) =>
                    v.name.toLowerCase().includes(viduQ.toLowerCase()) ||
                    v.voice_id.toLowerCase().includes(viduQ.toLowerCase()) ||
                    v.language.includes(viduQ))
                : viduVoices
              ).slice(0, 50).map((v) => (
                <div
                  key={v.voice_id}
                  onClick={() => setViduSelected(v)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', borderRadius: 8, cursor: 'pointer',
                    border: `1px solid ${viduSelected?.voice_id === v.voice_id ? 'var(--wr-primary-border)' : 'transparent'}`,
                    background: viduSelected?.voice_id === v.voice_id ? 'var(--wr-primary-bg)' : 'transparent',
                    marginBottom: 4,
                  }}
                >
                  <Text strong style={{ minWidth: 100 }}>{v.name}</Text>
                  <Text type="secondary" style={{ fontSize: 12, flex: 1 }}>{v.language}</Text>
                  {v.sample_url && (
                    <audio
                      controls preload="none" src={v.sample_url}
                      style={{ height: 28, maxWidth: 180 }}
                      onClick={(e) => e.stopPropagation()}
                    />
                  )}
                </div>
              ))
            )}
          </div>
          {viduSelected && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              已选：<Text strong>{viduSelected.name}</Text>（{viduSelected.voice_id}）
            </Text>
          )}
          <Input.TextArea
            rows={2}
            maxLength={500}
            showCount
            placeholder="介绍文本（TTS + 克隆共用此文本作为试听内容）"
            value={viduText}
            onChange={(e) => setViduText(e.target.value)}
          />
          <Input
            placeholder="平台音色名称（如：平台·温柔女声；留空自动生成）"
            value={viduName}
            onChange={(e) => setViduName(e.target.value)}
            maxLength={64}
          />
        </Space>
      </Modal>
    </div>
  )
}

export default AdminVoices
