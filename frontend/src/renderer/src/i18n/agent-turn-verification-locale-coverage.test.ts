import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Keys enumerated from agent-turn-verification-view-model.ts (089-05) i18n key mappings.
// Read-by-name, so this test prevents silent English fallbacks in non-English locales.
const KEYS = [
  'auto.components.reviewMap.turns.verification.agreement.verified',
  'auto.components.reviewMap.turns.verification.agreement.unverified',
  'auto.components.reviewMap.turns.verification.agreement.contradicted',
  'auto.components.reviewMap.turns.verification.agreement.partial',
  'auto.components.reviewMap.turns.verification.agreement.unknown',
  'auto.components.reviewMap.turns.verification.basis.stated',
  'auto.components.reviewMap.turns.verification.ranNothing',
  'auto.components.reviewMap.turns.verification.ran',
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

describe('Agent turn verification locale catalog entries', () => {
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: all keys exist and are non-empty strings`, () => {
      for (const key of KEYS) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key} should be a string`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key} should not be empty`).not.toBe('')
      }
    })
  }
})
