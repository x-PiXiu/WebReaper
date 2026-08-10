import { useState, useEffect } from 'react'
import { Typography, Button, Input, Select, Radio, Space, Tag, message, Row, Col, Empty, Table, Progress, Tabs } from 'antd'
import { VideoCameraOutlined, SoundOutlined, PictureOutlined, MergeCellsOutlined, RocketOutlined, ThunderboltOutlined, UploadOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { VideoTask } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input

const statusMeta: Record<string, { color: string; label: string }> = {
  pending: { color: 'default', label: '排队中' },
  generating: { color: 'processing', label: '生成中' },
  ready: { color: 'success', label: '已完成' },
  failed: { color: 'error', label: '失败' },
}

// 媒体创作工作台：视频生成 / 音频生成 / 图片生成 三个独立功能点 + 整合（预留）。
// 每个功能独立使用，不做流水线编排；整合依赖 goffmpeg（后续接入）。
export default function VideoWorkbench() {
  const queryClient = useQueryClient()
  const [activeTool, setActiveTool] = useState('video')

  // ---- 视频生成 ----
  const [mode, setMode] = useState<'text' | 'material'>('text')
  const [prompt, setPrompt] = useState('')
  const [videoList, setVideoList] = useState<VideoTask[]>([])

  // ---- 音频生成 ----
  const [audioText, setAudioText] = useState('')
  const [audioList, setAudioList] = useState<{ id: string; text: string; status: string }[]>([])

  // ---- 图片生成 ----
  const [imgPrompt, setImgPrompt] = useState('')
  const [imgList, setImgList] = useState<{ id: string; prompt: string; status: string }[]>([])

  // 视频任务列表（轮询进行中任务）
  const { data: tasks = [] } = useQuery({
    queryKey: ['video-tasks'],
    queryFn: () => businessApi.listVideoTasks().catch(() => []),
    refetchInterval: (query) => {
      const hasActive = (query.state.data || []).some((t: VideoTask) => ['generating', 'pending'].includes(t.status))
      return hasActive ? 5000 : false
    },
  })

  useEffect(() => { setVideoList(tasks as VideoTask[]) }, [tasks])

  const submitMutation = useMutation({
    mutationFn: () => businessApi.submitVideoTask({
      mode,
      prompt: mode === 'text' ? prompt : undefined,
      material_url: mode === 'material' ? prompt : undefined,
    }),
    onSuccess: () => {
      message.success('视频生成任务已提交')
      setPrompt('')
      queryClient.invalidateQueries({ queryKey: ['video-tasks'] })
    },
    onError: () => message.error('提交失败：视频生成服务暂未开通（后端 API 待挂载）'),
  })

  const genAudio = () => {
    if (audioText.trim().length < 2) { message.warning('请输入配音文本'); return }
    setAudioList((l) => [{ id: 'audio-' + Date.now(), text: audioText, status: 'pending' }, ...l])
    message.info('音频生成服务接入中（TTS 待配置）')
    setAudioText('')
  }

  const genImage = () => {
    if (imgPrompt.trim().length < 2) { message.warning('请输入图片提示词'); return }
    setImgList((l) => [{ id: 'img-' + Date.now(), prompt: imgPrompt, status: 'pending' }, ...l])
    message.info('图片生成服务接入中（模型待配置）')
    setImgPrompt('')
  }

  const canSubmitVideo = mode === 'text' ? prompt.trim().length >= 5 : prompt.trim().length > 0

  const videoColumns = [
    {
      title: '提示词', dataIndex: 'prompt', key: 'prompt', ellipsis: true,
      render: (p: string, r: VideoTask) => (
        <Space direction="vertical" size={0}>
          <Text style={{ fontSize: 13 }}>{p?.slice(0, 40) || r.mode}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{r.id}</Text>
        </Space>
      ),
    },
    {
      title: '模式', dataIndex: 'mode', key: 'mode', width: 80,
      render: (m: string) => <Tag>{m === 'text' ? '文生' : '素材'}</Tag>,
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 110,
      render: (s: string) => {
        const meta = statusMeta[s] || { color: 'default', label: s }
        return <Tag color={meta.color}>{meta.label}</Tag>
      },
    },
    {
      title: '进度', key: 'progress', width: 120,
      render: (_: unknown, r: VideoTask) => {
        if (r.status === 'ready') return <Text style={{ fontSize: 12, color: 'var(--wr-success)' }}>✓ 完成</Text>
        if (r.status === 'failed') return <Text type="danger" style={{ fontSize: 12 }}>{r.error?.slice(0, 20) || '失败'}</Text>
        return <Progress percent={r.status === 'generating' ? 60 : 20} size="small" showInfo={false} />
      },
    },
  ]

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* 页面标题 */}
        <div className="wr-page-header">
          <h1>媒体创作工作台</h1>
          <p>视频生成 · 音频生成 · 图片生成——独立功能点，随用随取</p>
        </div>

        <div className="wr-glass-card" style={{ padding: '8px 24px 24px' }}>
          <Tabs
            activeKey={activeTool}
            onChange={setActiveTool}
            items={[
              /* ---- ① 视频生成 ---- */
              {
                key: 'video',
                label: <Space><VideoCameraOutlined />视频生成</Space>,
                children: (
                  <Row gutter={[16, 16]}>
                    <Col xs={24} lg={9}>
                      <div style={{ padding: 20, borderRadius: 12, border: '1px solid var(--wr-border)', background: 'var(--wr-bg-elevated)' }}>
                        <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>输入方式</Text>
                        <Radio.Group value={mode} onChange={(e) => setMode(e.target.value)} style={{ marginBottom: 16 }}>
                          <Radio.Button value="text"><ThunderboltOutlined /> 文本生成</Radio.Button>
                          <Radio.Button value="material"><UploadOutlined /> 上传素材</Radio.Button>
                        </Radio.Group>
                        <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>
                          {mode === 'text' ? '视频提示词' : '素材描述'}
                        </Text>
                        <TextArea
                          value={prompt}
                          onChange={(e) => setPrompt(e.target.value)}
                          placeholder={mode === 'text'
                            ? '如：一间充满自然光的现代咖啡店，镜头缓缓推进吧台，咖啡师正在拉花，电影质感'
                            : '描述你的素材内容，AI 将据此生成视频'}
                          autoSize={{ minRows: 4, maxRows: 7 }}
                          style={{ marginBottom: 16 }}
                        />
                        <Button
                          type="primary" block size="large" icon={<RocketOutlined />}
                          disabled={!canSubmitVideo}
                          loading={submitMutation.isPending}
                          onClick={() => submitMutation.mutate()}
                        >
                          生成视频
                        </Button>
                      </div>
                    </Col>
                    <Col xs={24} lg={15}>
                      <Table
                        dataSource={videoList}
                        columns={videoColumns}
                        rowKey="id"
                        size="small"
                        pagination={false}
                        locale={{ emptyText: '暂无视频任务——输入提示词开始生成' }}
                      />
                    </Col>
                  </Row>
                ),
              },
              /* ---- ② 音频生成 ---- */
              {
                key: 'audio',
                label: <Space><SoundOutlined />音频生成</Space>,
                children: (
                  <Row gutter={[16, 16]}>
                    <Col xs={24} lg={9}>
                      <div style={{ padding: 20, borderRadius: 12, border: '1px solid var(--wr-border)', background: 'var(--wr-bg-elevated)' }}>
                        <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>配音文本</Text>
                        <TextArea
                          value={audioText}
                          onChange={(e) => setAudioText(e.target.value)}
                          placeholder="输入需要配音的文案，如：欢迎光临本店，我们是专注 10 年的老牌装修公司……"
                          autoSize={{ minRows: 4, maxRows: 7 }}
                          style={{ marginBottom: 12 }}
                        />
                        <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>音色（预留）</Text>
                        <Select
                          style={{ width: '100%', marginBottom: 16 }}
                          placeholder="选择音色（TTS 服务接入后可用）"
                          options={[{ value: 'default', label: '默认女声（待接入）' }, { value: 'male', label: '男声（待接入）' }]}
                        />
                        <Button type="primary" block size="large" icon={<SoundOutlined />} onClick={genAudio}>
                          生成配音
                        </Button>
                      </div>
                    </Col>
                    <Col xs={24} lg={15}>
                      {audioList.length === 0 ? (
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无音频——输入文本生成配音" style={{ padding: '60px 0' }} />
                      ) : (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                          {audioList.map((a) => (
                            <div key={a.id} style={{
                              padding: '12px 16px', borderRadius: 10,
                              border: '1px solid var(--wr-border)', background: 'var(--wr-bg-elevated)',
                              display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12,
                            }}>
                              <Text style={{ fontSize: 13, flex: 1 }} ellipsis>{a.text}</Text>
                              <Tag color={statusMeta[a.status]?.color || 'default'}>{statusMeta[a.status]?.label || a.status}</Tag>
                            </div>
                          ))}
                        </div>
                      )}
                    </Col>
                  </Row>
                ),
              },
              /* ---- ③ 图片生成 ---- */
              {
                key: 'image',
                label: <Space><PictureOutlined />图片生成</Space>,
                children: (
                  <Row gutter={[16, 16]}>
                    <Col xs={24} lg={9}>
                      <div style={{ padding: 20, borderRadius: 12, border: '1px solid var(--wr-border)', background: 'var(--wr-bg-elevated)' }}>
                        <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>图片提示词</Text>
                        <TextArea
                          value={imgPrompt}
                          onChange={(e) => setImgPrompt(e.target.value)}
                          placeholder="如：极简风格咖啡店门头，夜晚暖光，电影感，4K"
                          autoSize={{ minRows: 4, maxRows: 7 }}
                          style={{ marginBottom: 16 }}
                        />
                        <Button type="primary" block size="large" icon={<PictureOutlined />} onClick={genImage}>
                          生成图片
                        </Button>
                      </div>
                    </Col>
                    <Col xs={24} lg={15}>
                      {imgList.length === 0 ? (
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无图片——输入提示词生成" style={{ padding: '60px 0' }} />
                      ) : (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                          {imgList.map((im) => (
                            <div key={im.id} style={{
                              padding: '12px 16px', borderRadius: 10,
                              border: '1px solid var(--wr-border)', background: 'var(--wr-bg-elevated)',
                              display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12,
                            }}>
                              <Text style={{ fontSize: 13, flex: 1 }} ellipsis>{im.prompt}</Text>
                              <Tag color={statusMeta[im.status]?.color || 'default'}>{statusMeta[im.status]?.label || im.status}</Tag>
                            </div>
                          ))}
                        </div>
                      )}
                    </Col>
                  </Row>
                ),
              },
              /* ---- ④ 整合（预留）---- */
              {
                key: 'merge',
                label: <Space><MergeCellsOutlined />整合</Space>,
                children: (
                  <div className="wr-empty-hero" style={{ marginTop: 8 }}>
                    <div style={{
                      width: 64, height: 64, borderRadius: 18,
                      background: 'var(--wr-gradient)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                      fontSize: 28, color: '#fff', marginBottom: 16, opacity: 0.9,
                    }}>
                      <MergeCellsOutlined />
                    </div>
                    <h3 style={{ margin: '0 0 8px', fontWeight: 600, fontSize: 17 }}>音视频整合</h3>
                    <Text type="secondary" style={{ maxWidth: 420, textAlign: 'center' }}>
                      将生成的视频、音频、图片整合为成片。计划基于 goffmpeg 实现——后续版本开放。
                    </Text>
                    <Tag style={{ marginTop: 16 }} color="processing">即将上线</Tag>
                  </div>
                ),
              },
            ]}
          />
        </div>
      </div>
    </div>
  )
}
