import { useState, useEffect } from 'react'
import { Typography, Button, Input, Select, Radio, Space, Tag, Steps, message, Row, Col, Empty, Table, Progress } from 'antd'
import { VideoCameraOutlined, ThunderboltOutlined, UploadOutlined, SoundOutlined, RocketOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { VideoTask } from '../../types/api'

const { Title, Text } = Typography
const { TextArea } = Input

const statusMeta: Record<string, { color: string; label: string }> = {
  pending: { color: 'default', label: '排队中' },
  generating: { color: 'processing', label: '视频生成中' },
  dubbing: { color: 'processing', label: '配音中' },
  composing: { color: 'processing', label: '合成中' },
  ready: { color: 'success', label: '成片就绪' },
  failed: { color: 'error', label: '失败' },
}

const statusStepIndex: Record<string, number> = {
  pending: 0, generating: 1, dubbing: 2, composing: 3, ready: 4, failed: 1,
}

// 视频工作台：素材/文本 → Vidu 生成 → 配音 → 合成 → 发布视频平台。
// 流水线状态机：pending → generating → dubbing → composing → ready（可发布）
export default function VideoWorkbench() {
  const queryClient = useQueryClient()
  const [mode, setMode] = useState<'text' | 'material'>('text')
  const [prompt, setPrompt] = useState('')
  const [voiceText, setVoiceText] = useState('')
  const [platform, setPlatform] = useState('douyin')
  const [selectedTask, setSelectedTask] = useState<VideoTask | null>(null)

  // 任务列表 + 轮询（有进行中任务时 5s 刷新）
  const { data: tasks = [] } = useQuery({
    queryKey: ['video-tasks'],
    queryFn: () => businessApi.listVideoTasks(),
    refetchInterval: (query) => {
      const hasActive = (query.state.data || []).some((t: VideoTask) => ['generating', 'dubbing', 'composing', 'pending'].includes(t.status))
      return hasActive ? 5000 : false
    },
  })

  useEffect(() => {
    if (selectedTask) {
      const updated = tasks.find((t: VideoTask) => t.id === selectedTask.id)
      if (updated) setSelectedTask(updated)
    }
  }, [tasks, selectedTask])

  const submitMutation = useMutation({
    mutationFn: () => businessApi.submitVideoTask({
      mode,
      prompt: mode === 'text' ? prompt : undefined,
      material_url: mode === 'material' ? prompt : undefined,
      voice_text: voiceText,
    }),
    onSuccess: (task) => {
      message.success('视频生成任务已提交')
      setSelectedTask(task)
      queryClient.invalidateQueries({ queryKey: ['video-tasks'] })
    },
    onError: (e) => message.error('提交失败：' + ((e as Error)?.message || '服务暂未开通')),
  })

  const publishMutation = useMutation({
    mutationFn: (taskId: string) => businessApi.publishVideoTask({ task_id: taskId, platform }),
    onSuccess: () => {
      message.success(`已提交发布到 ${platform === 'douyin' ? '抖音' : '快手'}`)
      queryClient.invalidateQueries({ queryKey: ['video-tasks'] })
    },
    onError: (e) => message.error('发布失败：' + ((e as Error)?.message || '')),
  })

  const canSubmit = mode === 'text' ? prompt.trim().length >= 5 : prompt.trim().length > 0
  const isActive = ['pending', 'generating', 'dubbing', 'composing'].includes(selectedTask?.status || '')

  const columns = [
    {
      title: '任务', dataIndex: 'prompt', key: 'prompt', ellipsis: true,
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
      title: '进度', key: 'progress', width: 140,
      render: (_: unknown, r: VideoTask) => {
        if (r.status === 'ready') return <Text style={{ fontSize: 12, color: 'var(--wr-success)' }}>✓ 可发布</Text>
        if (r.status === 'failed') return <Text type="danger" style={{ fontSize: 12 }}>{r.error?.slice(0, 24)}</Text>
        return <Progress percent={statusStepIndex[r.status] * 25} size="small" showInfo={false} />
      },
    },
    {
      title: '操作', key: 'action', width: 90,
      render: (_: unknown, r: VideoTask) => (
        <Button size="small" type="link" onClick={() => setSelectedTask(r)}>查看</Button>
      ),
    },
  ]

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* 页面标题 */}
        <div className="wr-page-header">
          <h1>视频工作台</h1>
          <p>素材或文本 → AI 视频生成 → 配音 → 合成 → 发布抖音/快手</p>
        </div>

        {/* 左侧生成配置 + 右侧流水线预览 */}
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={24} lg={10}>
            <div className="wr-glass-card" style={{ padding: 24, height: '100%' }}>
              <Space style={{ marginBottom: 16 }} align="center">
                <div style={{
                  width: 36, height: 36, borderRadius: 10,
                  background: 'var(--wr-gradient)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 17, color: '#fff',
                }}>
                  <VideoCameraOutlined />
                </div>
                <Title level={5} style={{ margin: 0, fontSize: 15 }}>生成配置</Title>
              </Space>

              <div style={{ marginBottom: 16 }}>
                <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>输入方式</Text>
                <Radio.Group value={mode} onChange={(e) => setMode(e.target.value)} size="middle">
                  <Radio.Button value="text"><ThunderboltOutlined /> 文本生成</Radio.Button>
                  <Radio.Button value="material"><UploadOutlined /> 上传素材</Radio.Button>
                </Radio.Group>
              </div>

              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>
                {mode === 'text' ? '视频提示词' : '素材描述'}
              </Text>
              <TextArea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder={mode === 'text'
                  ? '如：一间充满自然光的现代咖啡店，镜头缓缓推进吧台，咖啡师正在拉花，电影质感'
                  : '描述你的素材内容，AI 将据此生成视频'}
                autoSize={{ minRows: 4, maxRows: 8 }}
                style={{ marginBottom: 16 }}
              />

              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>
                <SoundOutlined style={{ marginRight: 4 }} />配音文本（可选）
              </Text>
              <TextArea
                value={voiceText}
                onChange={(e) => setVoiceText(e.target.value)}
                placeholder="如：欢迎光临本店，我们是专注 10 年的老牌装修公司……"
                autoSize={{ minRows: 2, maxRows: 5 }}
                style={{ marginBottom: 20 }}
              />

              <Button
                type="primary" size="large" block
                icon={<RocketOutlined />}
                disabled={!canSubmit}
                loading={submitMutation.isPending}
                onClick={() => submitMutation.mutate()}
              >
                开始生成
              </Button>
            </div>
          </Col>

          <Col xs={24} lg={14}>
            <div className="wr-glass-card" style={{ padding: 24, height: '100%' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
                <Title level={5} style={{ margin: 0, fontSize: 15 }}>任务流水线</Title>
                {selectedTask && statusMeta[selectedTask.status] && (
                  <Tag color={statusMeta[selectedTask.status].color}>{statusMeta[selectedTask.status].label}</Tag>
                )}
              </div>

              {!selectedTask ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={
                    <div>
                      <Text type="secondary">提交任务后，这里将展示生成流水线进度</Text>
                      <div style={{ marginTop: 12 }}>
                        <Steps
                          size="small"
                          current={0}
                          items={[
                            { title: '生成', description: 'Vidu 文生视频' },
                            { title: '配音', description: 'TTS 语音合成' },
                            { title: '合成', description: '音视频合并' },
                            { title: '发布', description: '抖音 / 快手' },
                          ]}
                        />
                      </div>
                    </div>
                  }
                />
              ) : (
                <div>
                  <Steps
                    className="wr-steps"
                    size="small"
                    current={statusStepIndex[selectedTask.status] || 0}
                    status={selectedTask.status === 'failed' ? 'error' : 'process'}
                    items={[
                      { title: '生成', description: 'AI 视频生成' },
                      { title: '配音', description: 'TTS 语音' },
                      { title: '合成', description: '音视频合并' },
                      { title: '发布', description: '抖音 / 快手' },
                    ]}
                  />

                  {/* 产物区 */}
                  <div style={{ marginTop: 20, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                    {(selectedTask.video_url || selectedTask.final_url) ? (
                      <video
                        src={selectedTask.final_url || selectedTask.video_url}
                        controls
                        style={{ width: '100%', borderRadius: 12, border: '1px solid var(--wr-border)', background: '#000' }}
                      />
                    ) : (
                      <div style={{
                        aspectRatio: '16/9', borderRadius: 12, border: '1px solid var(--wr-border)',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        background: 'var(--wr-bg-elevated)', color: 'var(--wr-text-muted)', fontSize: 12,
                      }}>
                        {isActive ? <Progress type="circle" percent={statusStepIndex[selectedTask.status] * 25} size={48} /> : '暂无成片'}
                      </div>
                    )}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                      <div style={{
                        padding: 14, borderRadius: 10, border: '1px solid var(--wr-border)',
                        background: 'var(--wr-bg-elevated)',
                      }}>
                        <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>生成提示词</Text>
                        <Text style={{ fontSize: 12.5, lineHeight: 1.6 }}>{selectedTask.prompt}</Text>
                      </div>
                      {selectedTask.voice_text && (
                        <div style={{
                          padding: 14, borderRadius: 10, border: '1px solid var(--wr-border)',
                          background: 'var(--wr-bg-elevated)',
                        }}>
                          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>配音文本</Text>
                          <Text style={{ fontSize: 12.5, lineHeight: 1.6 }}>{selectedTask.voice_text}</Text>
                        </div>
                      )}
                      {selectedTask.status === 'failed' && (
                        <div style={{ padding: 14, borderRadius: 10, border: '1px solid var(--wr-danger)', color: 'var(--wr-danger)', fontSize: 12.5 }}>
                          {selectedTask.error}
                        </div>
                      )}
                      {/* 发布区（成片就绪后）*/}
                      {selectedTask.status === 'ready' && (
                        <div style={{
                          padding: 16, borderRadius: 10, border: '1px solid var(--wr-primary-border)',
                          background: 'var(--wr-primary-bg)',
                        }}>
                          <Text style={{ fontSize: 12.5, display: 'block', marginBottom: 10, color: 'var(--wr-text-primary)' }}>
                            成片就绪，发布到：
                          </Text>
                          <Space>
                            <Select
                              size="middle"
                              value={platform}
                              onChange={setPlatform}
                              style={{ width: 110 }}
                              options={[
                                { value: 'douyin', label: '抖音' },
                                { value: 'kuaishou', label: '快手' },
                              ]}
                            />
                            <Button
                              type="primary"
                              loading={publishMutation.isPending}
                              onClick={() => publishMutation.mutate(selectedTask.id)}
                            >
                              发布
                            </Button>
                          </Space>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>
          </Col>
        </Row>

        {/* 任务列表 */}
        <div className="wr-glass-card" style={{ padding: 8 }}>
          <Table
            dataSource={tasks}
            columns={columns}
            rowKey="id"
            size="small"
            pagination={{ pageSize: 8, size: 'small' }}
            locale={{ emptyText: '暂无视频任务——从左侧提交第一个生成任务' }}
          />
        </div>
      </div>
    </div>
  )
}
