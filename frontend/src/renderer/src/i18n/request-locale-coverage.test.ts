import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Request-flow keys are read by name (not hashed): this test keeps every locale
// in step with the keys the request components actually reference.
const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const PREFIX = 'auto.components.request.'

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {return undefined}
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

function collectSourceKeys(): string[] {
  const root = join(__dirname, '..', 'components', 'request')
  const keys = new Set<string>()
  const walk = (dir: string): void => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name)
      if (entry.isDirectory()) {walk(full)}
      else if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\./.test(entry.name)) {
        const src = readFileSync(full, 'utf8')
        for (const m of src.matchAll(/['"`](auto\.components\.request\.[A-Za-z0-9_.]+)['"`]/g)) {
          if (!m[1].endsWith('.')) {keys.add(m[1])}
        }
        // Keys written as `${T}name` where the file declares `const T = 'auto.components.request.X.'`.
        const prefix = /const T = '(auto\.components\.request\.[A-Za-z0-9_.]+\.)'/.exec(src)?.[1]
        if (prefix) {
          for (const m of src.matchAll(/`\$\{T\}([A-Za-z0-9_.]+)`/g)) {keys.add(prefix + m[1])}
        }
      }
    }
  }
  walk(root)
  return [...keys]
}

const BASE_KEYS = [
  'RequestPage.title',
  'RequestPage.tab.requests',
  'RequestPage.tab.approvals',
  'RequestPage.tab.backlog',
  'RequestUnsupportedNotice.title',
  'RequestUnsupportedNotice.body',
  'SidebarRequestNavButton.label',
  'SidebarRequestNavButton.badge',
  'error.forbidden',
  'error.notFound',
  'error.conflict',
  'error.invalidState',
  'error.network',
  'error.unavailable',
  'error.rateLimited',
  'error.unknown',
  ...['submitted', 'classifying', 'awaiting_type_confirmation', 'analyzing', 'awaiting_analysis_approval',
    'awaiting_information', 'planning', 'awaiting_plan_approval', 'executing', 'completed', 'cancelled',
    'request_backlog', 'unknown'].map((s) => `RequestStatus.${s}`),
  ...['bug', 'task', 'docs', 'question', 'hotfix', 'security', 'ops_request', 'change_request', 'refactor',
    'spike', 'performance', 'unknown'].flatMap((t) => [`RequestType.${t}.label`, `RequestType.${t}.description`]),
  ...['pending', 'approved', 'rejected', 'expired', 'unknown'].map((s) => `ApprovalStatus.${s}`),
  ...['classification', 'analysis', 'plan', 'phase', 'execution'].map((s) => `StageTimeline.step.${s}`),
  ...['done', 'current', 'pending', 'skipped'].map((s) => `StageTimeline.state.${s}`),
  ...['spawned_by_spike', 'spawned_by_question', 'followup_hotfix', 'escalation'].map((s) => `SpawnChildRequestDialog.reason.${s}`),
  ...['parent', 'child', 'followUp'].map((s) => `RequestRelatedTab.group.${s}`)
].map((k) => PREFIX + k)

describe('request locale coverage', () => {
  const keys = [...new Set([...BASE_KEYS, ...collectSourceKeys()])].filter(
    // Keys built dynamically (labelKey maps) are covered through BASE_KEYS.
    (k) => !k.endsWith('.step') && !k.endsWith('.unknown.')
  )

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every request key as a non-empty string`, () => {
      const missing = keys.filter((key) => {
        const value = lookup(catalog, key)
        return typeof value !== 'string' || value.trim() === ''
      })
      expect(missing).toEqual([])
    })
  }
})
