import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { readDeclarationBlock } from '../../../test-support/css-color-resolution'

const css = readFileSync(join(process.cwd(), 'src/renderer/src/assets/main.css'), 'utf8')

const LEVELS = ['error', 'warning', 'info', 'pass']
const EXPECTED = [
  ...LEVELS.flatMap((n) => [
    `--quality-${n}`,
    `--quality-${n}-background`,
    `--quality-${n}-border`
  ]),
  '--quality-unknown',
  ...[1, 2, 3, 4, 5].map((i) => `--quality-heat-${i}`)
]

function themeInlineTokens(): Set<string> {
  const start = css.indexOf('@theme inline {')
  const end = css.indexOf('\n}', start)
  const names = new Set<string>()
  for (const m of css
    .slice(start, end)
    .matchAll(/(--color-quality-[\w-]+):\s*var\((--quality-[\w-]+)\)/g)) {
    names.add(`${m[1]}=${m[2]}`)
  }
  return names
}

function sourceFiles(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const p = resolve(dir, name)
    if (statSync(p).isDirectory()) {
      sourceFiles(p, out)
    } else if (/\.(ts|tsx)$/.test(name) && !/\.test\.(ts|tsx)$/.test(name)) {
      out.push(p)
    }
  }
  return out
}

describe('quality token parity', () => {
  const root = readDeclarationBlock(css, ':root')
  const dark = readDeclarationBlock(css, '.dark')
  const inline = themeInlineTokens()

  it('defines every token in :root, .dark and @theme inline', () => {
    for (const token of EXPECTED) {
      expect(root.has(token), `:root ${token}`).toBe(true)
      expect(dark.has(token), `.dark ${token}`).toBe(true)
      expect(inline.has(`${token.replace('--', '--color-')}=${token}`), `@theme ${token}`).toBe(
        true
      )
    }
  })

  it('has no extra --quality-* tokens', () => {
    for (const block of [root, dark]) {
      const extra = [...block.keys()].filter(
        (k) => k.startsWith('--quality-') && !EXPECTED.includes(k)
      )
      expect(extra).toEqual([])
    }
    expect(inline.size).toBe(EXPECTED.length)
  })

  it('quality-charts sources use no hex colours or raw Tailwind palette classes', () => {
    const dir = resolve(process.cwd(), 'src/renderer/src/components/quality-charts')
    for (const file of sourceFiles(dir)) {
      const text = readFileSync(file, 'utf8')
      expect(text, file).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
      expect(text, file).not.toMatch(
        /\b(?:text|bg|border|fill|stroke)-(?:red|amber|yellow|orange|green|blue|sky|slate|gray|zinc|neutral)-\d{2,3}\b/
      )
    }
  })
})
