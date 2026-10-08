import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Keys enumerated from review-map/contract (059-04), review-map/findings (059-05/06),
// review-map/notes and review-map/turns (060-03/04/08). Read-by-name with an English fallback in
// code, so this test prevents a silent English fallback in the other locales.
const BASE = 'auto.components.reviewMap.'
const CONTRACT_KEYS = [
  'contract.migration.label',
  'contract.migration.files',
  'contract.migration.dialectOnly',
  'contract.detail.openErd',
  'contract.detail.label',
  'contract.detail.files',
  'contract.detail.noFiles',
  'contract.detail.viewDiff',
  'contract.detail.notInChange',
  'contract.detail.consumers',
  'contract.detail.noConsumersBreaking',
  'contract.detail.noConsumers',
  'contract.change.added',
  'contract.change.removed',
  'contract.change.modified',
  'contract.change.unknown',
  'contract.table.noDetails',
  'contract.table.label',
  'contract.table.col.contract',
  'contract.table.col.before',
  'contract.table.col.after',
  'contract.table.col.compat',
  'contract.compat.breaking',
  'contract.compat.risky',
  'contract.compat.compatible',
  'contract.compat.unknown',
  'contract.kind.proto',
  'contract.kind.wsChannel',
  'contract.kind.route',
  'contract.kind.migration',
  'contract.kind.unknown',
  'contract.loading',
  'contract.error.noIndex',
  'contract.error.load',
  'contract.retry',
  'contract.filter.kind',
  'contract.filter.service',
  'contract.filter.allServices',
  'contract.filter.onlyBreaking',
  'contract.filter.onlyChanged',
  'contract.filter.search',
  'contract.summary.label',
  'contract.truncated',
  'contract.empty',
  'contract.emptyFiltered',
  'contract.group.noService',
  'contract.migration.title',
  'contract.signature.absent'
]
const FINDINGS_KEYS = [
  'findings.reason.not_applicable',
  'findings.reason.accepted_risk',
  'findings.reason.false_positive',
  'findings.reason.later',
  'findings.dismiss.reasonLegend',
  'findings.dismiss.notePlaceholder',
  'findings.dismiss.noteLabel',
  'findings.dismiss.scopeNote',
  'findings.dismiss.confirm',
  'findings.kind.layer_violation',
  'findings.kind.dependency_cycle',
  'findings.kind.hotspot',
  'findings.kind.missing_tenant_id',
  'findings.kind.dead_code',
  'findings.kind.rls_removed',
  'findings.kind.unknown',
  'findings.filter.onlyIntroduced',
  'findings.filter.showDismissed',
  'findings.filter.search',
  'findings.freshness.stale',
  'findings.severity.error',
  'findings.severity.warning',
  'findings.severity.info',
  'findings.severity.unknown',
  'findings.origin.introduced',
  'findings.origin.touched',
  'findings.origin.preexisting',
  'findings.origin.unknown',
  'findings.error.forbidden',
  'findings.error.offline',
  'findings.error.notFound',
  'findings.error.conflict',
  'findings.error.reasonRequired',
  'findings.error.generic',
  'findings.confidence',
  'findings.state.resolved',
  'findings.state.ignored',
  'findings.saving',
  'findings.action.viewInGraph',
  'findings.action.note',
  'findings.action.restore',
  'findings.action.ignore',
  'findings.action.resolve',
  'findings.retry',
  'findings.loading',
  'findings.error.noIndex',
  'findings.error.load',
  'findings.label',
  'findings.empty',
  'findings.emptyIndexed',
  'findings.action.viewDiff',
  'findings.action.openFile',
  'findings.list.partial',
  'findings.list.loadMore',
  'findings.title.layer_violation',
  'findings.title.dependency_cycle',
  'findings.title.hotspot',
  'findings.title.missing_tenant_id',
  'findings.title.dead_code',
  'findings.title.rls_removed'
]
const REVIEWNOTE_KEYS = [
  'ReviewNote.scope.all',
  'ReviewNote.scope.lens',
  'ReviewNote.scope.selection',
  'ReviewNote.previewButton',
  'ReviewNote.badge',
  'ReviewNote.lens.findings',
  'ReviewNote.sent',
  'ReviewNote.label',
  'ReviewNote.saveFailed',
  'ReviewNote.save',
  'ReviewNote.cancel',
  'ReviewNote.jump',
  'ReviewNote.edit',
  'ReviewNote.delete',
  'ReviewNote.panel',
  'ReviewNote.persistFailed',
  'ReviewNote.empty',
  'ReviewNote.attachesTo',
  'ReviewNote.noFile',
  'ReviewNote.placeholder',
  'ReviewNote.button',
  'ReviewNote.noFileReason',
  'ReviewNote.preview.title',
  'ReviewNote.preview.description',
  'ReviewNote.preview.tooMany'
]
const REVIEWSENT_KEYS = [
  'ReviewSent.hint.changed',
  'ReviewSent.hint.unchanged',
  'ReviewSent.empty',
  'ReviewSent.title',
  'ReviewSent.count',
  'ReviewSent.recordFailed'
]
const REVIEWTURN_KEYS = [
  'ReviewTurn.label.new',
  'ReviewTurn.label.changed',
  'ReviewTurn.label.unchanged',
  'ReviewTurn.label.reverted',
  'ReviewTurn.groupLabel',
  'ReviewTurn.mode.all',
  'ReviewTurn.mode.since',
  'ReviewTurn.mode.previous',
  'ReviewTurn.turns',
  'ReviewTurn.agent',
  'ReviewTurn.interrupted',
  'ReviewTurn.sentNotes',
  'ReviewTurn.noPrevious',
  'ReviewTurn.estimate',
  'ReviewTurn.readOnly',
  'ReviewTurn.saveFailed',
  'ReviewTurn.retry'
]

const KEYS = [
  ...CONTRACT_KEYS,
  ...FINDINGS_KEYS,
  ...REVIEWNOTE_KEYS,
  ...REVIEWSENT_KEYS,
  ...REVIEWTURN_KEYS
].map((k) => BASE + k)

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
  return (text.match(/\{\{\w+\}\}/g) ?? []).sort()
}

describe('Contract, findings, review-note and turn locale catalog entries', () => {
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: all keys exist and are non-empty strings`, () => {
      for (const key of KEYS) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key} should be a string`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key} should not be empty`).not.toBe('')
      }
    })

    it(`${locale}: placeholders match the English text`, () => {
      for (const key of KEYS) {
        expect(placeholders(lookup(catalog, key) as string), `${locale}: ${key}`).toEqual(
          placeholders(lookup(en, key) as string)
        )
      }
    })
  }

  it('translated locales differ from English for prose strings', () => {
    const prose = KEYS.filter((k) => (lookup(en, k) as string).length > 24)
    for (const locale of ['es', 'ja', 'ko', 'zh'] as const) {
      const same = prose.filter((k) => lookup(CATALOGS[locale], k) === lookup(en, k))
      expect(same, `${locale} untranslated`).toEqual([])
    }
  })
})
