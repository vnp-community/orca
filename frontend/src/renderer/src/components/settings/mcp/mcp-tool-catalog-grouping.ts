import type { McpDecision, McpRisk, McpToolView } from '../../../../../shared/mcp-types'

export type ToolGroupBy = 'namespace' | 'pack' | 'risk'
export type ToolFilters = {
  query: string
  namespace: string | 'all'
  risk: McpRisk | 'all'
  pack: 1 | 2 | 3 | 4 | 'all'
  decision: McpDecision | 'all'
  hideHardDenied: boolean
}

export const DEFAULT_TOOL_FILTERS: ToolFilters = {
  query: '',
  namespace: 'all',
  risk: 'all',
  pack: 'all',
  decision: 'all',
  hideHardDenied: false
}

/** Least to most dangerous; groups and sorting follow this order. */
export const RISK_ORDER: readonly McpRisk[] = [
  'read',
  'write_reversible',
  'exec',
  'destructive',
  'admin'
]

export type ToolGroup = { key: string; tools: McpToolView[] }

export function hasActiveToolFilters(f: ToolFilters): boolean {
  return (
    f.query.trim() !== '' ||
    f.namespace !== 'all' ||
    f.risk !== 'all' ||
    f.pack !== 'all' ||
    f.decision !== 'all' ||
    f.hideHardDenied
  )
}

export function filterTools(tools: readonly McpToolView[], f: ToolFilters): McpToolView[] {
  const q = f.query.trim().toLowerCase()
  return tools.filter((t) => {
    if (q && !`${t.name}\n${t.title}\n${t.description}`.toLowerCase().includes(q)) {
      return false
    }
    return (
      (f.namespace === 'all' || t.namespace === f.namespace) &&
      (f.risk === 'all' || t.risk === f.risk) &&
      (f.pack === 'all' || t.pack === f.pack) &&
      (f.decision === 'all' || t.effective === f.decision) &&
      !(f.hideHardDenied && t.hardDenied)
    )
  })
}

export function sortTools(tools: readonly McpToolView[]): McpToolView[] {
  return [...tools].sort(
    (a, b) => a.namespace.localeCompare(b.namespace) || a.name.localeCompare(b.name)
  )
}

function groupKey(t: McpToolView, by: ToolGroupBy): string {
  return by === 'namespace' ? t.namespace : by === 'pack' ? String(t.pack) : t.risk
}

function compareKeys(a: string, b: string, by: ToolGroupBy): number {
  if (by === 'namespace') {
    return a.localeCompare(b)
  }
  if (by === 'pack') {
    return Number(a) - Number(b)
  }
  return RISK_ORDER.indexOf(a as McpRisk) - RISK_ORDER.indexOf(b as McpRisk)
}

/** Keys only (namespace, pack number, risk id); the component translates labels. */
export function groupTools(tools: readonly McpToolView[], by: ToolGroupBy): ToolGroup[] {
  const map = new Map<string, McpToolView[]>()
  for (const t of sortTools(tools)) {
    const key = groupKey(t, by)
    const list = map.get(key)
    if (list) {
      list.push(t)
    } else {
      map.set(key, [t])
    }
  }
  return [...map.entries()]
    .map(([key, list]) => ({ key, tools: list }))
    .sort((a, b) => compareKeys(a.key, b.key, by))
}

export function summarizeTools(tools: readonly McpToolView[]): {
  total: number
  byDecision: Record<McpDecision, number>
  hardDenied: number
} {
  const byDecision: Record<McpDecision, number> = { allow: 0, require_approval: 0, deny: 0 }
  let hardDenied = 0
  for (const t of tools) {
    byDecision[t.effective]++
    if (t.hardDenied) {
      hardDenied++
    }
  }
  return { total: tools.length, byDecision, hardDenied }
}

export function listToolNamespaces(tools: readonly McpToolView[]): string[] {
  return [...new Set(tools.map((t) => t.namespace))].sort((a, b) => a.localeCompare(b))
}

/** Groups at or below this size start open; larger ones start closed to avoid long scrolls. */
export const AUTO_OPEN_GROUP_MAX = 12
