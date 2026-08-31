import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Alert, Button, Image, Input, InputNumber, Modal, Popconfirm, Segmented,
  Space, Switch, Table, Tag, Typography, Upload,
} from 'antd'
import { PlusOutlined, UserOutlined, EnvironmentOutlined } from '@ant-design/icons'
import { businessApi } from '../../api/business'
import { message } from '../../utils/antdApp'
import type { SubjectAsset } from '../../types/api'

const { Text } = Typography

/**
 * 官方主体管理（25 号二′b 平台资产收官）：
 * 创建（上传形象照 → 服务端 Vidu 注册 + 自动链式 10s 形象视频 → subject_assets scope=official）
 * + 列表（person/scene 过滤）+ 编辑（名称/标签/排序/上下架）+ 删除。
 * 用户端数字资产页「官方资产」区即显（/api/v1/subjects scope=official）。
 */
function AdminSubjects() {
  const queryClient = useQueryClient()
  const [kind, setKind] = useState<'all' | 'person' | 'scene'>('all')
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [createKind, setCreateKind] = useState<'person' | 'scene'>('person')
  const [tags, setTags] = useState('')
  const [images, setImages] = useState<Array<{ id: string; url: string }>>([])
  const [uploading, setUploading] = useState(false)
  const [editing, setEditing] = useState<SubjectAsset | null>(null)
  const [editName, setEditName] = useState('')
  const [editTags, setEditTags] = useState('')
  const [editSort, setEditSort] = useState(0)
  const [editActive, setEditActive] = useState(true)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-subjects', kind],
    queryFn: () => businessApi.adminListSubjects(
      kind === 'all' ? { limit: 100 } : { kind, limit: 100 },
    ),
  })

  const subjects = useMemo(() => {
    const list = data?.subjects ?? []
    return kind === 'all' ? list : list.filter((s) => s.kind === kind)
  }, [data, kind])

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin-subjects'] })

  const doCreate = async () => {
    if (!name.trim()) { message.warning('请填写主体名称'); return }
    if (images.length === 0) { message.warning('请上传 1-3 张形象照'); return }
    setCreating(true)
    try {
      const r = await businessApi.adminCreateSubject({
        name: name.trim(),
        images: images.map((i) => i.url),
        kind: createKind,
        tags: tags.trim() || undefined,
      })
      message.success(`官方主体「${r.name}」已创建——形象视频自动生成中，用户端官方区即显`)
      setCreateOpen(false)
      setName('')
      setTags('')
      setImages([])
      setCreateKind('person')
      refresh()
    } catch { /* 拦截器已提示 */ } finally {
      setCreating(false)
    }
  }

  const doUpdate = async () => {
    if (!editing) return
    try {
      await businessApi.adminUpdateSubject(editing.id, {
        name: editName.trim() || undefined,
        tags: editTags.trim() || undefined,
        sort_order: editSort,
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
          <h1>官方主体</h1>
          <p className="ip-lead">运营制作官方数字分身/环境模板——用户端「官方资产」区即选即用</p>
        </div>
        <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          创建官方主体
        </Button>
      </div>

      <Alert
        type="info" showIcon style={{ marginBottom: 16 }}
        message="创建流程：上传形象照 → 服务端 Vidu 注册主体（同步）→ 自动链式生成 10s 形象视频 → 入库官方资产"
        description="建议起步 8-12 个人物主体（男女 × 青年/中年 × 亲和/专业）；环境模板作兜底（用户自己的店才是主角）。"
      />

      <div className="ip-toolbar">
        <Segmented
          value={kind}
          onChange={(v) => setKind(v as typeof kind)}
          options={[
            { value: 'all', label: `全部 ${data?.total ?? subjects.length}` },
            { value: 'person', label: '人物分身' },
            { value: 'scene', label: '环境模板' },
          ]}
        />
      </div>

      <Table<SubjectAsset>
        rowKey="id"
        loading={isLoading}
        dataSource={subjects}
        pagination={{ pageSize: 20, showTotal: (t) => `共 ${t} 条` }}
        columns={[
          {
            title: '主体', dataIndex: 'name', render: (_, s) => (
              <Space>
                {s.portrait_url ? (
                  <Image src={s.portrait_url} width={48} height={48} style={{ borderRadius: 8, objectFit: 'cover' }} preview={false} />
                ) : (
                  <span style={{ width: 48, height: 48, borderRadius: 8, background: 'var(--wr-primary-bg)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>
                    {s.kind === 'scene' ? <EnvironmentOutlined /> : <UserOutlined />}
                  </span>
                )}
                <Space direction="vertical" size={2}>
                  <Text strong>{s.name}</Text>
                  <Text type="secondary" style={{ fontSize: 12 }} copyable={{ text: s.server_id }}>{s.server_id.slice(0, 20)}{s.server_id.length > 20 ? '…' : ''}</Text>
                </Space>
              </Space>
            ),
          },
          {
            title: '类型', dataIndex: 'kind', width: 90, render: (k: string) => (
              <Tag color={k === 'scene' ? 'cyan' : 'geekblue'}>{k === 'scene' ? '环境' : '人物'}</Tag>
            ),
          },
          { title: '标签', dataIndex: 'tags', width: 130, render: (t: string) => t || <Text type="secondary">-</Text> },
          { title: '排序', dataIndex: 'sort_order', width: 70 },
          {
            title: '状态', dataIndex: 'status', width: 90, render: (st: string) => (
              <Tag color={st === 'active' ? 'green' : 'default'}>{st === 'active' ? '上架' : '下架'}</Tag>
            ),
          },
          {
            title: '形象视频', width: 110, render: (_, s) => s.avatar_video_url ? (
              <video controls preload="none" src={s.avatar_video_url} style={{ height: 40, maxWidth: 140 }} />
            ) : (
              <Text type="secondary" style={{ fontSize: 12 }}>生成中/无</Text>
            ),
          },
          {
            title: '操作', width: 140, render: (_, s) => (
              <Space size={4}>
                <Button size="small" onClick={() => {
                  setEditing(s)
                  setEditName(s.name)
                  setEditTags(s.tags || '')
                  setEditSort(s.sort_order || 0)
                  setEditActive(s.status !== 'disabled')
                }}>编辑</Button>
                <Popconfirm
                  title="删除该官方主体？"
                  description="仅删本地资产记录，Vidu 侧主体不受影响"
                  okText="删除" okButtonProps={{ danger: true }} cancelText="取消"
                  onConfirm={async () => {
                    try {
                      await businessApi.adminDeleteSubject(s.id)
                      message.success('已删除')
                      refresh()
                    } catch { /* 拦截器已提示 */ }
                  }}
                >
                  <Button size="small" danger>删除</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        open={createOpen}
        title="创建官方主体"
        okText="创建" cancelText="取消"
        confirmLoading={creating}
        onOk={doCreate}
        onCancel={() => setCreateOpen(false)}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Segmented
            value={createKind}
            onChange={(v) => setCreateKind(v as 'person' | 'scene')}
            options={[
              { value: 'person', label: '人物分身', icon: <UserOutlined /> },
              { value: 'scene', label: '环境模板', icon: <EnvironmentOutlined /> },
            ]}
          />
          <Input placeholder="主体名称（如：官方·亲和女店主）" value={name} onChange={(e) => setName(e.target.value)} maxLength={64} />
          <div>
            <Text strong>形象照（1-3 张）</Text>
            <Upload
              listType="picture-card"
              maxCount={3}
              accept="image/png,image/jpeg,image/jpg,image/webp"
              customRequest={async ({ file, onSuccess, onError }) => {
                setUploading(true)
                try {
                  const r = await businessApi.uploadAsset(file as File)
                  setImages((prev) => [...prev, { id: r.id, url: r.url }])
                  onSuccess?.(r)
                } catch (e) { onError?.(e as Error) } finally { setUploading(false) }
              }}
              onRemove={(file) => {
                const id = (file.response as { id?: string } | undefined)?.id
                const url = (file.response as { url?: string } | undefined)?.url
                if (id || url) setImages((prev) => prev.filter((a) => a.id !== id && a.url !== url))
              }}
            >
              {images.length < 3 && !uploading && (
                <div><PlusOutlined /><div style={{ fontSize: 12, marginTop: 4 }}>上传形象照</div></div>
              )}
            </Upload>
          </div>
          <Input placeholder="标签（如：女,青年,亲和——逗号分隔，可选）" value={tags} onChange={(e) => setTags(e.target.value)} maxLength={128} />
          <Text type="secondary" style={{ fontSize: 12 }}>
            创建成功后自动链式生成 10s 形象视频（消耗平台 Vidu 积分），完成后用户端官方区展示并可预览。
          </Text>
        </Space>
      </Modal>

      <Modal
        open={!!editing}
        title={`编辑主体 · ${editing?.name ?? ''}`}
        okText="保存" cancelText="取消"
        onOk={doUpdate}
        onCancel={() => setEditing(null)}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Input placeholder="名称" value={editName} onChange={(e) => setEditName(e.target.value)} maxLength={64} />
          <Input placeholder="标签（逗号分隔）" value={editTags} onChange={(e) => setEditTags(e.target.value)} maxLength={128} />
          <Space>
            <Text>排序</Text>
            <InputNumber min={0} max={999} value={editSort} onChange={(v) => setEditSort(v ?? 0)} />
            <Text type="secondary" style={{ fontSize: 12 }}>（小的靠前）</Text>
          </Space>
          <Space>
            <Switch checked={editActive} onChange={setEditActive} />
            <Text>{editActive ? '上架（用户端可见）' : '下架（用户端隐藏）'}</Text>
          </Space>
        </Space>
      </Modal>
    </div>
  )
}

export default AdminSubjects
