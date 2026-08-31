import { Link, useLocation } from 'react-router-dom'
import { UserOutlined, VideoCameraOutlined, FolderOpenOutlined, ExportOutlined } from '@ant-design/icons'

/**
 * 口播全旅程导航（23 号计划 §1）：分身 → 创作 → 作品 → 发布。
 * 挂在工作台/向导/分身/作品等页顶部，标当前阶段，降低侧栏名实分离的迷路感。
 */
const STEPS = [
  {
    key: 'avatar',
    label: '分身',
    hint: '准备形象',
    path: '/m/compose/avatar',
    match: (p: string) => p.startsWith('/m/compose/avatar'),
    icon: <UserOutlined />,
  },
  {
    key: 'create',
    label: '创作',
    hint: '口播成片',
    path: '/m/compose/lipsync',
    match: (p: string) =>
      p.startsWith('/m/compose/lipsync')
      || p === '/m/compose'
      || p.startsWith('/m/compose/copy')
      || p.startsWith('/m/compose/voice'),
    icon: <VideoCameraOutlined />,
  },
  {
    key: 'works',
    label: '作品',
    hint: '可插画面',
    path: '/m/works',
    match: (p: string) => p.startsWith('/m/works'),
    icon: <FolderOpenOutlined />,
  },
  {
    key: 'publish',
    label: '发布',
    hint: '多平台分发',
    path: '/m/distribution',
    match: (p: string) => p.startsWith('/m/distribution'),
    icon: <ExportOutlined />,
  },
] as const

export default function OralJourneyNav({ className = '' }: { className?: string }) {
  const { pathname } = useLocation()
  const activeIdx = STEPS.findIndex((s) => s.match(pathname))

  return (
    <nav className={`wr-oral-journey ${className}`.trim()} aria-label="口播旅程">
      <ol className="wr-oral-journey-list">
        {STEPS.map((step, i) => {
          const active = i === activeIdx
          const done = activeIdx >= 0 && i < activeIdx
          return (
            <li key={step.key} className={`wr-oral-journey-item${active ? ' is-active' : ''}${done ? ' is-done' : ''}`}>
              {i > 0 && <span className="wr-oral-journey-rail" aria-hidden />}
              <Link to={step.path} className="wr-oral-journey-link" aria-current={active ? 'step' : undefined}>
                <span className="wr-oral-journey-icon">{step.icon}</span>
                <span className="wr-oral-journey-copy">
                  <strong>{step.label}</strong>
                  <em>{step.hint}</em>
                </span>
              </Link>
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
