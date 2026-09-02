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

  it('rejects payload without images (video subject removed)', () => {
    // 31号定案：主体视频入口下线——无形象照的 payload 直接抛错
    expect(() => buildSubjectRegisterPayload({
      brand_id: 'b1',
      name: '场景A',
    })).toThrow('请上传至少 1 张形象照')
  })
})
