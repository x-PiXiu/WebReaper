import { Link, useLocation } from 'react-router-dom'
import { Alert, Space, Tag, Typography } from 'antd'
import { modulesForTrack, useComposeDraft } from '../../../store/composeDraft'

const { Text } = Typography

const LEGACY_VIDEO_PATHS = new Set([
  '/m/compose/benchmark', '/m/compose/copy', '/m/compose/titles',
  '/m/compose/voice', '/m/compose/avatar', '/m/compose/edit', '/m/compose/cover',
])

/** 模块页顶栏：仅展示当前轨道相关模块，避免视频/图文混跳 */
export function ComposeModuleHeader({
  title,
  lead,
  badge,
}: {
  title: string
  lead: string
  badge?: string
}) {
  const { pathname, search } = useLocation()
  const draft = useComposeDraft()
  const track = draft.track
  const mods = modulesForTrack(track)
  const trackLabel = track === 'graphic' ? '发图文' : '发视频'
  const trackHome = track === 'graphic' ? '/m/compose/graphic' : '/m/compose/lipsync'
  const showWizardHint = track !== 'graphic' && LEGACY_VIDEO_PATHS.has(pathname)

  const activePath = pathname + search

  return (
    <>
      <div className="ip-page-hero" style={{ marginBottom: 12 }}>
        <div>
          <p className="ip-kicker">
            <Link to="/m/compose" style={{ color: 'inherit' }}>工作台</Link>
            {' · '}
            <Link to={trackHome} style={{ color: 'inherit' }}>{trackLabel}</Link>
          </p>
          <h1 style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            {title}
            {badge ? <Tag style={{ fontSize: 12 }}>{badge}</Tag> : (
              <Tag style={{ fontSize: 12 }}>{trackLabel}</Tag>
            )}
          </h1>
          <p className="ip-lead">{lead}</p>
        </div>
      </div>
      {showWizardHint && (
        <Alert
          type="info"
          showIcon
          className="wz-draft-banner"
          message="推荐：用口播向导或快速生成"
          description={
            <>
              新流程步骤更少、自动对口型。试试
              {' '}<Link to="/m/compose/lipsync">口播向导</Link>
              {' '}或{' '}<Link to="/m/compose/quick">快速生成</Link>。
            </>
          }
        />
      )}
      <div className="ip-toolbar" style={{ marginBottom: 16, flexWrap: 'wrap', gap: 8 }}>
        <Text type="secondary" style={{ fontSize: 12, marginRight: 4 }}>本轨道模块</Text>
        <Space size={[6, 6]} wrap>
          {mods.map((m) => {
            const pathOnly = m.path.split('?')[0]
            const active = activePath === m.path
              || pathname === pathOnly
              || (pathOnly !== '/m/distribution' && pathname.startsWith(pathOnly) && pathOnly !== '/m/compose')
            return (
              <Link key={m.key} to={m.path}>
                <Tag color={active ? 'cyan' : undefined} style={{ cursor: 'pointer', margin: 0 }}>
                  {m.label}
                </Tag>
              </Link>
            )
          })}
        </Space>
      </div>
    </>
  )
}
