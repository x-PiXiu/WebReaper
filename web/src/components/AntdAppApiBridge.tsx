import { App } from 'antd'
import { bindAntdAppApis } from '../utils/antdApp'

/** 挂在 <App> 内部，把 useApp() 实例绑定到模块级桥接，供拦截器与非 hooks 代码使用 */
export default function AntdAppApiBridge() {
  const apis = App.useApp()
  // 同步绑定：避免首帧 useEffect 之前拦截器/早期逻辑拿不到 message/modal
  bindAntdAppApis(apis)
  return null
}
