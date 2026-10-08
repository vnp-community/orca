import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Report text is built from hand-named keys read by name (not auto-extracted),
// so this prevents a locale silently falling back to English.
const BASE = 'auto.components.reviewMap.report'
const GATE = ['pass', 'warn', 'fail', 'unknown'].map((k) => `gate.${k}`)
const SHARED = [
  ...GATE, 'gate.heading', 'summary.heading', 'summary.counts', 'findings.heading', 'findings.counts',
  'findings.colSeverity', 'findings.colRule', 'findings.colLocation', 'findings.colMessage',
  'contracts.heading', 'reading.heading', 'diagrams.heading', 'warnings.heading',
  'warning.no_gate', 'warning.index_stale', 'warning.overlay_partial', 'footer'
]
const KEYS = [
  ...SHARED.flatMap((k) => [`${BASE}.md.${k}`, `${BASE}.html.${k}`]),
  ...['title', 'subject', 'gate.moreReasons', 'gate.waivers', 'risk.heading', 'summary.components',
    'findings.truncated', 'contracts.breaking', 'contracts.channel', 'contracts.table', 'diagrams.alt', 'truncated',
    ...['LOW', 'MEDIUM', 'HIGH', 'CRITICAL', 'UNKNOWN'].map((l) => `risk.level.${l}`)
  ].map((k) => `${BASE}.md.${k}`),
  ...['title', 'findings.caption', 'breaking'].map((k) => `${BASE}.html.${k}`),
  ...['label', 'copyMarkdown', 'copyForDescription', 'saveHtml', 'copied', 'copiedShortened', 'copyError',
    'saved', 'saveError', 'fetchError'].map((k) => `${BASE}.menu.${k}`),
  `${BASE}.insert.label`
]

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) return undefined
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

function placeholders(value: string): string[] {
  return [...value.matchAll(/\{\{(\w+)\}\}/g)].map((m) => m[1]).sort()
}

describe('Review report catalog entries', () => {
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: all keys exist and are non-empty strings`, () => {
      for (const key of KEYS) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key} empty`).not.toBe('')
      }
    })

    it(`${locale}: placeholders match English`, () => {
      for (const key of KEYS) {
        expect(placeholders(lookup(catalog, key) as string), `${locale}: ${key}`).toEqual(
          placeholders(lookup(en, key) as string)
        )
      }
    })
  }

  it('catalog wording never overclaims', () => {
    const text = KEYS.map((k) => lookup(en, k) as string).join('\n').toLowerCase()
    for (const banned of ['safe to merge', 'approved by', 'ready to merge', 'all clear']) {
      expect(text).not.toContain(banned)
    }
  })

  it('every key used by the report sources exists in the catalog', () => {
    const dir = join(import.meta.dirname, '../components/review-map/report')
    const used = new Set<string>()
    for (const file of ['review-report-markdown.ts', 'review-report-html.ts']) {
      const src = readFileSync(join(dir, file), 'utf-8')
      const prefix = /const K = '([^']+)'/.exec(src)?.[1]
      for (const m of src.matchAll(/`\$\{K\}\.([A-Za-z0-9_.]+)`/g)) used.add(`${prefix}.${m[1]}`)
    }
    const missing = [...used].filter((k) => typeof lookup(en, k) !== 'string')
    expect(missing).toEqual([])
  })
})
