import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from '../../i18n/locales/en.json'
import es from '../../i18n/locales/es.json'
import ja from '../../i18n/locales/ja.json'
import ko from '../../i18n/locales/ko.json'
import zh from '../../i18n/locales/zh.json'

// The shared helper only reads literal keys; these files build keys as `${T}name`, so the prefix constants are resolved here.
const FILES = [
  'StartFromField.tsx',
  'StartFromManualEntry.tsx',
  'BaseRefPickerPopover.tsx',
  'BaseRefSaveFeedback.tsx',
  'BaseRefChangeControl.tsx',
  'RequestBaseRefRow.tsx',
  'base-ref-violation-message.ts',
  'base-ref-write-error.ts',
  'use-request-base-ref-save.ts',
  'create-request-submit.ts',
  'CreateRequestDialog.tsx',
  'execution/ExecutionFailureCard.tsx',
  'execution/execution-failure-cause.ts'
]
const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const STRING = String.raw`(?:'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)")`
const CALL = new RegExp(
  String.raw`translate\(\s*(?:\x60\$\{(\w+)\}([A-Za-z0-9_.]+)\x60|'([A-Za-z0-9_.]+)')\s*,\s*${STRING}`,
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

const calls = new Map<string, string>()
for (const file of FILES) {
  const source = readFileSync(join(__dirname, file), 'utf8')
  const prefixes = new Map(
    [...source.matchAll(/const (\w+) = '([^']+)'/g)].map((m) => [m[1], m[2]] as const)
  )
  for (const m of source.matchAll(CALL)) {
    const key = m[1] ? `${prefixes.get(m[1])}${m[2]}` : m[3]
    calls.set(key, (m[4] ?? m[5]).replace(/\\'/g, "'"))
  }
}

describe('locale coverage: starting branch UI', () => {
  it('finds the translate calls it is meant to check', () => {
    expect(calls.size).toBeGreaterThan(60)
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

  it('es, ja, ko and zh are translations, not copies of the English text', () => {
    const sentences = [...calls.keys()].filter((key) =>
      /[a-z]{4} [a-z]{3}/i.test(String(lookup(en, key)))
    )
    for (const [locale, catalog] of [
      ['es', es],
      ['ja', ja],
      ['ko', ko],
      ['zh', zh]
    ] as const) {
      const copied = sentences.filter((key) => lookup(catalog, key) === lookup(en, key))
      expect(copied, locale).toEqual([])
    }
  })
})
