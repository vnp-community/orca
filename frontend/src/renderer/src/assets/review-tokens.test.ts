import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

// FE-CV-TASK-050-19: review colors are semantic tokens in :root, .dark and @theme inline, never hex.
const css = readFileSync(resolve(import.meta.dirname, 'main.css'), 'utf8')

const TOKENS = [
  'review-changed',
  'review-affected',
  'review-untested',
  'review-violation',
  ...[1, 2, 3, 4, 5, 6].map((n) => `review-area-${n}`)
]

/** Body of the first top-level block starting with `selector {`. */
function block(selector: string): string {
  const start = css.indexOf(`\n${selector} {`)
  expect(start, `${selector} block`).toBeGreaterThanOrEqual(0)
  let depth = 0
  for (let i = css.indexOf('{', start); i < css.length; i++) {
    if (css[i] === '{') {depth++}
    if (css[i] === '}' && --depth === 0) {return css.slice(start, i)}
  }
  throw new Error(`unterminated block ${selector}`)
}

function declaration(body: string, name: string): string | undefined {
  return new RegExp(`${name}\\s*:\\s*([^;]+);`).exec(body)?.[1].trim()
}

describe('review color tokens in main.css', () => {
  const light = block(':root')
  const dark = block('.dark')
  const theme = block('@theme inline')

  it.each(TOKENS)('%s is defined in :root, .dark and bound in @theme inline', (token) => {
    expect(declaration(light, `--${token}`), `:root --${token}`).toBeTruthy()
    expect(declaration(dark, `--${token}`), `.dark --${token}`).toBeTruthy()
    expect(declaration(theme, `--color-${token}`), `@theme --color-${token}`).toBe(`var(--${token})`)
  })

  it('values reference palette/semantic variables, not hex or rgb literals', () => {
    for (const token of TOKENS) {
      for (const body of [light, dark]) {
        const value = declaration(body, `--${token}`)!
        expect(value, token).toMatch(/^var\(--[a-z0-9-]+\)$/)
      }
    }
  })

  it('dark values differ from light values (the pair is tuned per theme)', () => {
    for (const token of TOKENS) {
      expect(declaration(dark, `--${token}`), token).not.toBe(declaration(light, `--${token}`))
    }
  })
})
