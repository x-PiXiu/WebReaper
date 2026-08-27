import { useEffect, useState } from 'react'
import { VideoCameraOutlined } from '@ant-design/icons'
import { resolveMediaUrl } from '../utils/generationTask'

const frameCache = new Map<string, string>()

type Props = {
  url: string
  /** 服务端封面（有则优先，否则抓第一帧） */
  poster?: string
  className?: string
}

/**
 * 视频封面：优先 cover_url，否则从视频第一帧截取（同源可 canvas；跨域降级为 video 元素展示首帧）。
 */
export function VideoFrameCover({ url, poster, className }: Props) {
  const src = resolveMediaUrl(url)
  const cached = frameCache.get(src)
  const [thumb, setThumb] = useState<string | null>(poster || cached || null)
  const [useVideoEl, setUseVideoEl] = useState(false)

  useEffect(() => {
    if (poster) {
      setThumb(poster)
      return
    }
    const hit = frameCache.get(src)
    if (hit) {
      setThumb(hit)
      return
    }

    let cancelled = false
    const video = document.createElement('video')
    video.muted = true
    video.playsInline = true
    video.preload = 'auto'
    const sameOrigin = src.startsWith(window.location.origin) || src.startsWith('/media/')
    if (!sameOrigin) video.crossOrigin = 'anonymous'

    const capture = () => {
      if (cancelled || !video.videoWidth) return
      try {
        const canvas = document.createElement('canvas')
        canvas.width = video.videoWidth
        canvas.height = video.videoHeight
        const ctx = canvas.getContext('2d')
        if (!ctx) return
        ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
        const dataUrl = canvas.toDataURL('image/jpeg', 0.85)
        frameCache.set(src, dataUrl)
        setThumb(dataUrl)
      } catch {
        if (!cancelled) setUseVideoEl(true)
      }
    }

    const onSeeked = () => capture()
    const onLoaded = () => {
      try {
        video.currentTime = Math.min(0.08, video.duration > 0 ? video.duration * 0.02 : 0.08)
      } catch {
        capture()
      }
    }
    const onError = () => {
      if (!cancelled) setUseVideoEl(true)
    }

    video.addEventListener('seeked', onSeeked)
    video.addEventListener('loadeddata', onLoaded, { once: true })
    video.addEventListener('error', onError, { once: true })
    video.src = src

    return () => {
      cancelled = true
      video.removeEventListener('seeked', onSeeked)
      video.removeEventListener('loadeddata', onLoaded)
      video.removeEventListener('error', onError)
      video.removeAttribute('src')
      video.load()
    }
  }, [src, poster])

  if (thumb) {
    return (
      <div
        className={className}
        style={{
          position: 'absolute',
          inset: 0,
          backgroundImage: `linear-gradient(180deg, rgba(0,0,0,0.08), rgba(0,0,0,0.5)), url(${thumb})`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
        }}
        aria-hidden
      />
    )
  }

  if (useVideoEl) {
    return (
      <video
        className={className}
        src={src}
        muted
        playsInline
        preload="metadata"
        style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' }}
        aria-hidden
      />
    )
  }

  return (
    <div
      className={className}
      style={{
        position: 'absolute',
        inset: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(145deg, #12121a, #1f2937)',
      }}
      aria-hidden
    >
      <VideoCameraOutlined style={{ fontSize: 32, color: 'rgba(255,255,255,0.5)' }} />
    </div>
  )
}
