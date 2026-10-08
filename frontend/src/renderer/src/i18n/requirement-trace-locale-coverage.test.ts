import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

const T = 'auto.components.reviewMap.requirements.trace'
const P = 'auto.components.reviewMap.requirements.taskPicker'
const KEYS = [
  ...['has_evidence', 'partial', 'no_evidence', 'manual_pending', 'unknown'].map((k) => `${T}.state.${k}`),
  ...['noEvidence', 'partial', 'manualPending', 'hasEvidence', 'unknown', 'unlinked'].map((k) => `${T}.group.${k}`),
  ...['explicit', 'derived', 'inferred', 'none'].map((k) => `${T}.link.${k}`),
  ...['no_structured_criteria', 'index_stale', 'generic'].map((k) => `${T}.warning.${k}`),
  ...['suggestions', 'inferred', 'confirm', 'dismiss', 'manualConfirm', 'loading', 'error', 'retry', 'label',
    'showInferred', 'stale', 'readOnly', 'actionError', 'empty'].map((k) => `${T}.${k}`),
  ...['link', 'change', 'unlink', 'searchPlaceholder', 'noResults'].map((k) => `${P}.${k}`),
  'auto.components.reviewMap.lens.requirements.label'
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

describe('Requirement trace catalog entries', () => {
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: all keys exist and are non-empty strings`, () => {
      for (const key of KEYS) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key} empty`).not.toBe('')
      }
    })
  }

  it('English wording never claims a requirement is met', () => {
    const text = KEYS.map((k) => lookup(en, k) as string).join('\n').toLowerCase()
    for (const banned of ['satisfied', 'fulfilled', 'completed', 'is met', 'requirement met', 'passed']) {
      expect(text, banned).not.toContain(banned)
    }
  })

  it('unknown and no-evidence read differently', () => {
    expect(lookup(en, `${T}.state.unknown`)).not.toBe(lookup(en, `${T}.state.no_evidence`))
  })

  it('every translate key used by the requirements sources exists in the catalog', () => {
    const dir = join(import.meta.dirname, '../components/review-map/requirements')
    const used = new Set<string>()
    for (const file of ['RequirementRow.tsx', 'RequirementEvidenceList.tsx', 'UnlinkedChangesList.tsx', 'RequirementTracePanel.tsx', 'WorktreeTaskLinkPicker.tsx']) {
      const src = readFileSync(join(dir, file), 'utf-8')
      for (const m of src.matchAll(/translate\('([A-Za-z0-9_.]+)'/g)) {
        if (m[1].startsWith('auto.')) used.add(m[1])
      }
      for (const m of src.matchAll(/`\$\{BASE\}\.([A-Za-z0-9_.]+)`/g)) {
        const base = /const BASE = '([^']+)'/.exec(src)?.[1]
        if (base && !m[1].includes('${')) used.add(`${base}.${m[1]}`)
      }
    }
    const missing = [...used].filter((k) => typeof lookup(en, k) !== 'string')
    expect(missing).toEqual([])
  })
})
