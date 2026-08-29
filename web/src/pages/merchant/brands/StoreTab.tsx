import { useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Row, Col, AutoComplete, Space, Table, Tag, Typography } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, EnvironmentOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'
import type { Brand, StoreLocation } from '../../../types/api'
import QueryBoundary from '../../../components/QueryBoundary'
import { message } from '../../../utils/antdApp'

const { Text } = Typography

function geoStatusTag(s: StoreLocation) {
  if (s.geo_status === 'ok') return <Tag color="success">已定位</Tag>
  if (s.geo_status === 'failed') return <Tag color="error">定位失败</Tag>
  return <Tag color="warning">待定位</Tag>
}

/**
 * 门店档案 Tab（品牌档案·输入之家——从原「附近同行」页拆回）。
 * 轻量版：地址联想 / 电话 / 营业时间——AI 本地回答引用你的地基事实；
 * 双榜数据视图在「AI 体检 · 体检记录 · 附近对比」。
 */
export default function StoreTab({ brand }: { brand: Brand }) {
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<StoreLocation | null>(null)
  const [suggestOptions, setSuggestOptions] = useState<{ value: string; address?: string }[]>([])
  const [form] = Form.useForm()

  const { data: stores = [], isLoading, isError, refetch } = useQuery({
    queryKey: ['geo-stores', brand.id],
    queryFn: () => businessApi.listStoreLocations(brand.id),
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['geo-stores', brand.id] })

  // 地址联想（输入 2 字以上防抖查询；联想失败静默降级为手输）
  let suggestTimer: ReturnType<typeof setTimeout> | null = null
  const handleSuggest = (q: string) => {
    if (suggestTimer) clearTimeout(suggestTimer)
    if (q.trim().length < 2) {
      setSuggestOptions([])
      return
    }
    suggestTimer = setTimeout(async () => {
      try {
        const tips = await businessApi.suggestLocations(q.trim())
        setSuggestOptions(tips.map((t) => ({
          value: t.name + (t.address ? `（${t.address}）` : ''),
          address: t.address || t.name,
        })))
      } catch {
        setSuggestOptions([])
      }
    }, 300)
  }

  const createMut = useMutation({
    mutationFn: (v: { name?: string; address: string; phone?: string; hours?: string; price_level?: string }) =>
      businessApi.createStoreLocation(brand.id, v),
    onSuccess: () => {
      message.success('门店已创建（自动地图定位）')
      setModalOpen(false)
      form.resetFields()
      invalidate()
    },
  })

  const updateMut = useMutation({
    mutationFn: (v: { id: string; data: { name?: string; address: string; phone?: string; hours?: string; price_level?: string } }) =>
      businessApi.updateStoreLocation(brand.id, v.id, v.data),
    onSuccess: () => {
      message.success('门店已更新（地址变更已重新定位）')
      setModalOpen(false)
      form.resetFields()
      invalidate()
    },
  })

  const deleteMut = useMutation({
    mutationFn: (storeId: string) => businessApi.deleteStoreLocation(brand.id, storeId),
    onSuccess: () => {
      message.success('门店已删除')
      invalidate()
    },
  })

  const reGeoMut = useMutation({
    mutationFn: (storeId: string) => businessApi.reGeocodeStoreLocation(brand.id, storeId),
    onSuccess: (loc: StoreLocation) => {
      message.success(loc.geo_status === 'ok' ? '定位成功' : '定位失败（可修改地址后重试）')
      invalidate()
    },
  })

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setModalOpen(true)
  }

  const openEdit = (s: StoreLocation) => {
    setEditing(s)
    form.setFieldsValue({ name: s.name, address: s.address, phone: s.phone, hours: s.hours, price_level: s.price_level })
    setModalOpen(true)
  }

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      if (editing) {
        updateMut.mutate({ id: editing.id, data: values })
      } else {
        createMut.mutate(values)
      }
    })
  }

  return (
    <Card
      className="wr-glass-card"
      title={<Space><EnvironmentOutlined />门店档案</Space>}
      extra={
        <Space>
          <Text type="secondary" style={{ fontSize: 12 }}>地址/电话/营业时间是 AI 本地回答引用你的地基</Text>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建门店</Button>
        </Space>
      }
    >
      <Text type="secondary" style={{ display: 'block', fontSize: 12.5, marginBottom: 12 }}>
        门店建好后，本地发布可带定位；竞品推荐也可按门店周边给出候选。
      </Text>
      <QueryBoundary
        loading={isLoading}
        error={isError}
        onRetry={() => refetch()}
        empty={stores.length === 0}
        emptyText="还没有门店——添加真实地址后，AI 推荐附近商家才会带上你"
        emptyExtra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>添加第一家门店</Button>}
      >
        <Table
          dataSource={stores}
          rowKey="id"
          size="small"
          pagination={false}
          columns={[
            { title: '门店', dataIndex: 'name', render: (n, r) => <Text strong>{n || r.address}</Text> },
            { title: '地址', dataIndex: 'address', ellipsis: true },
            { title: '定位', dataIndex: 'geo_status', width: 100, render: (_, s) => geoStatusTag(s) },
            { title: '营业时间', dataIndex: 'hours', width: 120, render: (v) => v || '—' },
            { title: '电话', dataIndex: 'phone', width: 130, render: (v) => v || '—' },
            {
              title: '操作', key: 'op', width: 170,
              render: (_, s) => (
                <Space size={4}>
                  <Button size="small" onClick={() => openEdit(s)}>编辑</Button>
                  <Button size="small" icon={<ReloadOutlined />} onClick={() => reGeoMut.mutate(s.id)}>重定位</Button>
                  <Popconfirm title="删除该门店？" onConfirm={() => deleteMut.mutate(s.id)}>
                    <Button size="small" danger icon={<DeleteOutlined />} />
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
      </QueryBoundary>

      <Modal
        title={editing ? '编辑门店' : '新建门店'}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        confirmLoading={createMut.isPending || updateMut.isPending}
        okText="保存"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="门店名称" extra="留空则使用品牌名">
            <Input placeholder="如：望京店" />
          </Form.Item>
          <Form.Item name="address" label="详细地址" rules={[{ required: !editing, message: '请输入门店详细地址（用于地图定位）' }]}>
            <AutoComplete
              placeholder="如：北京市朝阳区望京街10号（输入有地址联想）"
              options={suggestOptions}
              onSearch={handleSuggest}
              onSelect={(_, opt) => {
                form.setFieldValue('address', opt.address || opt.value)
              }}
              filterOption={false}
            />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item name="hours" label="营业时间">
                <Input placeholder="如：10:00-22:00" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="phone" label="联系电话">
                <Input placeholder="如：010-12345678" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="price_level" label="人均消费档位">
            <Input placeholder="如：¥80/人" />
          </Form.Item>
          <Text type="secondary" style={{ fontSize: 12 }}>
            保存后将自动调用高德地图定位（未配置地图服务时标记"待定位"，可稍后重试）。地址/电话/营业时间会注入文章生成与公开页展示。
          </Text>
        </Form>
      </Modal>
    </Card>
  )
}
