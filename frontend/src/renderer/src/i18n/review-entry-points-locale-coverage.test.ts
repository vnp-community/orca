import { readdirSync, readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const PREFIX = 'auto.components.reviewMap.'
const KEY_RE = /'(auto\.components\.reviewMap\.(?:EntryButton|ReviewSummaryPanel|QuickActions)\.[\w.]*\w)'/g

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {
      return undefined
    }
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) {
      return e.name === 'node_modules' ? [] : sourceFiles(p)
    }
    return /\.(ts|tsx)$/.test(e.name) && !e.name.includes('.test.') ? [p] : []
  })
}

const used = new Set<string>()
for (const root of ['../components', '../lib']) {
  for (const file of sourceFiles(resolve(__dirname, root))) {
    for (const m of readFileSync(file, 'utf8').matchAll(KEY_RE)) {
      used.add(m[1])
    }
  }
}

describe('Review entry points locale catalog', () => {
  it('finds the keys used by the entry points', () => {
    expect(used.size).toBeGreaterThan(25)
    expect([...used].every((k) => k.startsWith(PREFIX))).toBe(true)
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: every entry-point key exists and is non-empty`, () => {
      const bad = [...used].filter((k) => {
        const v = lookup(catalog, k)
        return typeof v !== 'string' || v.trim() === ''
      })
      expect(bad).toEqual([])
    })
  }
})
