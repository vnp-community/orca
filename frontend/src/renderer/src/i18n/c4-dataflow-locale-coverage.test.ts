import { describe, expect, it } from 'vitest'
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const P = 'auto.components.reviewMap.'
// Keys built at runtime in the C4 / data-flow lenses.
const DYNAMIC = [
  ...['grpc-server', 'usecase', 'domain', 'adapter', 'external', 'support'].map((k) => `c4.layer.${k}`),
  ...['uses', 'implements', 'calls-rpc', 'reads', 'writes', 'publishes', 'subscribes', 'unknown'].map((k) => `c4.relation.${k}`),
  ...['c4_yaml', 'package-doc', 'none'].map((k) => `c4.descSource.${k}`)
].map((k) => P + k)

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {return undefined}
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

function usedKeys(): string[] {
  const base = join(import.meta.dirname, '../components/review-map')
  const keys = new Set<string>(DYNAMIC)
  for (const dir of ['c4', 'dataflow']) {
    for (const file of readdirSync(join(base, dir))) {
      if (!file.endsWith('.tsx') || file.includes('.test.')) {continue}
      const src = readFileSync(join(base, dir, file), 'utf-8')
      for (const m of src.matchAll(/translate\(\s*'(auto\.components\.reviewMap\.[A-Za-z0-9_.-]+)'/g)) {keys.add(m[1])}
    }
  }
  return [...keys]
}

const placeholders = (s: string): string => (s.match(/\{\{\w+\}\}/g) ?? []).sort().join(',')

describe('C4 and data-flow catalog entries', () => {
  const keys = usedKeys()
  it('finds the keys the lenses use', () => {
    expect(keys.length).toBeGreaterThan(100)
  })
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: every key exists, is non-empty and keeps the English placeholders`, () => {
      for (const key of keys) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key} empty`).not.toBe('')
        expect(placeholders(value as string), `${locale}: ${key} placeholders`).toBe(placeholders(lookup(en, key) as string))
      }
    })
  }
  it('English copy never presents the inferred diagram as verified', () => {
    const text = [`${P}c4.notice.inferred`, `${P}c4.origin.derived`].map((k) => lookup(en, k) as string).join(' ').toLowerCase()
    expect(text).toContain('inferred')
    for (const banned of ['verified', 'accurate', 'correct']) {expect(text).not.toContain(banned)}
  })
})
