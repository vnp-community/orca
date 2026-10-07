import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Keys are read-by-name (not auto-generated), so this test prevents
// non-English locales from silently showing English for quality gate notices.
// Keys enumerated from buildQualityNoticeViewModel i18n key mappings (085-02).
const KEYS = [
  'auto.components.right.sidebar.qualityGateNotice.title.fail',
  'auto.components.right.sidebar.qualityGateNotice.title.warn',
  'auto.components.right.sidebar.qualityGateNotice.title.unknown',
  'auto.components.right.sidebar.qualityGateNotice.stale',
  'auto.components.right.sidebar.qualityGateNotice.timedOut',
  'auto.components.right.sidebar.qualityGateNotice.unavailable',
  'auto.components.right.sidebar.qualityGateNotice.reasons',
  'auto.components.right.sidebar.qualityGateNotice.openReason',
  'auto.components.right.sidebar.qualityGateNotice.runChecks',
  'auto.components.right.sidebar.qualityGateNotice.moreReasons',
  'auto.components.right.sidebar.qualityGateNotice.reason.coverage_below_threshold',
  'auto.components.right.sidebar.qualityGateNotice.reason.complexity_exceeded',
  'auto.components.right.sidebar.qualityGateNotice.reason.duplication_exceeded',
  'auto.components.right.sidebar.qualityGateNotice.reason.security_violations',
  'auto.components.right.sidebar.qualityGateNotice.reason.reliability_violations',
  'auto.components.right.sidebar.qualityGateNotice.reason.maintainability_violations',
  'auto.components.right.sidebar.qualityGateNotice.reason.tech_debt_exceeded',
  'auto.components.right.sidebar.qualityGateNotice.reason.hotspot_count_exceeded',
  'auto.components.right.sidebar.qualityGateNotice.reason.new_violations_found',
  'auto.components.right.sidebar.qualityGateNotice.reason.dependency_vulnerabilities',
  'auto.components.right.sidebar.qualityGateNotice.reason.unknown',
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

describe('Quality gate notice catalog entries', () => {
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
