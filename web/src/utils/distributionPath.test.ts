import { describe, expect, it } from 'vitest'
import { distributionPathFromLipSync, distributionPathFromWork } from './distributionPath'

describe('distributionPathFromWork', () => {
  it('includes cover and media for video works', () => {
    const path = distributionPathFromWork({
      kind: 'video',
      title: '口播成片',
      media_urls: ['https://cdn/a.mp4'],
      cover_url: 'https://cdn/c.jpg',
      brand_id: 'b1',
    })
    expect(path).toContain('contentType=video')
    expect(path).toContain(encodeURIComponent('https://cdn/a.mp4'))
    expect(path).toContain('coverUrl=')
    expect(path).toContain('brandId=b1')
    expect(path).toContain('title=')
  })
})

describe('distributionPathFromLipSync', () => {
  it('prefills media title content and cover', () => {
    const path = distributionPathFromLipSync({
      brandId: 'b1',
      mediaUrl: 'https://cdn/v.mp4',
      coverUrl: 'https://cdn/c.jpg',
      title: '标题',
      content: '正文',
    })
    expect(path.startsWith('/m/distribution?')).toBe(true)
    expect(path).toContain('contentType=video')
    expect(path).toContain('coverUrl=')
    expect(path).toContain('content=')
  })
})
