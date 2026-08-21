import { Link, useNavigate } from 'react-router-dom'
import { Button, Col, Row, Tag, Typography } from 'antd'
import { EditOutlined, VideoCameraOutlined } from '@ant-design/icons'
import { useComposeDraft } from '../../../store/composeDraft'
import { PRODUCT } from '../../../config/product'

const { Text, Title } = Typography

/**
 * 爆款获客总览：明确拆成「发视频」与「发图文」两条能力线。
 */
export default function ComposeHub() {
  const navigate = useNavigate()
  const draft = useComposeDraft()
  const hasCopy = !!(draft.script || draft.rewritten || draft.transcript)

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">{PRODUCT.nameEn}</p>
          <h1>爆款获客</h1>
          <p className="ip-lead">选一条线进入三步引导：左编辑、右预览——发视频或发图文互不混淆</p>
        </div>
      </div>

      <Row gutter={[20, 20]} className="ip-stagger">
        <Col xs={24} md={12}>
          <button
            type="button"
            className={`ip-track-card${draft.track === 'video' ? ' is-active' : ''}`}
            onClick={() => {
              draft.setTrack('video')
              navigate('/m/compose/video')
            }}
          >
            <div className="ip-track-icon"><VideoCameraOutlined /></div>
            <Tag color="cyan">轨道 A</Tag>
            <Title level={3} style={{ margin: '10px 0 8px' }}>发视频</Title>
            <Text type="secondary" style={{ display: 'block', marginBottom: 14 }}>
              三步：写脚本 → 配素材 → 出成片，右侧实时预览竖屏效果
            </Text>
            <ul className="ip-track-list">
              <li>适合：口播获客、门店探店、产品演示</li>
              <li>产出：竖屏短视频成片</li>
              <li>引导：配音、数字人、封面一站完成</li>
            </ul>
            <Button type="primary" className="ip-btn-primary" block style={{ marginTop: 16 }}>
              进入发视频
            </Button>
          </button>
        </Col>
        <Col xs={24} md={12}>
          <button
            type="button"
            className={`ip-track-card${draft.track === 'graphic' ? ' is-active' : ''}`}
            onClick={() => {
              draft.setTrack('graphic')
              navigate('/m/compose/graphic')
            }}
          >
            <div className="ip-track-icon"><EditOutlined /></div>
            <Tag color="gold">轨道 B</Tag>
            <Title level={3} style={{ margin: '10px 0 8px' }}>发图文</Title>
            <Text type="secondary" style={{ display: 'block', marginBottom: 14 }}>
              三步：写图文 → 配图 → 发布，右侧实时预览笔记卡片
            </Text>
            <ul className="ip-track-list">
              <li>适合：种草笔记、长文种草、图文帖</li>
              <li>产出：标题 + 正文 + 配图</li>
              <li>引导：文案、配图、封面一站完成</li>
            </ul>
            <Button type="primary" className="ip-btn-primary" block style={{ marginTop: 16 }}>
              进入发图文
            </Button>
          </button>
        </Col>
      </Row>

      {hasCopy && (
        <div className="ip-panel ip-rise" style={{ marginTop: 20 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            共享草稿 · 当前轨道 {draft.track === 'graphic' ? '发图文' : '发视频'}
          </Text>
          <div style={{ marginTop: 8, display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center' }}>
            {draft.selectedTitle && <Tag color="cyan">{draft.selectedTitle}</Tag>}
            <Text type="secondary" style={{ fontSize: 13 }}>
              {(draft.rewritten || draft.script || draft.transcript || '').slice(0, 56)}…
            </Text>
            <Link to={draft.track === 'graphic' ? '/m/compose/graphic' : '/m/compose/video'}>继续编辑 →</Link>
          </div>
        </div>
      )}
    </div>
  )
}
