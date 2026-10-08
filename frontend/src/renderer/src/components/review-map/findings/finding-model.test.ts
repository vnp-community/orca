import { describe, expect, it } from 'vitest'
import { countByKindAndSeverity, filterFindings, foldSearchText } from './finding-filter'
import { groupFindingsByKind, sortFindings } from './finding-sort'
import { toFindingRow } from './finding-view-model'
import { FINDINGS_MIXED, makeFinding } from '../../../test-support/contract-findings-fixtures'

const t = (_key: string, fallback: string, params?: Record<string, unknown>): string =>
  fallback.replace(/\{\{(\w+)\}\}/g, (_m, k: string) => String(params?.[k] ?? ''))

const rows = FINDINGS_MIXED.map((f) => toFindingRow(f, t))

describe('toFindingRow', () => {
  it('builds the title from titleKey + params', () => {
    expect(rows[0].title).toBe('Layer violation: handlers depends on repository')
    expect(rows[0].locationLabel).toBe('svc/handlers/order.go:40')
  })

  it('falls back to the raw rule for an unknown titleKey or kind', () => {
    const unknownTitle = toFindingRow(makeFinding({ titleKey: 'finding.future.title', rule: 'my.rule' }), t)
    expect(unknownTitle.title).toBe('my.rule')
    const unknownKind = toFindingRow(makeFinding({ kind: 'weird' as never, rule: 'r2' }), t)
    expect(unknownKind.kind).toBe('unknown')
    expect(unknownKind.title).toBe('r2')
  })

  it('masks DSN-like params and subject', () => {
    const row = toFindingRow(
      makeFinding({
        params: { from: 'postgres://u:hunter2@h/db', to: 'x' },
        subject: 'dsn postgres://u:hunter2@h/db'
      }),
      t
    )
    expect(row.params.from).not.toContain('hunter2')
    expect(row.title).not.toContain('hunter2')
    expect(row.subject).not.toContain('hunter2')
  })

  it('has no location when evidence is empty and reports dismissal', () => {
    const row = toFindingRow(
      makeFinding({
        evidence: [],
        dismissed: { by: 'u', at: '2026-10-07T00:00:00Z', reason: 'false_positive', disposition: 'ignored' }
      }),
      t
    )
    expect(row.locationLabel).toBeNull()
    expect(row.isDismissed).toBe(true)
    expect(row.disposition).toBe('ignored')
    expect(row.dismissReason).toBe('false_positive')
  })
})

describe('sort / group', () => {
  it('sorts by severity, then origin, then key deterministically', () => {
    const sorted = sortFindings(rows).map((r) => r.findingKey)
    expect(sorted).toEqual(['layer_violation:a->b', 'tenant:orders', 'hotspot:order.go', 'dead:util'])
    expect(sortFindings(rows.toReversed()).map((r) => r.findingKey)).toEqual(sorted)
  })

  it('groups by kind with the most severe group first', () => {
    const groups = groupFindingsByKind(rows)
    expect(groups[0].rows[0].severity).toBe('error')
    expect(groups.map((g) => g.kind)).toContain('hotspot')
  })
})

describe('filter / count', () => {
  it('hides dismissed rows unless includeDismissed', () => {
    const dismissed = toFindingRow(
      makeFinding({
        findingKey: 'd',
        dismissed: { by: 'u', at: 'x', reason: 'later', disposition: 'resolved' }
      }),
      t
    )
    expect(filterFindings([...rows, dismissed], {})).toHaveLength(4)
    expect(filterFindings([...rows, dismissed], { includeDismissed: true })).toHaveLength(5)
  })

  it('combines origin, kind and severity', () => {
    expect(filterFindings(rows, { origins: ['introduced'] })).toHaveLength(2)
    expect(filterFindings(rows, { origins: ['introduced'], kinds: ['missing_tenant_id'] })).toHaveLength(1)
    expect(filterFindings(rows, { severities: ['warning', 'info'] })).toHaveLength(2)
  })

  it('searches without diacritics or case', () => {
    expect(foldSearchText('Hộp ÉM')).toBe('hop em')
    expect(filterFindings(rows, { query: 'SVC/ORDER.GO' })).toHaveLength(1)
  })

  it('counts by kind and severity', () => {
    const c = countByKindAndSeverity(rows)
    expect(c.total).toBe(4)
    expect(c.bySeverity.error).toBe(2)
    expect(c.byKind.hotspot).toBe(1)
  })
})
