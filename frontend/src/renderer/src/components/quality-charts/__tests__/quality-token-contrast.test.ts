import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  contrastRatio,
  resolveThemeColor,
  type ThemeName
} from '../../../test-support/css-color-resolution'

const css = readFileSync(join(process.cwd(), 'src/renderer/src/assets/main.css'), 'utf8')
const THEMES: ThemeName[] = ['light', 'dark']
const LEVEL_TOKENS = [
  '--quality-error',
  '--quality-warning',
  '--quality-info',
  '--quality-pass',
  '--quality-unknown'
]

function ratio(fg: string, bg: string, theme: ThemeName): number {
  return contrastRatio(resolveThemeColor(css, fg, theme), resolveThemeColor(css, bg, theme))
}

// Why: thin margins make rounding changes in tokens easy to miss; surface them without failing.
function warnIfThin(label: string, value: number, threshold: number): void {
  if (value - threshold < 0.05) {
    console.warn(`[contrast] thin margin ${label}: ${value.toFixed(3)} vs ${threshold}`)
  }
}

describe('quality token contrast', () => {
  for (const theme of THEMES) {
    for (const token of LEVEL_TOKENS) {
      for (const surface of ['--card', '--background']) {
        it(`${theme}: text ${token} on ${surface} >= 4.5`, () => {
          const value = ratio(token, surface, theme)
          warnIfThin(`${theme} ${token}/${surface}`, value, 4.5)
          expect(value).toBeGreaterThanOrEqual(4.5)
        })
      }
      it(`${theme}: graphic ${token} on --muted >= 3`, () => {
        expect(ratio(token, '--muted', theme)).toBeGreaterThanOrEqual(3)
      })
    }

    for (const step of [1, 2, 3]) {
      it(`${theme}: --foreground on heat-${step} >= 4.5`, () => {
        const value = ratio('--foreground', `--quality-heat-${step}`, theme)
        warnIfThin(`${theme} foreground/heat-${step}`, value, 4.5)
        expect(value).toBeGreaterThanOrEqual(4.5)
      })
    }
    for (const step of [4, 5]) {
      it(`${theme}: --background on heat-${step} >= 4.5`, () => {
        const value = ratio('--background', `--quality-heat-${step}`, theme)
        warnIfThin(`${theme} background/heat-${step}`, value, 4.5)
        expect(value).toBeGreaterThanOrEqual(4.5)
      })
    }
  }

  // Why: records the reason for the usage rule "level-coloured text never sits on --muted".
  it('light: error and unknown text on --muted stay below 4.5 (rule: icons/borders only)', () => {
    expect(ratio('--quality-error', '--muted', 'light')).toBeLessThan(4.5)
    expect(ratio('--quality-unknown', '--muted', 'light')).toBeLessThan(4.5)
  })

  it('matches the independently computed reference values', () => {
    expect(ratio('--quality-error', '--card', 'light')).toBeCloseTo(4.87, 1)
    expect(ratio('--quality-warning', '--card', 'dark')).toBeCloseTo(10.44, 0)
  })
})
