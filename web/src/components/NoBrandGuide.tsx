import { Alert, Button } from 'antd'
import { useNavigate } from 'react-router-dom'
import { useBrandContext } from '../hooks/useBrands'

/**
 * 「还没有人设」前置引导：在创作/发布入口就提示并直达创建，
 * 替代原先"走完全流程才被 toast 拦下"的末端拦截。创建后自动消失。
 */
export function NoBrandGuide({ style }: { style?: React.CSSProperties }) {
  const navigate = useNavigate()
  const { brands, isLoading } = useBrandContext()
  if (isLoading || brands.length > 0) return null
  return (
    <Alert
      type="warning"
      showIcon
      style={style}
      message="第一步：创建人设"
      description="人设是 AI 生成与发布的依据——还没有人设时，生成和发布会在最后一步被拦下。"
      action={
        <Button size="small" type="primary" onClick={() => navigate('/m/brands')}>
          去创建人设
        </Button>
      }
    />
  )
}
