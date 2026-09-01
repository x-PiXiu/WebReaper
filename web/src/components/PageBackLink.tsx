import { Link } from 'react-router-dom'
import { ArrowLeftOutlined } from '@ant-design/icons'

type Props = {
  to?: string
  label?: string
  onClick?: () => void
  className?: string
}

/** 二级页返回入口：圆角 pill + 图标底，替代裸链接 */
export function PageBackLink({ to, label = '返回', onClick, className = '' }: Props) {
  const body = (
    <>
      <span className="wr-page-back-icon" aria-hidden>
        <ArrowLeftOutlined />
      </span>
      <span className="wr-page-back-label">{label}</span>
    </>
  )

  if (onClick) {
    return (
      <button type="button" className={`wr-page-back ${className}`.trim()} onClick={onClick}>
        {body}
      </button>
    )
  }

  return (
    <Link to={to || '/m/works'} className={`wr-page-back ${className}`.trim()}>
      {body}
    </Link>
  )
}
