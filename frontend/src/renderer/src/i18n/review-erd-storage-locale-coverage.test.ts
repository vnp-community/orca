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
const DIRS = ['erd', 'storage'].map((d) => resolve(__dirname, '../components/review-map', d))
// Words that are legitimately identical across languages.
const SAME_AS_ENGLISH = new Set(['StorageNodeDetail.stream', 'ErdTableDetail.no'])

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

function staticKeys(): string[] {
  const keys = new Set<string>()
  for (const dir of DIRS) {
    for (const file of readdirSync(dir)) {
      if (!/\.(ts|tsx)$/.test(file) || file.includes('.test.') || file.includes('.fixture.')) {
        continue
      }
      const source = readFileSync(join(dir, file), 'utf8')
      for (const m of source.matchAll(
        /['`](auto\.components\.reviewMap\.(?:Erd|Storage)[A-Za-z.]+)['`]/g
      )) {
        keys.add(m[1])
      }
    }
  }
  return [...keys]
}

// Keys composed at runtime (prefix + id) cannot be found by scanning.
const DYNAMIC = [
  ...['added', 'modified', 'removed', 'related'].map((m) => `StorageCanvas.mark.${m}`),
  ...['inferred', 'derived'].map((c) => `StorageCanvas.confidence.${c}`),
  ...['added', 'modified', 'removed'].map((m) => `ErdTableNode.change.${m}`)
].map((k) => PREFIX + k)

const placeholders = (s: string): string[] => (s.match(/\{\{\w+\}\}/g) ?? []).sort()
const ALL = [...new Set([...staticKeys(), ...DYNAMIC])].filter((k) => !k.endsWith('.'))

describe('ERD + Storage lens locale catalog', () => {
  it('finds a non-trivial set of keys', () => {
    expect(ALL.length).toBeGreaterThan(120)
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: every key exists, is non-empty and keeps its placeholders`, () => {
      for (const key of ALL) {
        const english = lookup(en, key)
        const value = lookup(catalog, key)
        expect(typeof english, `en: ${key}`).toBe('string')
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key} empty`).not.toBe('')
        expect(placeholders(value as string), `${locale}: ${key}`).toEqual(
          placeholders(english as string)
        )
        if (locale !== 'en' && !SAME_AS_ENGLISH.has(key.slice(PREFIX.length))) {
          expect(value, `${locale}: ${key} is untranslated`).not.toBe(english)
        }
      }
    })
  }
})
