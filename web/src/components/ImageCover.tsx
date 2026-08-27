import { useState } from 'react'
import { PictureOutlined } from '@ant-design/icons'
import { resolveMediaUrl } from '../utils/generationTask'

type Props = {
  url: string
  className?: string
}

/** 图片封面缩略图（解析相对路径，加载失败有占位） */
export function ImageCover({ url, className }: Props) {
  const src = resolveMediaUrl(url)
  const [failed, setFailed] = useState(false)

  if (!src || failed) {
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
        <PictureOutlined style={{ fontSize: 32, color: 'rgba(255,255,255,0.5)' }} />
      </div>
    )
  }

  return (
    <img
      src={src}
      alt=""
      className={className}
      loading="lazy"
      decoding="async"
      onError={() => setFailed(true)}
      style={{
        position: 'absolute',
        inset: 0,
        width: '100%',
        height: '100%',
        objectFit: 'cover',
        display: 'block',
      }}
    />
  )
}
