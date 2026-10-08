import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Approval inbox keys are read by name; this keeps every locale in step with them (CR-REQ-022-07).
const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const P = 'auto.components.request.approval.'
const SUBJECT_KINDS = ['request_type', 'solution', 'findings', 'answer', 'plan', 'phase', 'task_list', 'pre_deploy']

const KEYS = [
  ...['title', 'empty', 'emptyFiltered', 'clearFilters', 'loadMore'].map((k) => `ApprovalInboxTab.${k}`),
  ...['all', 'requestType', 'solution', 'plan', 'phase', 'preDeploy', 'other'].map((k) => `ApprovalSubjectFilter.${k}`),
  ...['open', 'approve', 'reject', 'dueIn', 'overdueBy', 'requestedBy', 'systemActor', 'alreadyDecided',
    'changed', 'requestGone', 'decided'].map((k) => `ApprovalRow.${k}`),
  ...['request_type', 'findings', 'answer', 'task_list', 'phase', 'pre_deploy'].map((k) => `ApprovalRow.confirmApprove.${k}`),
  'OverdueToggle.label',
  ...['network', 'forbidden', 'retry'].map((k) => `ApprovalInboxErrorState.${k}`),
  ...[...SUBJECT_KINDS, 'unknown'].map((k) => `ApprovalSubjectType.${k}`)
].map((k) => P + k)

// Same word in English and Spanish.
const SAME_AS_ENGLISH = new Set(
  ['ApprovalSubjectFilter.plan', 'ApprovalSubjectType.plan'].map((k) => `es:${P}${k}`)
)

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {return undefined}
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

describe('request approval locale coverage', () => {
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every approval inbox key as a non-empty string`, () => {
      const missing = KEYS.filter((k) => {
        const v = lookup(catalog, k)
        return typeof v !== 'string' || v.trim() === ''
      })
      expect(missing).toEqual([])
    })
  }

  for (const locale of ['es', 'ja', 'ko', 'zh']) {
    it(`${locale} does not reuse the English text`, () => {
      const same = KEYS.filter(
        (k) => lookup(CATALOGS[locale], k) === lookup(en, k) && !SAME_AS_ENGLISH.has(`${locale}:${k}`)
      )
      expect(same).toEqual([])
    })
  }

  it('keeps interpolation placeholders in every locale', () => {
    for (const [key, token] of [[`${P}ApprovalRow.dueIn`, '{{relative}}'], [`${P}ApprovalRow.overdueBy`, '{{relative}}'], [`${P}ApprovalRow.requestedBy`, '{{name}}']]) {
      for (const catalog of Object.values(CATALOGS)) {
        expect(String(lookup(catalog, key))).toContain(token)
      }
    }
  })
})
