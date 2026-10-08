import { describe, expect, it } from 'vitest'
import { buildSymbolLineIndex, findInnermostSymbolAtLine } from './symbol-line-index'

describe('symbol-line-index', () => {
  const index = buildSymbolLineIndex([
    { key: 'class', filePath: 'src/a.ts', startLine: 1, endLine: 100 },
    { key: 'method', filePath: 'src/a.ts', startLine: 10, endLine: 20 },
    { key: 'other', filePath: 'src\\b.ts', startLine: 5, endLine: 8 },
    { key: 'nolines', filePath: 'src/a.ts' },
    { key: 'method', filePath: 'src/a.ts', startLine: 10, endLine: 20 }
  ])
  it('chooses the narrowest enclosing range', () => {
    expect(findInnermostSymbolAtLine(index, 'src/a.ts', 15)).toBe('method')
    expect(findInnermostSymbolAtLine(index, 'src/a.ts', 50)).toBe('class')
  })
  it('range bounds are inclusive and 1-based', () => {
    expect(findInnermostSymbolAtLine(index, 'src/a.ts', 10)).toBe('method')
    expect(findInnermostSymbolAtLine(index, 'src/a.ts', 20)).toBe('method')
    expect(findInnermostSymbolAtLine(index, 'src/a.ts', 21)).toBe('class')
    expect(findInnermostSymbolAtLine(index, 'src/a.ts', 101)).toBeNull()
  })
  it('handles several files and windows separators', () => {
    expect(findInnermostSymbolAtLine(index, 'src/b.ts', 6)).toBe('other')
    expect(findInnermostSymbolAtLine(index, 'src\\b.ts', 6)).toBe('other')
    expect(findInnermostSymbolAtLine(index, 'src/zzz.ts', 6)).toBeNull()
  })
  it('ignores symbols without a start line', () => {
    expect(findInnermostSymbolAtLine(index, 'src/a.ts', 1)).toBe('class')
  })
})
