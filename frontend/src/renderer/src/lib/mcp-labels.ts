import { translate } from '@/i18n/i18n'
import type { McpRisk, McpScopeDescriptor } from '../../../shared/mcp-types'

// i18next treats ':' as a namespace separator, so scope ids use '_' in keys.
const scopeKey = (id: string): string => id.replace(/[^A-Za-z0-9]/g, '_')

const SCOPE_LABELS: Record<string, string> = {
  'orca:read': 'Read',
  'orca:write': 'Write',
  'orca:exec': 'Run commands',
  'orca:admin': 'Administer'
}

const RISK_LABELS: Record<McpRisk, string> = {
  read: 'Read only',
  write_reversible: 'Reversible change',
  exec: 'Runs commands',
  destructive: 'Destructive',
  admin: 'Admin'
}

/** Server label wins only when no local string exists; falls back to the raw id. */
export function mcpScopeLabel(scope: Pick<McpScopeDescriptor, 'id' | 'label'> | string): string {
  const id = typeof scope === 'string' ? scope : scope.id
  const serverLabel = typeof scope === 'string' ? undefined : scope.label
  return translate(`auto.mcp.scope.${scopeKey(id)}`, serverLabel || SCOPE_LABELS[id] || id)
}

export function mcpScopeDescription(scope: McpScopeDescriptor): string {
  return translate(`auto.mcp.scope.${scopeKey(scope.id)}_description`, scope.description)
}

export function mcpRiskLabel(risk: McpRisk): string {
  return translate(`auto.mcp.risk.${risk}`, RISK_LABELS[risk] ?? risk)
}
