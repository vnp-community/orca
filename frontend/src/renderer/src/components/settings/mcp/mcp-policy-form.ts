import type { McpRisk, McpToolPolicy, McpToolView } from '../../../../../shared/mcp-types'

export type McpPolicyMatch = McpToolPolicy['match']
export type McpPolicyDraft = Pick<McpToolPolicy, 'match' | 'decision' | 'note'> &
  Partial<Pick<McpToolPolicy, 'id' | 'version'>>

// Server floor: exec/destructive can only be allowed by a rule naming one exact tool.
export const EXACT_TOOL_REQUIRED_RISKS: ReadonlySet<McpRisk> = new Set(['exec', 'destructive'])

export const MCP_POLICY_NOTE_MAX = 500

export type McpPolicyIssue =
  | { kind: 'no_dimension' }
  | { kind: 'hard_deny'; tools: string[] }
  | { kind: 'exact_tool_required'; risks: McpRisk[] }
  | { kind: 'admin_still_needs_admin_role' }

/** Mirrors how the server narrows a match (client/roles do not narrow the tool set). */
export function expandMatch(match: McpPolicyMatch, tools: McpToolView[]): McpToolView[] {
  return tools.filter(
    (t) =>
      (!match.tool || t.name === match.tool) &&
      (!match.namespace || t.namespace === match.namespace) &&
      (!match.risk || t.risk === match.risk)
  )
}

export function hasMatchDimension(match: McpPolicyMatch): boolean {
  return Boolean(
    match.tool || match.namespace || match.risk || match.clientId || match.roles?.length
  )
}

// Client checks only warn early; the server stays the source of truth.
export function validatePolicyDraft(
  d: McpPolicyDraft,
  tools: McpToolView[]
): { errors: McpPolicyIssue[]; warnings: McpPolicyIssue[]; matched: McpToolView[] } {
  const matched = expandMatch(d.match, tools)
  const errors: McpPolicyIssue[] = []
  const warnings: McpPolicyIssue[] = []
  if (!hasMatchDimension(d.match)) {
    errors.push({ kind: 'no_dimension' })
  }
  if (d.decision !== 'deny') {
    const denied = matched.filter((t) => t.hardDenied).map((t) => t.name)
    if (denied.length > 0) {
      errors.push({ kind: 'hard_deny', tools: denied })
    }
  }
  if (d.decision === 'allow' && !d.match.tool) {
    const risks = [
      ...new Set(matched.filter((t) => EXACT_TOOL_REQUIRED_RISKS.has(t.risk)).map((t) => t.risk))
    ]
    if (risks.length > 0) {
      errors.push({ kind: 'exact_tool_required', risks })
    }
  }
  if (d.decision === 'allow' && matched.some((t) => t.risk === 'admin')) {
    warnings.push({ kind: 'admin_still_needs_admin_role' })
  }
  return { errors, warnings, matched }
}

/** Drops empty dimensions: a JSON null/'' would make the server's policy engine misbehave. */
export function buildUpsertPayload(draft: McpPolicyDraft): McpPolicyDraft {
  const match: McpPolicyMatch = {}
  if (draft.match.tool?.trim()) {
    match.tool = draft.match.tool.trim()
  }
  if (draft.match.namespace) {
    match.namespace = draft.match.namespace
  }
  if (draft.match.risk) {
    match.risk = draft.match.risk
  }
  if (draft.match.clientId) {
    match.clientId = draft.match.clientId
  }
  if (draft.match.roles && draft.match.roles.length > 0) {
    match.roles = [...draft.match.roles]
  }
  const note = draft.note?.trim()
  return {
    ...(draft.id ? { id: draft.id } : {}),
    ...(draft.id && draft.version !== undefined ? { version: draft.version } : {}),
    match,
    decision: draft.decision,
    ...(note ? { note } : {})
  }
}
