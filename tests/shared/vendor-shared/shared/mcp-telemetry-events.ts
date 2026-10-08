// MCP UI telemetry schemas (FE-MCP-SOL-012). Coarse enums/buckets only: never a secret, token,
// scope id list, client name, URL, tool args or any entity id. Spread into `eventSchemas`.
import { z } from 'zod'

const tab = z.enum([
  'connect',
  'apps',
  'clients',
  'grants',
  'tokens',
  'approvals',
  'tools',
  'policy',
  'prompts',
  'audit',
  'servers'
])
const risk = z.enum(['read', 'write_reversible', 'exec', 'destructive', 'admin'])
const count = z.enum(['1', '2', '3+'])
const decision = z.enum(['approve', 'deny'])

export const mcpEventSchemas = {
  mcp_settings_opened: z.object({ tab, role: z.enum(['admin', 'user']) }).strict(),
  mcp_connect_snippet_copied: z
    .object({
      client: z.enum(['claude-code', 'claude-desktop', 'cursor']),
      mode: z.enum(['oauth', 'token'])
    })
    .strict(),
  mcp_session_closed: z.object({ outcome: z.enum(['success', 'error']) }).strict(),
  mcp_consent_decided: z
    .object({
      decision,
      scope_count: count,
      new_client: z.boolean(),
      via_dcr: z.boolean(),
      narrowed: z.boolean()
    })
    .strict(),
  mcp_token_created: z
    .object({ lifetime_bucket: z.enum(['<=7d', '<=30d', '<=90d', '>90d']), scope_count: count })
    .strict(),
  // Single optional discriminator: same convention as smart_to_recent_switch (empty objects are
  // awkward for the main-process validator).
  mcp_token_revoked: z.object({ _v: z.literal(1).optional() }).strict(),
  mcp_approval_decided: z
    .object({
      decision,
      risk,
      via: z.enum(['dialog', 'inbox', 'deeplink']),
      latency_bucket: z.enum(['<10s', '<60s', '<10m', '>=10m'])
    })
    .strict(),
  mcp_killswitch_toggled: z
    .object({ scope: z.enum(['tenant', 'client', 'grant', 'session']), active: z.boolean() })
    .strict()
} as const
