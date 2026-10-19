import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from '../../../i18n/locales/en.json'
import es from '../../../i18n/locales/es.json'
import ja from '../../../i18n/locales/ja.json'
import ko from '../../../i18n/locales/ko.json'
import zh from '../../../i18n/locales/zh.json'

// The shared reader only sees literal keys; activity-entry-text.ts spells them as `${T}name`, so the prefix is resolved here.
const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const STRING = String.raw`(?:'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)")`
const CALL = new RegExp(
  String.raw`translate\(\s*\x60\$\{(\w+)\}([A-Za-z0-9_.]+)\x60\s*,\s*${STRING}`,
  'g'
)

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

const placeholders = (text: string): string[] => text.match(/\{\{\w+\}\}/g)?.sort() ?? []

const source = readFileSync(join(__dirname, 'activity-entry-text.ts'), 'utf8')
const prefixes = new Map([...source.matchAll(/const (\w+) = '([^']+)'/g)].map((m) => [m[1], m[2]]))
const calls = new Map<string, string>()
for (const m of source.matchAll(CALL)) {
  calls.set(`${prefixes.get(m[1])}${m[2]}`, (m[3] ?? m[4]).replace(/\\'/g, "'"))
}

describe('locale coverage: request activity text', () => {
  it('finds the translate calls it is meant to check, including the CR-REQ-040/041 ones', () => {
    expect(calls.size).toBeGreaterThan(40)
    expect(calls.get('auto.components.request.activity.title.analysisRevisionRequested')).toBe(
      'Requested changes to the analysis'
    )
    expect(calls.has('auto.components.request.activity.cause.baseRefNotFound')).toBe(true)
  })

  it('en equals the in-code fallback', () => {
    for (const [key, fallback] of calls) {
      expect(lookup(en, key), key).toBe(fallback)
    }
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every key with the same placeholders as en`, () => {
      for (const [key, fallback] of calls) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key}`).not.toBe('')
        expect(placeholders(value as string), `${locale}: ${key}`).toEqual(placeholders(fallback))
      }
    })
  }

  it('the two new keys are translated, not copied from English', () => {
    for (const [locale, catalog] of [
      ['es', es],
      ['ja', ja],
      ['ko', ko],
      ['zh', zh]
    ] as const) {
      for (const key of [
        'auto.components.request.activity.title.analysisRevisionRequested',
        'auto.components.request.activity.cause.baseRefNotFound'
      ]) {
        expect(lookup(catalog, key), `${locale}: ${key}`).not.toBe(lookup(en, key))
      }
    }
  })
})
