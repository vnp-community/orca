import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Keys behind the Review tab (FE-CV-TASK-050-20). They are read by name, not hashed, so nothing
// generates their catalog entries: this test keeps non-English UIs from silently showing English.
// Each later code-intel solution appends its own keys here in the same change.
const KEYS = [
  'auto.lib.ensure.review.tab.title',
  'auto.settings.codeIntel.loading',
  'auto.settings.codeIntel.notAuthorized',
  'auto.settings.codeIntel.codeIntelLabel',
  'auto.settings.codeIntel.qualityLabel',
  'auto.settings.codeIntel.needsMaster',
  'auto.settings.codeIntel.serverOff',
  'auto.settings.codeIntel.readOnly',
  'auto.components.settings.Settings.codeIntelTitle',
  'auto.components.settings.Settings.codeIntelDesc',
  'auto.hooks.useSettingsNavigationMetadata.codeIntelTitle',
  'auto.hooks.useSettingsNavigationMetadata.codeIntelDesc',
  'auto.components.reviewMap.shell.dock.label',
  'auto.components.reviewMap.shell.dock.findings',
  'auto.components.reviewMap.shell.dock.notes',
  'auto.components.reviewMap.shell.dock.collapse',
  'auto.components.reviewMap.shell.dock.expand',
  'auto.components.reviewMap.shell.turnFilter',
  'auto.components.reviewMap.symbolDetail.indexLines'
]

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

describe('code-intel catalog entries', () => {
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every key as a non-empty string`, () => {
      for (const key of KEYS) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key}`).not.toBe('')
      }
    })
  }

  for (const locale of ['es', 'ja', 'ko', 'zh']) {
    it(`${locale} is translated, not a copy of the English text`, () => {
      for (const key of KEYS) {
        expect(lookup(CATALOGS[locale], key), `${locale}: ${key}`).not.toBe(lookup(en, key))
      }
    })
  }
})
