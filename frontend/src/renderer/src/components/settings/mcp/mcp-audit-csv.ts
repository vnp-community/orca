import type { McpAuditEntry } from '../../../../../shared/mcp-types'

export const MCP_AUDIT_CSV_COLUMNS = [
  'id',
  'at',
  'userId',
  'userName',
  'clientName',
  'sessionId',
  'tool',
  'risk',
  'decision',
  'approver',
  'result',
  'durationMs',
  'traceId',
  'argsSummary'
] as const

// Agent-controlled text must not execute as a formula when opened in Excel/Sheets.
const FORMULA_PREFIX = /^[=+\-@\t\r]/

export function csvCell(value: unknown): string {
  let text = value === undefined || value === null ? '' : String(value)
  if (FORMULA_PREFIX.test(text)) {
    text = `'${text}`
  }
  return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

export function toCsv(rows: readonly McpAuditEntry[]): string {
  const lines = [MCP_AUDIT_CSV_COLUMNS.join(',')]
  for (const row of rows) {
    lines.push(
      MCP_AUDIT_CSV_COLUMNS.map((c) => csvCell((row as Record<string, unknown>)[c])).join(',')
    )
  }
  return `﻿${lines.join('\r\n')}\r\n`
}

/** mcp-audit-YYYYMMDD-HHmm.csv — no ':' so it is valid on Windows. */
export function mcpAuditCsvFilename(now: Date = new Date()): string {
  const p = (n: number): string => String(n).padStart(2, '0')
  return `mcp-audit-${now.getFullYear()}${p(now.getMonth() + 1)}${p(now.getDate())}-${p(now.getHours())}${p(now.getMinutes())}.csv`
}

export function downloadMcpAuditCsv(csv: string, now: Date = new Date()): void {
  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
  const a = document.createElement('a')
  a.href = url
  a.download = mcpAuditCsvFilename(now)
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
