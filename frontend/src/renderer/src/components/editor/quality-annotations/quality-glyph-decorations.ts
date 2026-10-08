/**
 * quality-glyph-decorations.ts — FE-CV-TASK-087-10
 *
 * Gutter decorations for QualityGlyph. The hover is plain text: Markdown metacharacters in tool
 * output are escaped so a message can never format or link (U9).
 *
 * @module components/editor/quality-annotations/quality-glyph-decorations
 */

import type { QualityGlyph } from './quality-marker-model'
import { qa } from './quality-annotations-copy'

export type QualityGlyphDecoration = {
  range: { startLineNumber: number; startColumn: number; endLineNumber: number; endColumn: number }
  options: {
    glyphMarginClassName: string
    glyphMarginHoverMessage: { value: string }
  }
}

export function escapeMarkdownText(text: string): string {
  return text.replace(/[\\`*_{}[\]()#+\-.!|<>~&]/g, '\\$&').replace(/\r?\n/g, '  \n')
}

export function glyphHoverText(glyph: QualityGlyph): string {
  const lines = [qa('glyphCount', { count: glyph.count }), ...glyph.summaries]
  if (glyph.count > glyph.summaries.length) {
    lines.push(qa('glyphMore', { count: glyph.count - glyph.summaries.length }))
  }
  return lines.map(escapeMarkdownText).join('\n\n')
}

export function buildQualityGlyphDecorations(
  glyphs: readonly QualityGlyph[]
): QualityGlyphDecoration[] {
  return glyphs.map((glyph) => ({
    range: { startLineNumber: glyph.line, startColumn: 1, endLineNumber: glyph.line, endColumn: 1 },
    options: {
      glyphMarginClassName: `orca-quality-glyph-${glyph.severity}`,
      glyphMarginHoverMessage: { value: glyphHoverText(glyph) }
    }
  }))
}
