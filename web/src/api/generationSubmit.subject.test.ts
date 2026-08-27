import { describe, expect, it } from 'vitest'
import { buildSubjectRegisterPayload } from './generationSubmit'

describe('buildSubjectRegisterPayload', () => {
  it('uses material ids and mirrors urls in params', () => {
    const p = buildSubjectRegisterPayload({
      brand_id: 'b1',
      name: '陈师傅',
      imageMaterialIds: ['tenant/2026/08/abc.jpg'],
      imageUrls: ['http://host/media/tenant/2026/08/abc.jpg'],
      voice_id: 'v1',
    })
    expect(p.sub_type).toBe('subject')
    expect(p.text).toBe('陈师傅')
    expect(p.materials).toEqual(['tenant/2026/08/abc.jpg'])
    expect(p.params).toMatchObject({
      name: '陈师傅',
      voice_id: 'v1',
      images: ['http://host/media/tenant/2026/08/abc.jpg'],
    })
  })

  it('supports video-only payload', () => {
    const p = buildSubjectRegisterPayload({
      brand_id: 'b1',
      name: '场景A',
      videoUrl: 'http://host/media/v.mp4',
    })
    expect(p.params?.videos).toEqual(['http://host/media/v.mp4'])
    expect(p.materials).toEqual(['http://host/media/v.mp4'])
  })
})
