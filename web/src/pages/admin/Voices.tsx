import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Alert, Button, Input, Modal, Popconfirm, Segmented, Space, Switch,
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

function AdminVoices() {
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

  const { data, isLoading } = useQuery({
    queryKey: ['admin-voices', scope, q],
    queryFn: () => businessApi.adminListVoices({ scope, q }).then((r) => r.voices),
  })

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin-voices'] })

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
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <h1>官方音色</h1>
          <p className="ip-lead">运营复刻平台音色（样本+介绍文本）——用户端「平台精选」置顶展示</p>
        </div>
        <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          创建官方音色
        </Button>
      </div>

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
            title: '状态', dataIndex: 'status', width: 90, render: (st: string) => (
              <Tag color={st === 'active' ? 'green' : 'default'}>{st === 'active' ? '启用' : st === 'deleted' ? '已删' : '停用'}</Tag>
            ),
          },
          {
            title: '试听', width: 90, render: (_, v) => v.sample_url ? (
              <audio controls preload="none" src={v.sample_url} style={{ height: 32, maxWidth: 200 }} />
            ) : <Text type="secondary">无</Text>,
          },
          {
            title: '操作', width: 150, render: (_, v) => (
              <Space size={4}>
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
    </div>
  )
}

export default AdminVoices
