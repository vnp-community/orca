import { describe, expect, it } from 'vitest'
import {
  QUALITY_CHART_COPY,
  QUALITY_CHART_COPY_PREFIX
} from '../components/quality-charts/quality-chart-copy'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Keys are read by name (not hashed), so nothing generates their catalog entries: this test keeps
// non-English UIs from silently falling back to English for the quality chart primitives.
// Shared with the review-quality features, which append their own key groups below.
const KEYS = Object.keys(QUALITY_CHART_COPY).map((k) => `${QUALITY_CHART_COPY_PREFIX}${k}`)

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }

// Strings that are legitimately identical to English in a given locale.
const SAME_AS_ENGLISH: Record<string, string[]> = {
  es: ['severity.error', 'treemap.tileLabel'],
  ko: ['treemap.tileLabel']
}

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

describe('quality chart catalog entries', () => {
  it('lists a non-trivial number of keys', () => {
    expect(KEYS.length).toBeGreaterThan(60)
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every key as a non-empty string`, () => {
      for (const key of KEYS) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key}`).not.toBe('')
      }
    })
  }

  it('en matches the in-code fallbacks', () => {
    for (const [key, text] of Object.entries(QUALITY_CHART_COPY)) {
      expect(lookup(en, `${QUALITY_CHART_COPY_PREFIX}${key}`)).toBe(text)
    }
  })

  for (const locale of ['es', 'ja', 'ko', 'zh']) {
    it(`${locale} is translated (not English) and keeps interpolation placeholders`, () => {
      for (const key of Object.keys(QUALITY_CHART_COPY)) {
        const full = `${QUALITY_CHART_COPY_PREFIX}${key}`
        const value = lookup(CATALOGS[locale], full) as string
        const english = lookup(en, full) as string
        if (!(SAME_AS_ENGLISH[locale] ?? []).includes(key)) {
          expect(value, `${locale}: ${key}`).not.toBe(english)
        }
        expect(value.match(/\{\{\w+\}\}/g)?.sort() ?? [], `${locale}: ${key}`).toEqual(
          english.match(/\{\{\w+\}\}/g)?.sort() ?? []
        )
      }
    })
  }
})
