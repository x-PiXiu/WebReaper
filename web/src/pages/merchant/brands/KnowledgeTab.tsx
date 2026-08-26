import { useState } from 'react'
import { Button, Card, Empty, Input, List, Modal, Popconfirm, Space, Spin, Tag, Typography, Upload } from 'antd'
import { UploadOutlined, DeleteOutlined, FileTextOutlined, PlusOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'
import { message } from '../../../utils/antdApp'

const { Text } = Typography
const { TextArea } = Input

interface KnowledgeMaterialView {
  id: string
  title: string
  summary: string
  has_vector: boolean
  crawl_keyword: string
  created_at: string
}

/**
 * 品牌知识库 Tab（品牌档案·输入之家——获客智能体转型新增）。
 * 商户上传品牌文档/粘贴文本 → 自动向量化 → 写文章和做视频时自动引用。
 * 格式：纯文本粘贴 + .txt/.md 文件上传。
 */
export default function KnowledgeTab({ brandId }: { brandId: string }) {
  const queryClient = useQueryClient()
  const [uploadOpen, setUploadOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [fileName, setFileName] = useState('')

  const { data: materials = [], isLoading } = useQuery({
    queryKey: ['brand-knowledge', brandId],
    queryFn: () => businessApi.listBrandKnowledge(brandId).then((r) => r.materials),
    enabled: !!brandId,
  })

  const uploadMut = useMutation({
    mutationFn: (data: { title: string; content: string }) => businessApi.uploadBrandKnowledge(brandId, data),
    onSuccess: (res) => {
      message.success(res.message || 'AI 已学习你的资料——写文章和做视频时会自动引用')
      setUploadOpen(false)
      setTitle('')
      setContent('')
      setFileName('')
      queryClient.invalidateQueries({ queryKey: ['brand-knowledge'] })
      queryClient.invalidateQueries({ queryKey: ['brand-knowledge-count'] })
    },
  })

  const deleteMut = useMutation({
    mutationFn: (materialId: string) => businessApi.deleteBrandKnowledge(brandId, materialId),
    onSuccess: () => {
      message.success('已删除')
      queryClient.invalidateQueries({ queryKey: ['brand-knowledge'] })
      queryClient.invalidateQueries({ queryKey: ['brand-knowledge-count'] })
    },
  })

  const handleFileRead = (file: File) => {
    setFileName(file.name)
    const reader = new FileReader()
    reader.onload = (e) => {
      setContent((e.target?.result as string) || '')
      if (!title) setTitle(file.name.replace(/\.(txt|md|markdown)$/i, ''))
    }
    reader.readAsText(file)
    return false
  }

  const handleUpload = () => {
    if (!title.trim()) { message.warning('请填写标题'); return }
    if (!content.trim() || content.trim().length < 50) {
      message.warning('内容太短（至少 50 字）——请粘贴有实质内容的品牌资料')
      return
    }
    uploadMut.mutate({ title: title.trim(), content: content.trim() })
  }

  return (
    <Card
      className="wr-glass-card"
      title={<Space><FileTextOutlined />知识库</Space>}
      extra={
        <Space>
          <Text type="secondary" style={{ fontSize: 12 }}>上传品牌资料，AI 写文章和做视频时自动引用</Text>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setUploadOpen(true)}>上传资料</Button>
        </Space>
      }
    >
      <Text type="secondary" style={{ display: 'block', fontSize: 12.5, marginBottom: 14 }}>
        把你的品牌资料（产品介绍/服务流程/客户案例/价格表等）上传到这里——
        AI 会自动学习，写文章和做视频时引用这些内容，确保专业准确。
      </Text>

      {isLoading ? <Spin /> : materials.length === 0 ? (
        <Empty description="还没有上传资料——AI 还不认识你的品牌细节">
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setUploadOpen(true)}>上传第一份资料</Button>
        </Empty>
      ) : (
        <List
          dataSource={materials}
          renderItem={(m: KnowledgeMaterialView) => (
            <List.Item
              actions={[
                <Popconfirm key="del" title="删除这份资料？" onConfirm={() => deleteMut.mutate(m.id)}>
                  <Button size="small" type="text" danger icon={<DeleteOutlined />}>删除</Button>
                </Popconfirm>,
              ]}
            >
              <List.Item.Meta
                title={
                  <Space size={8}>
                    <Text strong style={{ fontSize: 14 }}>{m.title}</Text>
                    {m.has_vector
                      ? <Tag color="success" style={{ fontSize: 10, margin: 0 }}>AI 已学习</Tag>
                      : <Tag color="warning" style={{ fontSize: 10, margin: 0 }}>学习中</Tag>}
                  </Space>
                }
                description={
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {m.summary?.slice(0, 100)}{m.summary?.length > 100 ? '...' : ''}
                  </Text>
                }
              />
            </List.Item>
          )}
        />
      )}

      <Modal
        title="上传品牌资料"
        open={uploadOpen}
        onCancel={() => setUploadOpen(false)}
        onOk={handleUpload}
        confirmLoading={uploadMut.isPending}
        okText="上传并让 AI 学习"
        okButtonProps={{ disabled: !title.trim() || content.trim().length < 50 }}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Upload beforeUpload={handleFileRead} accept=".txt,.md,.markdown" maxCount={1} showUploadList={false}>
            <Button icon={<UploadOutlined />}>选择 .txt / .md 文件</Button>
          </Upload>
          {fileName && <Tag color="blue">{fileName}</Tag>}
          <Input
            placeholder="标题（如：产品介绍 / 服务流程 / 客户案例）"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={100}
          />
          <TextArea
            rows={8}
            placeholder={'粘贴品牌资料内容（至少 50 字）……\n\n例如：\n- 我们的主打产品和特色\n- 服务流程和价格\n- 客户评价和案例\n- 品牌故事和理念'}
            value={content}
            onChange={(e) => setContent(e.target.value)}
            showCount
            maxLength={5000}
          />
          <Text type="secondary" style={{ fontSize: 12 }}>
            上传后 AI 会自动学习这份资料——写文章和做视频时会作为参考引用，确保内容专业准确。
          </Text>
        </Space>
      </Modal>
    </Card>
  )
}
