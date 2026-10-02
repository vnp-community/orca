// Public entry for MCP shared types (CONTRACT-mcp-ui-api.md §1/§2). Import from here only.
import type { McpEvent } from './mcp-entity-types'
import type { McpRpcMethod, McpRpcSchema } from './mcp-rpc-schema'

export type * from './mcp-entity-types'
export type * from './mcp-rpc-schema'

export const MCP_EVENT_TYPES = [
  'approval.requested',
  'approval.resolved',
  'grant.revoked',
  'session.closed',
  'killswitch.changed'
] as const satisfies readonly McpEvent['type'][]

// CONTRACT §2.3, in contract order (first 17 are the original set; the rest were added later).
export const MCP_ERROR_CODES = [
  'MCP_DISABLED',
  'MCP_NOT_ADMIN',
  'MCP_KILL_SWITCH_ACTIVE',
  'MCP_CONSENT_NOT_FOUND',
  'MCP_CONSENT_EXPIRED',
  'MCP_SCOPE_INVALID',
  'MCP_SCOPE_NOT_ALLOWED',
  'MCP_TOKEN_TOO_LONG',
  'MCP_TOKEN_LIMIT',
  'MCP_APPROVAL_EXPIRED',
  'MCP_APPROVAL_ALREADY_DECIDED',
  'MCP_APPROVAL_HASH_MISMATCH',
  'MCP_POLICY_HARD_DENY',
  'MCP_POLICY_VERSION_CONFLICT',
  'MCP_SERVER_SSRF_BLOCKED',
  'MCP_SERVER_NOT_APPROVED',
  'MCP_NOT_FOUND',
  'MCP_INVALID_ARGUMENT',
  'MCP_INTERNAL',
  'MCP_TIMEOUT',
  'MCP_UNAVAILABLE',
  'MCP_SERVER_INVALID',
  'MCP_SERVER_STDIO_NOT_ALLOWED',
  'MCP_SERVER_DIGEST_MISMATCH',
  'MCP_SERVER_NAME_CONFLICT',
  'MCP_PROMPT_INVALID',
  'MCP_PROMPT_NAME_CONFLICT',
  'MCP_PROMPT_VERSION_CONFLICT',
  'MCP_PROMPT_BUILTIN_READONLY'
] as const
export type McpErrorCode = (typeof MCP_ERROR_CODES)[number]

// Runtime list of the schema keys, CONTRACT §2 order (contract test in FE-MCP-SOL-012 diffs it).
export const MCP_RPC_METHODS = [
  'mcp.server.info',
  'mcp.session.list',
  'mcp.session.close',
  'mcp.consent.get',
  'mcp.consent.decide',
  'mcp.grant.list',
  'mcp.grant.revoke',
  'mcp.token.list',
  'mcp.token.create',
  'mcp.token.revoke',
  'mcp.approval.list',
  'mcp.approval.decide',
  'mcp.admin.settings.get',
  'mcp.admin.settings.set',
  'mcp.admin.killswitch.set',
  'mcp.admin.killswitch.list',
  'mcp.admin.tool.list',
  'mcp.admin.policy.list',
  'mcp.admin.policy.upsert',
  'mcp.admin.policy.delete',
  'mcp.admin.policy.explain',
  'mcp.admin.client.list',
  'mcp.admin.client.setStatus',
  'mcp.admin.grant.list',
  'mcp.admin.grant.revoke',
  'mcp.admin.session.list',
  'mcp.admin.audit.query',
  'mcp.admin.prompt.list',
  'mcp.admin.prompt.upsert',
  'mcp.admin.prompt.delete',
  'mcp.externalServer.list',
  'mcp.externalServer.upsert',
  'mcp.externalServer.setSecret',
  'mcp.externalServer.probe',
  'mcp.externalServer.review',
  'mcp.externalServer.delete'
] as const satisfies readonly McpRpcMethod[]
export const MCP_STREAM_METHODS = ['mcp.events.subscribe'] as const

export type McpRpcParams<M extends McpRpcMethod> = McpRpcSchema[M]['params']
export type McpRpcResult<M extends McpRpcMethod> = McpRpcSchema[M]['result']
export type McpRpcArgs<M extends McpRpcMethod> =
  McpRpcParams<M> extends void ? [] : [McpRpcParams<M>]
