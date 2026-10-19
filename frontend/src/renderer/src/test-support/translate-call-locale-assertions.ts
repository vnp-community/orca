/**
 * Locale-coverage assertions driven by the source itself: every literal `translate('key', 'fallback')` in the given files
 * must exist in all five catalogs, en must equal the in-code fallback, and no locale may drop or add a placeholder.
 */

import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import en from '../i18n/locales/en.json'
import es from '../i18n/locales/es.json'
import ja from '../i18n/locales/ja.json'
import ko from '../i18n/locales/ko.json'
import zh from '../i18n/locales/zh.json'

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const CALL_RE = /translate\(\s*'([A-Za-z0-9_.]+)',\s*'((?:[^'\\]|\\.)*)'/g

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

export function describeTranslateCallLocales(
  group: string,
  absolutePaths: readonly string[]
): void {
  const calls = new Map<string, string>()
  for (const path of absolutePaths) {
    for (const match of readFileSync(path, 'utf8').matchAll(CALL_RE)) {
      calls.set(match[1], match[2].replace(/\\'/g, "'"))
    }
  }

  describe(`locale coverage: ${group}`, () => {
    it('finds the translate calls it is meant to check', () => {
      expect(calls.size).toBeGreaterThan(0)
    })

    it('en equals the in-code fallback', () => {
      for (const [key, fallback] of calls) {
        expect(lookup(en, key), key).toBe(fallback)
      }
    })

    for (const [locale, catalog] of Object.entries(CATALOGS)) {
      it(`${locale} has every key, with the same placeholders as en`, () => {
        for (const [key, fallback] of calls) {
          const value = lookup(catalog, key)
          expect(typeof value, `${locale}: ${key}`).toBe('string')
          expect((value as string).trim(), `${locale}: ${key}`).not.toBe('')
          expect(placeholders(value as string), `${locale}: ${key}`).toEqual(placeholders(fallback))
        }
      })
    }
  })
}
