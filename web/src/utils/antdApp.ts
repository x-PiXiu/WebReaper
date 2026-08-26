import type { MessageInstance } from 'antd/es/message/interface'
import type { ModalStaticFunctions } from 'antd/es/modal/confirm'
import type { NotificationInstance } from 'antd/es/notification/interface'

/**
 * Ant Design 5 静态 API 桥接。
 * 在 <App> 内通过 useApp() 注入后，业务代码与 axios 拦截器可安全调用，
 * 避免 "Static function can not consume context" 警告。
 */
type ModalApi = Omit<ModalStaticFunctions, 'warn'>

let messageApi: MessageInstance | null = null
let modalApi: ModalApi | null = null
let notificationApi: NotificationInstance | null = null

export function bindAntdAppApis(apis: {
  message: MessageInstance
  modal: ModalApi
  notification: NotificationInstance
}) {
  messageApi = apis.message
  modalApi = apis.modal
  notificationApi = apis.notification
}

function ensureMessage(): MessageInstance {
  if (!messageApi) {
    // App 尚未挂载时的极短窗口：静默跳过，避免再触发静态 API 警告
    return {
      success: () => ({ then() { return Promise.resolve(null) }, promise: Promise.resolve(null) }),
      error: () => ({ then() { return Promise.resolve(null) }, promise: Promise.resolve(null) }),
      info: () => ({ then() { return Promise.resolve(null) }, promise: Promise.resolve(null) }),
      warning: () => ({ then() { return Promise.resolve(null) }, promise: Promise.resolve(null) }),
      loading: () => ({ then() { return Promise.resolve(null) }, promise: Promise.resolve(null) }),
      open: () => ({ then() { return Promise.resolve(null) }, promise: Promise.resolve(null) }),
      destroy: () => {},
    } as unknown as MessageInstance
  }
  return messageApi
}

function ensureModal(): ModalApi {
  if (!modalApi) {
    throw new Error('Ant Design App 尚未就绪，请稍后重试')
  }
  return modalApi
}

/** 可在组件外 / 拦截器中使用的 message（走 App.useApp） */
export const message = {
  success: (...args: Parameters<MessageInstance['success']>) => ensureMessage().success(...args),
  error: (...args: Parameters<MessageInstance['error']>) => ensureMessage().error(...args),
  info: (...args: Parameters<MessageInstance['info']>) => ensureMessage().info(...args),
  warning: (...args: Parameters<MessageInstance['warning']>) => ensureMessage().warning(...args),
  loading: (...args: Parameters<MessageInstance['loading']>) => ensureMessage().loading(...args),
  open: (...args: Parameters<MessageInstance['open']>) => ensureMessage().open(...args),
  destroy: (...args: Parameters<MessageInstance['destroy']>) => ensureMessage().destroy(...args),
}

/** 可在组件外使用的 modal（confirm / info 等） */
export const modal = {
  confirm: (...args: Parameters<ModalApi['confirm']>) => ensureModal().confirm(...args),
  info: (...args: Parameters<ModalApi['info']>) => ensureModal().info(...args),
  success: (...args: Parameters<ModalApi['success']>) => ensureModal().success(...args),
  error: (...args: Parameters<ModalApi['error']>) => ensureModal().error(...args),
  warning: (...args: Parameters<ModalApi['warning']>) => ensureModal().warning(...args),
}

export const notification = {
  success: (...args: Parameters<NotificationInstance['success']>) => {
    notificationApi?.success(...args)
  },
  error: (...args: Parameters<NotificationInstance['error']>) => {
    notificationApi?.error(...args)
  },
  info: (...args: Parameters<NotificationInstance['info']>) => {
    notificationApi?.info(...args)
  },
  warning: (...args: Parameters<NotificationInstance['warning']>) => {
    notificationApi?.warning(...args)
  },
  open: (...args: Parameters<NotificationInstance['open']>) => {
    notificationApi?.open(...args)
  },
  destroy: (...args: Parameters<NotificationInstance['destroy']>) => {
    notificationApi?.destroy(...args)
  },
}
