// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest'
import { readReportThemeTokens, REPORT_TOKEN_VARIABLES } from './review-report-theme-tokens'

afterEach(() => {
  document.head.innerHTML = ''
})

describe('readReportThemeTokens', () => {
  it('returns every token for light and dark and removes its probe element', () => {
    const before = document.body.childElementCount
    const tokens = readReportThemeTokens()
    for (const name of Object.keys(REPORT_TOKEN_VARIABLES)) {
      expect(typeof tokens.light[name as keyof typeof tokens.light]).toBe('string')
      expect(tokens.dark[name as keyof typeof tokens.dark].length).toBeGreaterThan(0)
    }
    expect(document.body.childElementCount).toBe(before)
  })

  it('falls back to CSS system colors when the variables are unresolved', () => {
    const tokens = readReportThemeTokens()
    expect(tokens.light.background).toBe('Canvas')
    expect(tokens.light.foreground).toBe('CanvasText')
  })

  it('never copies an unresolved var() into the export', () => {
    const style = document.createElement('style')
    style.textContent = ':root{--border: var(--elsewhere);}'
    document.head.appendChild(style)
    expect(readReportThemeTokens().light.border).not.toContain('var(')
  })
})
