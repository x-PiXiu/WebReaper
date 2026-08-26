import { useCallback, useEffect, useState } from 'react'

const STORAGE_KEY = 'wr_merchant_onboard_v1'

/** 商户端新手引导：首次登录完成后展示，localStorage 记完成态 */
export function useMerchantOnboarding(active = true) {
  const [open, setOpen] = useState(false)

  const isDone = () => localStorage.getItem(STORAGE_KEY) === 'done'

  useEffect(() => {
    if (!active || isDone()) return
    const t = window.setTimeout(() => setOpen(true), 720)
    return () => window.clearTimeout(t)
  }, [active])

  const finish = useCallback(() => {
    localStorage.setItem(STORAGE_KEY, 'done')
    setOpen(false)
  }, [])

  const skip = useCallback(() => {
    finish()
  }, [finish])

  const replay = useCallback(() => {
    setOpen(true)
  }, [])

  return { open, setOpen, finish, skip, replay, isDone: isDone() }
}
