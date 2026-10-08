import { describe, expect, it } from 'vitest'
import { buildQualityGlyphDecorations, escapeMarkdownText } from './quality-glyph-decorations'
import {
  qualityAnnotationNoticeText,
  resolveQualityAnnotationNotice
} from './quality-annotation-notice'

describe('quality glyph decorations', () => {
  it('escapes Markdown so tool text stays plain', () => {
    expect(escapeMarkdownText('[x](http://e) <b>*hi*</b>')).not.toMatch(/(?<!\\)[[\]*<]/)
  })

  it('builds a class per severity and a plain hover', () => {
    const [decoration] = buildQualityGlyphDecorations([
      { line: 4, severity: 'warning', fingerprint: 'f', count: 5, summaries: ['t/r: [a](b)'] }
    ])
    expect(decoration.options.glyphMarginClassName).toBe('orca-quality-glyph-warning')
    expect(decoration.range.startLineNumber).toBe(4)
    expect(decoration.options.glyphMarginHoverMessage.value).toContain('\\[a\\]\\(b\\)')
    expect(decoration.options.glyphMarginHoverMessage.value).toContain('4 more')
  })
})

describe('quality annotation notice', () => {
  it('prefers the content-changed notice over soft warnings', () => {
    expect(resolveQualityAnnotationNotice({ contentChanged: true, softWarning: 'dirty' })).toBe(
      'content-changed'
    )
    expect(resolveQualityAnnotationNotice({ contentChanged: false, softWarning: 'dirty' })).toBe(
      'dirty'
    )
    expect(resolveQualityAnnotationNotice({ contentChanged: false })).toBeNull()
  })

  it('has distinct text per kind', () => {
    const texts = (['content-changed', 'dirty', 'changed-during-run'] as const).map(
      qualityAnnotationNoticeText
    )
    expect(new Set(texts).size).toBe(3)
  })
})
