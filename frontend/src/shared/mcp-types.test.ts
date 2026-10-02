import { describe, expect, it } from 'vitest'
import {
  MCP_ERROR_CODES,
  MCP_EVENT_TYPES,
  MCP_RPC_METHODS,
  MCP_STREAM_METHODS,
  type McpServerInfo
} from './mcp-types'

describe('mcp-types', () => {
  it('keeps the original 17 contract error codes first, then the additive ones', () => {
    expect(MCP_ERROR_CODES.slice(0, 17)).toEqual([
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
      'MCP_NOT_FOUND'
    ])
    expect(new Set(MCP_ERROR_CODES).size).toBe(MCP_ERROR_CODES.length)
  })

  it('lists unique mcp.* rpc methods plus the stream channel', () => {
    expect(new Set(MCP_RPC_METHODS).size).toBe(MCP_RPC_METHODS.length)
    expect(MCP_RPC_METHODS.every((m) => m.startsWith('mcp.'))).toBe(true)
    expect(MCP_STREAM_METHODS).toEqual(['mcp.events.subscribe'])
    expect(MCP_EVENT_TYPES).toHaveLength(5)
  })

  it('compiles a server info fixture', () => {
    const info: McpServerInfo = {
      enabled: true,
      tenantEnabled: true,
      resourceUrl: 'https://x/mcp',
      protocolVersions: ['2025-06-18'],
      authorizationServer: 'https://x',
      scopesSupported: [{ id: 'orca:read', label: 'Read', description: 'd', risk: 'read' }],
      dcrEnabled: true,
      maxTokenDays: 90,
      killSwitch: { active: false }
    }
    expect(info.enabled).toBe(true)
  })
})
