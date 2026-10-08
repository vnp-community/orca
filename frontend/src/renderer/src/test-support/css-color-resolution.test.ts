import { describe, expect, it } from 'vitest'
import {
  contrastRatio,
  oklchToRgba,
  readDeclarationBlock,
  resolveColorValue
} from './css-color-resolution'

describe('css-color-resolution', () => {
  it('white on black is 21:1', () => {
    const white = resolveColorValue('#fff', new Map())
    const black = resolveColorValue('#000000', new Map())
    expect(contrastRatio(white, black)).toBeCloseTo(21, 5)
  })

  it('converts oklch white and black', () => {
    const white = oklchToRgba(1, 0, 0)
    expect(Math.round(white.r)).toBe(255)
    expect(Math.round(white.g)).toBe(255)
    const black = oklchToRgba(0, 0, 0)
    expect(Math.round(black.r)).toBe(0)
  })

  it('resolves var() chains and color-mix in srgb', () => {
    const table = new Map([
      ['--a', '#ffffff'],
      ['--b', 'var(--a)']
    ])
    const mixed = resolveColorValue('color-mix(in srgb, var(--b) 50%, #000000)', table)
    expect(Math.round(mixed.r)).toBe(128)
  })

  it('throws with the token name on a var() cycle', () => {
    const table = new Map([
      ['--x', 'var(--y)'],
      ['--y', 'var(--x)']
    ])
    expect(() => resolveColorValue('var(--x)', table)).toThrow(/--x/)
  })

  it('reads the first declaration block and strips comments', () => {
    const css = ':root {\n  /* c; */\n  --a: #fff;\n}\n.dark {\n  --a: #000;\n}\n'
    expect(readDeclarationBlock(css, ':root').get('--a')).toBe('#fff')
    expect(readDeclarationBlock(css, '.dark').get('--a')).toBe('#000')
  })
})
