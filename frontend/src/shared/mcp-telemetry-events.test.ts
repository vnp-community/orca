import { describe, expect, it } from 'vitest'
import { eventSchemas } from './telemetry-events'
import { mcpEventSchemas } from './mcp-telemetry-events'

const VALID: Record<keyof typeof mcpEventSchemas, unknown> = {
  mcp_settings_opened: { tab: 'tokens', role: 'user' },
  mcp_connect_snippet_copied: { client: 'cursor', mode: 'token' },
  mcp_session_closed: { outcome: 'success' },
  mcp_consent_decided: {
    decision: 'approve',
    scope_count: '2',
    new_client: true,
    via_dcr: true,
    narrowed: false
  },
  mcp_token_created: { lifetime_bucket: '<=30d', scope_count: '1' },
  mcp_token_revoked: {},
  mcp_approval_decided: {
    decision: 'deny',
    risk: 'exec',
    via: 'dialog',
    latency_bucket: '<10s'
  },
  mcp_killswitch_toggled: { scope: 'tenant', active: true }
}

describe('MCP telemetry schemas', () => {
  it('are registered in the shared eventSchemas registry', () => {
    for (const name of Object.keys(mcpEventSchemas)) {
      expect(eventSchemas).toHaveProperty(name)
    }
  })

  it.each(Object.entries(VALID))('%s accepts a valid payload', (name, payload) => {
    const schema = eventSchemas[name as keyof typeof VALID]
    expect(schema.safeParse(payload).success).toBe(true)
  })

  it.each(Object.entries(VALID))('%s rejects secrets, ids, names and previews', (name, payload) => {
    const schema = eventSchemas[name as keyof typeof VALID]
    for (const extra of [
      'token',
      'secret',
      'client_name',
      'redirect_url',
      'argsPreview',
      'approval_id'
    ]) {
      expect(schema.safeParse({ ...(payload as object), [extra]: 'x' }).success).toBe(false)
    }
  })

  it('rejects free-form values in enum fields', () => {
    expect(
      mcpEventSchemas.mcp_connect_snippet_copied.safeParse({ client: 'My Client', mode: 'oauth' })
        .success
    ).toBe(false)
    expect(
      mcpEventSchemas.mcp_token_created.safeParse({ lifetime_bucket: '45', scope_count: '1' })
        .success
    ).toBe(false)
  })

  it('declares no string-typed (free text) property anywhere', () => {
    for (const schema of Object.values(mcpEventSchemas)) {
      for (const field of Object.values(schema.shape) as { def: { type: string } }[]) {
        expect(['enum', 'boolean', 'literal', 'optional']).toContain(field.def.type)
      }
    }
  })
})
