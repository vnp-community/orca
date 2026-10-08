/**
 * quality-copy-locale-assertions.ts — FE-CV-TASK-087-08
 *
 * Shared locale-coverage assertions for the review-quality copy tables (one table per feature
 * group). Each group's own test file calls `describeQualityCopyLocales`.
 */

import { describe, expect, it } from 'vitest'
import en from '../i18n/locales/en.json'
import es from '../i18n/locales/es.json'
import ja from '../i18n/locales/ja.json'
import ko from '../i18n/locales/ko.json'
import zh from '../i18n/locales/zh.json'
import { qualityCopyPrefix } from '../components/review-map/quality/quality-copy-factory'

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }

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

function placeholders(text: string): string[] {
  return text.match(/\{\{\w+\}\}/g)?.sort() ?? []
}

/**
 * @param sameAsEnglish keys (per locale) whose translation is legitimately identical to English.
 */
export function describeQualityCopyLocales(
  group: string,
  table: Record<string, string>,
  sameAsEnglish: Partial<Record<'es' | 'ja' | 'ko' | 'zh', string[]>> = {}
): void {
  const prefix = qualityCopyPrefix(group)
  const keys = Object.keys(table)

  describe(`review quality catalog: ${group}`, () => {
    it('lists keys', () => {
      expect(keys.length).toBeGreaterThan(0)
    })

    for (const [locale, catalog] of Object.entries(CATALOGS)) {
      it(`${locale} has every key as a non-empty string`, () => {
        for (const key of keys) {
          const value = lookup(catalog, `${prefix}${key}`)
          expect(typeof value, `${locale}: ${key}`).toBe('string')
          expect((value as string).trim(), `${locale}: ${key}`).not.toBe('')
        }
      })
    }

    it('en matches the in-code fallbacks', () => {
      for (const [key, text] of Object.entries(table)) {
        expect(lookup(en, `${prefix}${key}`), key).toBe(text)
      }
    })

    for (const locale of ['es', 'ja', 'ko', 'zh'] as const) {
      it(`${locale} is translated and keeps interpolation placeholders`, () => {
        for (const key of keys) {
          const value = lookup(CATALOGS[locale], `${prefix}${key}`) as string
          const english = lookup(en, `${prefix}${key}`) as string
          if (!(sameAsEnglish[locale] ?? []).includes(key)) {
            expect(value, `${locale}: ${key}`).not.toBe(english)
          }
          expect(placeholders(value), `${locale}: ${key}`).toEqual(placeholders(english))
        }
      })
    }
  })
}
