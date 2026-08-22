import * as SimpleIcons from 'simple-icons'
import { getPlatformIconSource, getPlatformMeta, normalizePlatform } from '../data/platforms'

type Props = {
  platform?: string
  size?: number
  className?: string
}

type SimpleIconData = { title: string; hex: string; path: string }

function getSimpleIcon(slug: string): SimpleIconData | null {
  const icon = (SimpleIcons as Record<string, SimpleIconData | undefined>)[slug]
  return icon ?? null
}

/**
 * 各社交平台 Logo
 * - 抖音：官网 favicon（douyin.com）
 * - 其余：simple-icons 官方品牌矢量（https://simpleicons.org）
 */
export function PlatformIcon({ platform, size = 18, className = '' }: Props) {
  const key = normalizePlatform(platform)
  const cls = `platform-icon ${className}`.trim()
  const source = getPlatformIconSource(platform)

  if (source?.type === 'official-asset') {
    return (
      <img
        className={`${cls} platform-icon-official`}
        src={source.src}
        width={size}
        height={size}
        alt=""
        aria-hidden
        loading="lazy"
        decoding="async"
      />
    )
  }

  if (source?.type === 'simple-icons') {
    const icon = getSimpleIcon(source.slug)
    if (icon) {
      return (
        <svg
          className={cls}
          width={size}
          height={size}
          viewBox="0 0 24 24"
          role="img"
          aria-label={icon.title}
        >
          <title>{icon.title}</title>
          <path fill={`#${icon.hex}`} d={icon.path} />
        </svg>
      )
    }
  }

  const meta = getPlatformMeta(key)
  return (
    <svg className={cls} width={size} height={size} viewBox="0 0 24 24" aria-hidden>
      <rect width="24" height="24" rx="6" fill={meta.color} />
      <circle cx="12" cy="12" r="4" fill="#fff" opacity="0.9" />
    </svg>
  )
}
