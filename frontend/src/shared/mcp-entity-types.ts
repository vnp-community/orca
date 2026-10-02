// CONTRACT-mcp-ui-api.md §1/§2 mirror — the contract is the source of truth; change it first.
// Only additive changes (C9).
// Split from mcp-types.ts for the 300-line .ts limit; import from './mcp-types'.

export type McpRisk = 'read' | 'write_reversible' | 'exec' | 'destructive' | 'admin'
export type McpScopeId = 'orca:read' | 'orca:write' | 'orca:exec' | 'orca:admin'
export type McpDecision = 'allow' | 'require_approval' | 'deny'

export type McpScopeDescriptor = {
  id: McpScopeId | string
  label: string
  description: string
  risk: McpRisk
}

export type McpServerInfo = {
  enabled: boolean
  tenantEnabled?: boolean
  resourceUrl: string
  protocolVersions: string[]
  authorizationServer: string
  scopesSupported: McpScopeDescriptor[]
  dcrEnabled: boolean
  maxTokenDays: number
  killSwitch: { active: boolean; reason?: string; at?: string }
}

export type McpGrant = {
  id: string
  clientId: string
  clientName: string
  clientUri?: string
  scopes: McpScopeId[]
  createdAt: string
  lastUsedAt?: string
  status: 'active' | 'revoked'
  userId?: string
  userName?: string
}

export type McpSessionView = {
  id: string
  clientName: string
  grantId?: string
  tokenId?: string
  createdAt: string
  lastSeenAt: string
  protocolVersion: string
  activeStreams: number
  toolCalls: number
  userId?: string
  userName?: string
}

export type McpConsentRequest = {
  requestId: string
  clientId: string
  clientName: string
  clientUri?: string
  redirectHost: string
  scopes: McpScopeDescriptor[]
  alreadyGranted: McpScopeId[]
  tenant: { id: string; name: string }
  isNewClient: boolean
  registeredViaDcr: boolean
  expiresAt: string
}

export type McpToken = {
  id: string
  name: string
  scopes: McpScopeId[]
  createdAt: string
  expiresAt: string
  lastUsedAt?: string
  status: 'active' | 'revoked' | 'expired'
  userId?: string
  userName?: string
}

export type McpApproval = {
  id: string
  createdAt: string
  expiresAt: string
  status: 'pending' | 'approved' | 'denied' | 'expired' | 'cancelled'
  tool: { name: string; title: string; risk: McpRisk }
  clientName: string
  sessionId: string
  argsPreview: { text: string; redacted: boolean }
  paramsHash: string
  decidedAt?: string
  decidedVia?: 'web' | 'mobile' | 'elicitation'
  reasons?: string[]
}

export type McpToolView = {
  name: string
  channel: string
  title: string
  description: string
  namespace: string
  risk: McpRisk
  requiredScope: McpScopeId
  pack: 1 | 2 | 3 | 4
  hardDenied: boolean
  effective: McpDecision
  effectiveSource: 'default' | 'tenant_policy' | 'hard_deny' | 'kill_switch'
  annotations: {
    readOnly: boolean
    destructive: boolean
    idempotent: boolean
    openWorld: boolean
  }
}

export type McpToolPolicy = {
  id: string
  version: number
  updatedAt: string
  updatedBy: string
  match: {
    tool?: string
    namespace?: string
    risk?: McpRisk
    clientId?: string
    roles?: ('admin' | 'user')[]
  }
  decision: McpDecision
  note?: string
}

export type McpAuditEntry = {
  id: string
  at: string
  actorType: 'agent'
  userId: string
  userName?: string
  clientName: string
  sessionId: string
  tool: string
  risk: McpRisk
  decision: 'allow' | 'deny' | 'approved' | 'denied' | 'expired'
  argsSummary: string
  result: 'ok' | 'error' | null
  durationMs?: number
  approver?: string
  traceId?: string
}

export type McpOAuthClient = {
  clientId: string
  name: string
  redirectUris: string[]
  registeredVia: 'dcr' | 'admin'
  status: 'allowed' | 'blocked' | 'pending'
  createdAt: string
  lastUsedAt?: string
  activeGrants: number
}

/** CONTRACT §5: present on terminal/agent results only when an MCP client created them. */
export type McpOrigin = {
  type: 'mcp'
  clientName: string
  mcpSessionId: string
  userId: string
}

export type McpPrompt = {
  id: string
  name: string
  description: string
  version: number
  updatedAt: string
  arguments: { name: string; description: string; required: boolean }[]
  template: string
  builtin: boolean
}

export type McpExternalServer = {
  id: string
  scope: 'tenant' | 'team' | 'user'
  scopeId?: string
  name: string
  transport: 'http' | 'stdio'
  url?: string
  command?: string
  args?: string[]
  envRefs: { name: string; hasSecret: boolean }[]
  headerRefs: { name: string; hasSecret: boolean }[]
  status: 'pending_review' | 'approved' | 'disabled'
  toolsDigest?: string
  toolsChanged: boolean
  health?: { ok: boolean; checkedAt: string; error?: string }
  createdBy: string
  reviewedBy?: string
  updatedAt?: string
}

/** Upsert body: refs carry names only (hasSecret/status/digest are server-computed, CONTRACT). */
export type McpExternalServerUpsertInput = Omit<
  Partial<McpExternalServer>,
  | 'envRefs'
  | 'headerRefs'
  | 'status'
  | 'toolsDigest'
  | 'toolsChanged'
  | 'health'
  | 'createdBy'
  | 'reviewedBy'
  | 'updatedAt'
> & { envRefs?: { name: string }[]; headerRefs?: { name: string }[] }

export type McpEvent =
  | { type: 'approval.requested'; approval: McpApproval }
  | { type: 'approval.resolved'; id: string; status: McpApproval['status'] }
  | { type: 'grant.revoked'; grantId: string }
  | { type: 'session.closed'; sessionId: string }
  | { type: 'killswitch.changed'; active: boolean; reason?: string }
