/**
 * code-intel-findings-types.ts — CONTRACT-codeintel-ui-api §4.5 findings and contract diff
 * Part of FE-CV-TASK-050-01; re-exported from code-intel-types.ts.
 */

import type { WithUnknown } from './code-intel-enum-fallback'
import type { SourceRef, SymbolRef } from './code-intel-index-types'
import type { OverlayScope, Owner } from './code-intel-overlay-types'

// ---------------------------------------------------------------------------
// §4.5  Findings and contract diff
// ---------------------------------------------------------------------------

export type FindingSeverity = WithUnknown<'error' | 'warning' | 'info'>

export type Finding = {
  findingKey: string
  rule: string
  kind: WithUnknown<
    'layer_violation' | 'dependency_cycle' | 'hotspot' | 'missing_tenant_id' | 'dead_code' | 'rls_removed'
  >
  severity: FindingSeverity
  titleKey: string
  params: Record<string, string>
  subject: string
  evidence: { path: string; line?: number; symbol?: SymbolRef }[]
  metrics: Record<string, number>
  owner?: Owner
  scope: { service?: string; componentId?: string; layer?: string }
  origin: WithUnknown<'introduced' | 'touched' | 'preexisting'>
  confidence: WithUnknown<'high' | 'medium' | 'low'>
  dismissed?: {
    by: string
    at: string
    reason: string
    disposition: 'ignored' | 'resolved'
    note?: string
  }
}

export type ContractCompatibility = WithUnknown<'breaking' | 'risky' | 'compatible'>

export type ContractChange = {
  id: string
  kind: WithUnknown<
    | 'proto-service'
    | 'proto-rpc'
    | 'proto-message'
    | 'proto-field'
    | 'proto-enum'
    | 'ws-channel'
    | 'ws-channel-arg'
    | 'route'
    | 'route-field'
    | 'sql-table'
    | 'sql-column'
  >
  name: string
  service?: string
  change: WithUnknown<'added' | 'removed' | 'modified'>
  compatibility: ContractCompatibility
  /** Unknown rule ids are shown verbatim. */
  ruleId: string
  details: Record<string, string>
  files: string[]
  consumers: {
    kind: WithUnknown<'rpc-client' | 'ws-channel' | 'route'>
    service?: string
    symbol?: SymbolRef
    path?: string
  }[]
  evidence: SourceRef[]
}

export type ContractDiff = {
  scope: OverlayScope
  summary: { breaking: number; risky: number; compatible: number; unknown: number }
  changes: ContractChange[]
  migrations: {
    service: string
    dialects: string[]
    files: string[]
    statements: {
      table: string
      op: string
      column?: string
      compatibility: string
      ruleId: string
      dialectOnly?: string
    }[]
    tables: {
      table: string
      service: string
      change: string
      accessors: SymbolRef[]
      columnsReferenced: string[]
      crossService?: boolean
      evidence: SourceRef[]
    }[]
    findings: Finding[]
  }[]
  truncated: boolean
  totalCount: number
}
