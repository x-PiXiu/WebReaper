import { describe, expect, it } from 'vitest'
import { insertPauseMarker, previewPauseMarkers, splitPauseMarkers } from './pauseMarkers'

describe('insertPauseMarker', () => {
  it('appends at end when caret omitted', () => {
    const { text, caret } = insertPauseMarker('你好', 1)
    expect(text).toBe('你好<#1#>')
    expect(caret).toBe(7)
  })

  it('inserts at caret position', () => {
    const { text, caret } = insertPauseMarker('你好世界', 0.5, 2)
    expect(text).toBe('你好<#0.5#>世界')
    expect(caret).toBe(9)
  })
})

describe('previewPauseMarkers', () => {
  it('renders readable pause tags', () => {
    expect(previewPauseMarkers('开场<#0.5#>正文<#2#>结尾')).toBe(
      '开场 ⏸0.5s 正文 ⏸2s 结尾',
    )
  })
})

describe('splitPauseMarkers', () => {
  it('splits text and pause segments', () => {
    const parts = splitPauseMarkers('你好<#1#>世界')
    expect(parts).toEqual([
      { type: 'text', value: '你好' },
      { type: 'pause', value: '<#1#>', sec: '1' },
      { type: 'text', value: '世界' },
    ])
  })
})
