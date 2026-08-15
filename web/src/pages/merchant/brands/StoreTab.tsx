import { useState } from 'react'
import { Card, Table, Tag, Space, Button, Modal, Form, Input, InputNumber, message, Popconfirm, Empty, Typography } from 'antd'
import { PlusOutlined, EnvironmentOutlined, ReloadOutlined, DeleteOutlined, RadarChartOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'
import type { Brand, StoreLocation } from '../../../types/api'

const { Text } = Typography

// 门店档案 Tab（品牌工作区"门店与附近"——立身份的 NAP 事实层）。
// 简版管理：地址/电话/营业时间/人均 + 地理编码状态；双榜对比/AI 探查在「附近同行」页。
export default function StoreTab({ brand }: { brand: Brand }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<StoreLocation | null>(null)
  const [form] = Form.useForm()

  const { data: stores = [], isLoading } = useQuery({
    queryKey: ['geo-stores', brand.id],
    queryFn: () => businessApi.listStoreLocations(brand.id),
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['geo-stores', brand.id] })

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setModalOpen(true)
  }
  const openEdit = (s: StoreLocation) => {
    setEditing(s)
    form.setFieldsValue({
      name: s.name,
      address: s.address,
      phone: s.phone,
      hours: s.hours,
      price_level: s.price_level,
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    try {
      if (editing) {
        await businessApi.updateStoreLocation(brand.id, editing.id, values)
        message.success('门店已更新（地址变更已重新定位）')
      } else {
        await businessApi.createStoreLocation(brand.id, values)
        message.success('门店已创建（自动地理编码定位）')
      }
      setModalOpen(false)
      invalidate()
    } catch { /* 拦截器已提示 */ }
  }

  const handleDelete = async (id: string) => {
    try {
      await businessApi.deleteStoreLocation(brand.id, id)
      message.success('已删除')
      invalidate()
    } catch { /* 拦截器已提示 */ }
  }

  const handleReGeocode = async (id: string) => {
    try {
      await businessApi.reGeocodeStoreLocation(brand.id, id)
      message.success('已重新定位')
      invalidate()
    } catch { /* 拦截器已提示 */ }
  }

  const geoTag = (s: StoreLocation) => {
    if (s.geo_status === 'ok') return <Tag color="success" style={{ margin: 0 }}>已定位</Tag>
    if (s.geo_status === 'failed') return <Tag color="error" style={{ margin: 0 }}>定位失败</Tag>
    return <Tag color="warning" style={{ margin: 0 }}>待定位</Tag>
  }

  return (
    <Card className="wr-glass-card" styles={{ body: { padding: 16 } }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 8 }}>
        <Space>
          <EnvironmentOutlined style={{ color: 'var(--wr-success)' }} />
          <Text strong style={{ fontSize: 14 }}>门店档案（NAP 事实）</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>地址/电话/营业时间是 AI 本地回答的引用地基</Text>
        </Space>
        <Space>
          <Button size="small" icon={<PlusOutlined />} type="primary" ghost onClick={openCreate}>添加门店</Button>
          <Button size="small" icon={<RadarChartOutlined />} onClick={() => navigate('/m/nearby')}>附近同行双榜 →</Button>
        </Space>
      </div>

      {stores.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={isLoading ? '加载中...' : '暂无门店——添加真实地址后 AI 本地回答才能引用你的 NAP'}>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>添加第一家门店</Button>
        </Empty>
      ) : (
        <Table
          dataSource={stores}
          rowKey="id"
          size="small"
          loading={isLoading}
          pagination={false}
          columns={[
            {
              title: '地址', dataIndex: 'address', key: 'address',
              render: (a: string, r: StoreLocation) => (
                <Space direction="vertical" size={0}>
                  <Text style={{ fontSize: 13 }}>{a}</Text>
                  {r.name && <Text type="secondary" style={{ fontSize: 11 }}>{r.name}</Text>}
                </Space>
              ),
            },
            {
              title: '电话', dataIndex: 'phone', key: 'phone', width: 130,
              render: (p: string) => <Text style={{ fontSize: 12 }}>{p || '-'}</Text>,
            },
            {
              title: '营业时间', dataIndex: 'hours', key: 'hours', width: 150,
              render: (h: string) => <Text type="secondary" style={{ fontSize: 12 }}>{h || '-'}</Text>,
            },
            {
              title: '定位', key: 'geo', width: 100, align: 'center',
              render: (_: unknown, r: StoreLocation) => geoTag(r),
            },
            {
              title: '操作', key: 'action', width: 180,
              render: (_: unknown, r: StoreLocation) => (
                <Space size={0}>
                  <Button size="small" type="link" onClick={() => openEdit(r)}>编辑</Button>
                  <Button size="small" type="link" icon={<ReloadOutlined />} onClick={() => handleReGeocode(r.id)}>重新定位</Button>
                  <Popconfirm title="删除该门店？" onConfirm={() => handleDelete(r.id)}>
                    <Button size="small" type="text" danger icon={<DeleteOutlined />} />
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
      )}

      <Modal
        title={editing ? '编辑门店' : '添加门店'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        okText={editing ? '保存' : '创建'}
        width={480}
      >
        <Form form={form} layout="vertical" requiredMark={false} style={{ marginTop: 8 }}>
          <Form.Item label="门店名" name="name">
            <Input placeholder="如 蜀香居·春熙路店（可选）" />
          </Form.Item>
          <Form.Item label="地址" name="address" rules={[{ required: true, message: '请输入真实地址' }]}>
            <Input placeholder="如 成都市锦江区春熙路 100 号" />
          </Form.Item>
          <Form.Item label="电话" name="phone">
            <Input placeholder="如 028-88888888" />
          </Form.Item>
          <Form.Item label="营业时间" name="hours">
            <Input placeholder="如 10:00-22:00" />
          </Form.Item>
          <Form.Item label="人均消费（元）" name="price_level">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="如 80" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
