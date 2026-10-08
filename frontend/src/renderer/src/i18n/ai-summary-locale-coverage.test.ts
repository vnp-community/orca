import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

const B = 'auto.components.reviewMap.aiSummary'
const KEYS = [
  ...['title', 'description', 'defaultProvider', 'counts', 'withheld', 'fileWithheld', 'moreFiles', 'injectionWarning', 'cancel', 'confirm',
    'level.metadata', 'level.diff', 'level.unknown'].map((k) => `${B}.preview.${k}`),
  ...['title', 'generate', 'regenerate', 'cancel', 'levelLabel', 'level.metadata', 'level.diff', 'preparing', 'generating', 'retry',
    'disclaimer', 'meta', 'risks', 'readFirst', 'refsDropped', 'helpful', 'incorrect', 'idleHint',
    ...['no-relay', 'bad-output', 'rate-limited', 'timeout', 'forbidden', 'unknown'].map((e) => `error.${e}`)].map((k) => `${B}.card.${k}`),
  ...['heading', 'disclaimer', 'risks', 'readFirst'].map((k) => `${B}.report.${k}`)
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
const placeholders = (v: string): string[] => [...v.matchAll(/\{\{(\w+)\}\}/g)].map((m) => m[1]).sort()

describe('AI summary catalog entries', () => {
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: keys exist, are non-empty, and keep the English placeholders`, () => {
      for (const key of KEYS) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key} empty`).not.toBe('')
        expect(placeholders(value as string), `${locale}: ${key} placeholders`).toEqual(placeholders(lookup(en, key) as string))
      }
    })
  }

  it('English never says the AI reviewed, approved or verified anything', () => {
    const text = KEYS.map((k) => lookup(en, k) as string).join('\n').toLowerCase()
    for (const banned of ['ai reviewed', 'reviewed by ai', 'approved', 'verified', 'is safe', 'checked by ai']) {
      expect(text, banned).not.toContain(banned)
    }
  })

  it('every key used in the AI summary sources exists in the catalog', () => {
    const dir = join(import.meta.dirname, '../components/review-map/ai-summary')
    const used = new Set<string>()
    for (const file of ['ReviewAiSummaryCard.tsx', 'AiSummaryDataPreviewDialog.tsx', 'ai-summary-report-section.ts']) {
      const src = readFileSync(join(dir, file), 'utf-8')
      const base = /const (?:BASE|K) = '([^']+)'/.exec(src)?.[1]
      for (const m of src.matchAll(/`\$\{(?:BASE|K)\}\.([A-Za-z0-9_.-]+)`/g)) used.add(`${base}.${m[1]}`)
    }
    const missing = [...used].filter((k) => typeof lookup(en, k) !== 'string')
    expect(missing).toEqual([])
  })
})
