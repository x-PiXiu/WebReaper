import { useState } from 'react'
import { Typography, Card, Space, message, Button, Input, Select, Upload, Tag, Steps } from 'antd'
import { ThunderboltOutlined, UploadOutlined, SendOutlined, FileImageOutlined, AudioOutlined, VideoCameraOutlined } from '@ant-design/icons'
import { useQuery, useMutation } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { GenerationTemplate, MediaAsset } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input
const { Dragger } = Upload

// 统一提交页面（傻瓜式：用户不需要选择端点/模型）
export default function UnifiedSubmit() {
  const [brandId, setBrandId] = useState('')
  const [text, setText] = useState('')
  const [selectedTemplate, setSelectedTemplate] = useState<string>('')
  const [selectedMaterials, setSelectedMaterials] = useState<string[]>([])
  const [duration, setDuration] = useState<number>(0)
  const [quality, setQuality] = useState<string>('720p')

  // 查询可用模板
  const { data: templates = [] } = useQuery({
    queryKey: ['generation-templates'],
    queryFn: () => businessApi.listTemplates(),
  })

  // 查询素材库
  const { data: assets = [] } = useQuery({
    queryKey: ['assets'],
    queryFn: () => businessApi.listAssets(),
  })

  // 上传素材
  const uploadMutation = useMutation({
    mutationFn: (file: File) => businessApi.uploadAsset(file),
    onSuccess: (data) => {
      message.success('素材上传成功')
      setSelectedMaterials(prev => [...prev, data.id])
    },
    onError: () => message.error('上传失败'),
  })

  // 提交生成任务
  const submitMutation = useMutation({
    mutationFn: () => businessApi.submitGeneration({
      brand_id: brandId,
      text,
      materials: selectedMaterials,
      template: selectedTemplate || undefined,
      duration: duration > 0 ? duration : undefined,
      quality,
    }),
    onSuccess: (data) => {
      message.success('任务已提交，请等待生成完成')
      // 跳转到任务列表
      window.location.href = '/m/compose/tools?tab=media'
    },
    onError: () => message.error('提交失败'),
  })

  // 选中的模板
  const currentTemplate = templates.find(t => t.id === selectedTemplate)

  // 素材类型图标
  const getMaterialIcon = (type: string) => {
    switch (type) {
      case 'image': return <FileImageOutlined />
      case 'video': return <VideoCameraOutlined />
      case 'audio': return <AudioOutlined />
      default: return <FileImageOutlined />
    }
  }

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1>生成内容</h1>
        <p>上传素材、输入描述，系统自动选择端点和模型</p>
      </div>

      <div style={{ display: 'flex', gap: 16 }}>
        {/* 左侧：输入区域 */}
        <Card className="wr-glass-card" style={{ flex: 1 }}>
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            {/* 品牌选择 */}
            <div>
              <Text strong>品牌</Text>
              <Select
                style={{ width: '100%', marginTop: 8 }}
                placeholder="选择品牌"
                value={brandId || undefined}
                onChange={setBrandId}
              >
                {/* 品牌列表需要从API获取 */}
              </Select>
            </div>

            {/* 模板选择 */}
            <div>
              <Text strong>模板（可选）</Text>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 8 }}>
                {templates.map(template => (
                  <Tag
                    key={template.id}
                    color={selectedTemplate === template.id ? 'blue' : 'default'}
                    style={{ cursor: 'pointer', padding: '8px 12px' }}
                    onClick={() => {
                      setSelectedTemplate(selectedTemplate === template.id ? '' : template.id)
                      if (template.default_params?.duration) {
                        setDuration(template.default_params.duration as number)
                      }
                    }}
                  >
                    {template.icon} {template.name}
                  </Tag>
                ))}
              </div>
              {currentTemplate && (
                <Text type="secondary" style={{ display: 'block', marginTop: 4, fontSize: 12 }}>
                  {currentTemplate.description}
                </Text>
              )}
            </div>

            {/* 素材上传 */}
            <div>
              <Text strong>素材</Text>
              <div style={{ marginTop: 8 }}>
                <Dragger
                  multiple
                  showUploadList={false}
                  beforeUpload={(file) => {
                    uploadMutation.mutate(file)
                    return false
                  }}
                >
                  <p className="ant-upload-drag-icon">
                    <UploadOutlined />
                  </p>
                  <p className="ant-upload-text">点击或拖拽上传素材</p>
                  <p className="ant-upload-hint">支持图片、视频、音频</p>
                </Dragger>
              </div>

              {/* 已选素材 */}
              {selectedMaterials.length > 0 && (
                <div style={{ marginTop: 8 }}>
                  <Text type="secondary" style={{ fontSize: 12 }}>已选素材：</Text>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 4 }}>
                    {selectedMaterials.map(id => {
                      const asset = assets.find(a => a.id === id)
                      return (
                        <Tag
                          key={id}
                          closable
                          onClose={() => setSelectedMaterials(prev => prev.filter(m => m !== id))}
                        >
                          {getMaterialIcon(asset?.type || 'image')} {asset?.name || id}
                        </Tag>
                      )
                    })}
                  </div>
                </div>
              )}
            </div>

            {/* 文本输入 */}
            <div>
              <Text strong>描述</Text>
              <TextArea
                style={{ marginTop: 8 }}
                placeholder="描述你想要生成的内容，如：一个现代化的品牌宣传视频"
                autoSize={{ minRows: 3, maxRows: 6 }}
                value={text}
                onChange={(e) => setText(e.target.value)}
              />
            </div>

            {/* 提交按钮 */}
            <Button
              type="primary"
              size="large"
              icon={<SendOutlined />}
              block
              loading={submitMutation.isPending}
              disabled={!brandId || !text}
              onClick={() => submitMutation.mutate()}
            >
              生成
            </Button>
          </Space>
        </Card>

        {/* 右侧：预览/说明 */}
        <Card className="wr-glass-card" style={{ width: 300 }}>
          <Space direction="vertical" size="middle">
            <Text strong>使用说明</Text>
            <div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                1. 选择品牌
              </Text>
              <br />
              <Text type="secondary" style={{ fontSize: 12 }}>
                2. 选择模板（可选，自动填充参数）
              </Text>
              <br />
              <Text type="secondary" style={{ fontSize: 12 }}>
                3. 上传素材（图片/视频/音频）
              </Text>
              <br />
              <Text type="secondary" style={{ fontSize: 12 }}>
                4. 输入描述
              </Text>
              <br />
              <Text type="secondary" style={{ fontSize: 12 }}>
                5. 点击生成
              </Text>
            </div>
            <Text type="secondary" style={{ fontSize: 12 }}>
              系统会根据素材自动选择端点和模型，无需手动配置。
            </Text>
          </Space>
        </Card>
      </div>
    </div>
  )
}
