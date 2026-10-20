import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from '../../../../i18n/locales/en.json'
import es from '../../../../i18n/locales/es.json'
import ja from '../../../../i18n/locales/ja.json'
import ko from '../../../../i18n/locales/ko.json'
import zh from '../../../../i18n/locales/zh.json'
import { describeTranslateCallLocales } from '../../../../test-support/translate-call-locale-assertions'

const sources = readdirSync(__dirname)
  .filter((name) => /\.(ts|tsx)$/.test(name) && !/\.test\./.test(name))
  .map((name) => join(__dirname, name))

describeTranslateCallLocales('plan revision', sources)

// Why: a translation that is the English text word for word is a missed translation, unless it is a word that is the same in
// the language (listed here) or a symbol.
const SAME_IN_LANGUAGE: Record<string, readonly string[]> = {
  es: ['Irreversible', 'Error', '{{hours}} h']
}
const CALL_RE = /translate\(\s*'([A-Za-z0-9_.]+)',/g

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

describe('plan revision: translations are not the English text', () => {
  const keys = sources.flatMap((path) =>
    [...readFileSync(path, 'utf8').matchAll(CALL_RE)].map((m) => m[1])
  )
  for (const [locale, catalog] of Object.entries({ es, ja, ko, zh })) {
    it(`${locale} translates every string`, () => {
      const copied = keys.filter((key) => {
        const value = lookup(catalog, key)
        return (
          value === lookup(en, key) && !(SAME_IN_LANGUAGE[locale] ?? []).includes(value as string)
        )
      })
      expect(copied).toEqual([])
    })
  }
})
