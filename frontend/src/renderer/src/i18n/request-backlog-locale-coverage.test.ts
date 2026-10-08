import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Backlog keys are read by name; this keeps every locale in step with them (CR-REQ-023-07).
const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const B = 'auto.components.request.backlog.'

const KEYS = [
  ...['request', 'task', 'execute'].map((k) => `${B}BacklogSegmentControl.${k}`),
  ...['search', 'type', 'category', 'refresh', 'allTypes', 'allCategories'].map((k) => `${B}BacklogToolbar.${k}`),
  ...['request', 'source', 'type', 'stage', 'category', 'reason', 'by', 'at', 'actions'].map((k) => `${B}RequestBacklogTable.col.${k}`),
  `${B}RequestBacklogTable.actorSystem`, `${B}RequestBacklogTable.reopen`, `${B}RequestBacklogTable.cancel`,
  ...['task', 'estimate', 'dependencies'].map((k) => `${B}TaskBacklogTable.col.${k}`),
  ...['task', 'blockedBy', 'lastError', 'failedAttempts', 'engine'].map((k) => `${B}ExecuteBacklogTable.col.${k}`),
  `${B}ExecuteBacklogTable.noErrorDetail`,
  ...['classification', 'analysis', 'plan', 'phase', 'task'].map((k) => `${B}ReturnedFromStage.${k}`),
  ...['missing_info', 'infeasible', 'blocked_dependency', 'rejected', 'other'].map((k) => `${B}ReturnedCategory.${k}`),
  ...['approved', 'pending', 'rejected', 'none', 'unknown'].map((k) => `${B}GateStatus.${k}`),
  ...['workflow', 'orchestration', 'direct_agent'].map((k) => `${B}BacklogEngineBadge.${k}`),
  ...['title', 'body', 'cancel', 'confirm', 'viewRequest'].map((k) => `${B}ReopenRequestDialog.${k}`),
  ...['requests', 'tasks', 'execute', 'filtered', 'clearFilters'].map((k) => `${B}BacklogEmptyState.${k}`),
  ...['network', 'forbidden', 'retry'].map((k) => `${B}BacklogErrorState.${k}`),
  `${B}BacklogRow.reopened`, `${B}BacklogRow.alreadyHandled`, `${B}BacklogTab.loadMore`,
  `${B}BacklogGroupHeaderRow.openPlan`, `${B}TaskBacklogRow.planNotSplit`,
  `${B}BacklogTaskSheet.title`, `${B}BacklogTaskSheet.description`,
  'auto.components.request.CancelRequestDialog.reasonRequired',
  'auto.components.task.TaskBoardView.backlogMoved', 'auto.components.task.TaskBoardView.openBacklog'
]

// Spanish shares these words with English.
const SAME_AS_ENGLISH = new Set([
  `es:${B}ReturnedFromStage.plan`
])

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {return undefined}
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

describe('request backlog locale coverage', () => {
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every backlog key as a non-empty string`, () => {
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

  it('keeps the reopen dialog placeholders in every locale', () => {
    for (const catalog of Object.values(CATALOGS)) {
      const body = String(lookup(catalog, `${B}ReopenRequestDialog.body`))
      expect(body).toContain('{{number}}')
      expect(body).toContain('{{stage}}')
    }
  })
})
