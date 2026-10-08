import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Keys enumerated from agent-turn-verification-view-model.ts (089-05) and AgentTurnVerificationLine.tsx (089-06).
// Read-by-name, so this test prevents silent English fallbacks in non-English locales.
const BASE = 'auto.components.reviewMap.turns.verification'
const KEYS = [
  ...['consistent', 'contradicted', 'unverified', 'unknown'].map((k) => `${BASE}.agreement.${k}`),
  ...['tests_pass', 'tests_fail', 'lint_clean', 'typecheck_clean', 'build_ok', 'all_done', 'other'].map(
    (k) => `${BASE}.kind.${k}`
  ),
  ...['tree_may_differ', 'no_run', 'run_failed'].map((k) => `${BASE}.reason.${k}`),
  ...['test', 'lint', 'typecheck', 'build', 'install', 'git', 'other'].map((k) => `${BASE}.category.${k}`),
  ...['basis.stated', 'ran', 'ranItemMany', 'ranMore', 'recordedNote', 'viewRun', 'runChecks'].map(
    (k) => `${BASE}.${k}`
  )
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
