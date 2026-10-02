import type { McpAuditEntry, McpRpcParams } from '../../../../../shared/mcp-types'

export type McpAuditFilterState = {
  from: string
  to: string
  userId: string
  tool: string
  decision: McpAuditEntry['decision'] | ''
}

export const EMPTY_AUDIT_FILTERS: McpAuditFilterState = {
  from: '',
  to: '',
  userId: '',
  tool: '',
  decision: ''
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export const hasAuditFilters = (f: McpAuditFilterState): boolean =>
  Object.values(f).some((v) => v !== '')

function dayBound(date: string, end: boolean): string | null {
  const [y, m, d] = date.split('-').map(Number)
  if (!y || !m || !d) {
    return null
  }
  const dt = end ? new Date(y, m - 1, d, 23, 59, 59, 999) : new Date(y, m - 1, d, 0, 0, 0, 0)
  return Number.isNaN(dt.getTime()) ? null : dt.toISOString()
}

export type McpAuditFilterIssue = 'userId' | 'range' | null

export function auditFilterIssue(f: McpAuditFilterState): McpAuditFilterIssue {
  if (f.userId.trim() && !UUID.test(f.userId.trim())) {
    return 'userId'
  }
  if (f.from && f.to && f.from > f.to) {
    return 'range'
  }
  return null
}

/** Empty fields are dropped; from/to are start/end of the local day as RFC 3339 UTC. */
export function toAuditParams(f: McpAuditFilterState): McpRpcParams<'mcp.admin.audit.query'> {
  const from = f.from ? dayBound(f.from, false) : null
  const to = f.to ? dayBound(f.to, true) : null
  return {
    ...(from ? { from } : {}),
    ...(to ? { to } : {}),
    ...(f.userId.trim() ? { userId: f.userId.trim() } : {}),
    ...(f.tool.trim() ? { tool: f.tool.trim() } : {}),
    ...(f.decision ? { decision: f.decision } : {})
  }
}
