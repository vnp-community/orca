import { describe, expect, it } from 'vitest'
import type { McpAuditEntry } from '../../../../../shared/mcp-types'
import { csvCell, mcpAuditCsvFilename, toCsv, MCP_AUDIT_CSV_COLUMNS } from './mcp-audit-csv'
import { EMPTY_AUDIT_FILTERS, auditFilterIssue, toAuditParams } from './mcp-audit-filters'

const entry = (over: Partial<McpAuditEntry> = {}): McpAuditEntry => ({
  id: '1',
  at: '2026-10-01T00:00:00Z',
  actorType: 'agent',
  userId: 'u',
  clientName: 'c',
  sessionId: 's',
  tool: 't',
  risk: 'read',
  decision: 'allow',
  argsSummary: 'x',
  result: 'ok',
  ...over
})

describe('csv', () => {
  it('guards formula injection with a leading apostrophe', () => {
    for (const bad of ['=cmd|calc', '+1', '-1', '@x', '\tx', '\rx']) {
      expect(csvCell(bad).replace(/^"/, '')).toMatch(/^'/)
    }
    expect(csvCell('safe')).toBe('safe')
    expect(csvCell(12)).toBe('12')
  })
  it('escapes commas, quotes and newlines', () => {
    expect(csvCell('a,b')).toBe('"a,b"')
    expect(csvCell('say "hi"')).toBe('"say ""hi"""')
    expect(csvCell('l1\nl2')).toBe('"l1\nl2"')
  })
  it('emits BOM, CRLF and the header in order; guards agent-controlled args', () => {
    const csv = toCsv([entry({ argsSummary: '=HYPERLINK("x")' })])
    expect(csv.startsWith(`\uFEFF${MCP_AUDIT_CSV_COLUMNS.join(',')}`)).toBe(true)
    expect(csv.split('\r\n')).toHaveLength(3)
    expect(csv).toContain(`"'=HYPERLINK(""x"")"`)
  })
  it('filename has no colon', () => {
    expect(mcpAuditCsvFilename(new Date(2026, 9, 2, 8, 5))).toBe('mcp-audit-20261002-0805.csv')
  })
})

describe('filters', () => {
  it('builds params: local-day bounds as ISO UTC, empty fields dropped', () => {
    const p = toAuditParams({
      ...EMPTY_AUDIT_FILTERS,
      from: '2026-10-01',
      to: '2026-10-02',
      decision: 'approved'
    })
    expect(p.from).toBe(new Date(2026, 9, 1, 0, 0, 0, 0).toISOString())
    expect(p.to).toBe(new Date(2026, 9, 2, 23, 59, 59, 999).toISOString())
    expect(p).toEqual({ from: p.from, to: p.to, decision: 'approved' })
    expect(toAuditParams(EMPTY_AUDIT_FILTERS)).toEqual({})
  })
  it('flags non-UUID user ids and inverted ranges', () => {
    expect(auditFilterIssue({ ...EMPTY_AUDIT_FILTERS, userId: 'bob' })).toBe('userId')
    expect(
      auditFilterIssue({ ...EMPTY_AUDIT_FILTERS, userId: '123e4567-e89b-12d3-a456-426614174000' })
    ).toBeNull()
    expect(auditFilterIssue({ ...EMPTY_AUDIT_FILTERS, from: '2026-10-02', to: '2026-10-01' })).toBe(
      'range'
    )
  })
})
