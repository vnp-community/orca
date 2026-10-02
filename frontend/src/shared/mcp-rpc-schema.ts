import type {
  McpApproval,
  McpAuditEntry,
  McpConsentRequest,
  McpDecision,
  McpExternalServer,
  McpExternalServerUpsertInput,
  McpGrant,
  McpOAuthClient,
  McpPrompt,
  McpRisk,
  McpScopeId,
  McpServerInfo,
  McpSessionView,
  McpToken,
  McpToolPolicy,
  McpToolView
} from './mcp-entity-types'

export type McpOk = { ok: true }

export type McpAdminSettings = {
  enabled: boolean
  dcrEnabled: boolean
  maxTokenDays: number
  approvalTtlSeconds: number
  killSwitch: McpServerInfo['killSwitch']
}

export type McpKillSwitchEntry = {
  scope: 'tenant' | 'client' | 'grant' | 'session'
  targetId?: string
  reason: string
  at: string
  by: string
}

// C10: every channel takes ONE object (or nothing) as args[0].
export type McpRpcSchema = {
  'mcp.server.info': { params: void; result: McpServerInfo }
  'mcp.session.list': { params: void; result: McpSessionView[] }
  'mcp.session.close': { params: { sessionId: string }; result: McpOk }
  'mcp.consent.get': {
    params: { requestId: string }
    result: McpConsentRequest
  }
  'mcp.consent.decide': {
    params: {
      requestId: string
      decision: 'approve' | 'deny'
      scopes: McpScopeId[]
    }
    result: { redirectUrl: string }
  }
  'mcp.grant.list': { params: void; result: McpGrant[] }
  'mcp.grant.revoke': { params: { grantId: string }; result: McpOk }
  'mcp.token.list': { params: void; result: McpToken[] }
  'mcp.token.create': {
    params: { name: string; scopes: McpScopeId[]; expiresInDays: number }
    result: { token: McpToken; secret: string }
  }
  'mcp.token.revoke': { params: { tokenId: string }; result: McpOk }
  'mcp.approval.list': {
    params: { status?: 'pending' | 'all'; cursor?: string; limit?: number }
    result: { approvals: McpApproval[]; nextCursor?: string }
  }
  'mcp.approval.decide': {
    params: {
      approvalId: string
      decision: 'approve' | 'deny'
      paramsHash: string
      note?: string
    }
    result: McpApproval
  }
  'mcp.admin.settings.get': { params: void; result: McpAdminSettings }
  'mcp.admin.settings.set': {
    params: Partial<Omit<McpAdminSettings, 'killSwitch'>>
    result: McpAdminSettings
  }
  'mcp.admin.killswitch.set': {
    params: {
      scope: 'tenant' | 'client' | 'grant' | 'session'
      targetId?: string
      active: boolean
      reason: string
    }
    result: McpOk
  }
  'mcp.admin.killswitch.list': { params: void; result: McpKillSwitchEntry[] }
  'mcp.admin.tool.list': {
    params: { namespace?: string; risk?: McpRisk }
    result: McpToolView[]
  }
  'mcp.admin.policy.list': { params: void; result: McpToolPolicy[] }
  'mcp.admin.policy.upsert': {
    params: Omit<McpToolPolicy, 'id' | 'version' | 'updatedAt' | 'updatedBy'> &
      Partial<Pick<McpToolPolicy, 'id' | 'version'>>
    result: McpToolPolicy
  }
  'mcp.admin.policy.delete': { params: { policyId: string }; result: McpOk }
  'mcp.admin.policy.explain': {
    params: { tool: string; userId?: string; clientId?: string }
    result: { decision: McpDecision; reasons: string[] }
  }
  'mcp.admin.client.list': { params: void; result: McpOAuthClient[] }
  'mcp.admin.client.setStatus': {
    params: { clientId: string; status: 'allowed' | 'blocked' }
    result: McpOAuthClient
  }
  'mcp.admin.grant.list': { params: { userId?: string }; result: McpGrant[] }
  'mcp.admin.grant.revoke': { params: { grantId: string }; result: McpOk }
  'mcp.admin.session.list': { params: void; result: McpSessionView[] }
  'mcp.admin.audit.query': {
    params: {
      from?: string
      to?: string
      userId?: string
      tool?: string
      decision?: McpAuditEntry['decision']
      cursor?: string
      limit?: number
    }
    result: { entries: McpAuditEntry[]; nextCursor?: string }
  }
  'mcp.admin.prompt.list': { params: void; result: McpPrompt[] }
  'mcp.admin.prompt.upsert': { params: McpPrompt; result: McpPrompt }
  'mcp.admin.prompt.delete': { params: { promptId: string }; result: McpOk }
  'mcp.externalServer.list': {
    params: { scope?: McpExternalServer['scope'] }
    result: McpExternalServer[]
  }
  'mcp.externalServer.upsert': {
    params: Partial<McpExternalServer> | McpExternalServerUpsertInput
    result: McpExternalServer
  }
  'mcp.externalServer.setSecret': {
    params: {
      serverId: string
      kind: 'env' | 'header'
      name: string
      value: string
    }
    result: { hasSecret: true }
  }
  'mcp.externalServer.probe': {
    params: { serverId: string }
    result: {
      transport: McpExternalServer['transport']
      tools: { name: string; description: string }[]
      digest: string
      approvedTools?: { name: string; description: string }[]
    }
  }
  'mcp.externalServer.review': {
    params: {
      serverId: string
      decision: 'approve' | 'reject'
      toolsDigest: string
    }
    result: McpExternalServer
  }
  'mcp.externalServer.delete': { params: { serverId: string }; result: McpOk }
}
export type McpRpcMethod = keyof McpRpcSchema
