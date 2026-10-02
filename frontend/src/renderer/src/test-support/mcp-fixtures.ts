// Tests-only builders for CONTRACT §1 entities. Never import from production code.
import type {
  McpApproval,
  McpConsentRequest,
  McpGrant,
  McpServerInfo,
  McpSessionView,
  McpToken
} from '../../../shared/mcp-types'

export const MCP_TEST_SCOPES: McpServerInfo['scopesSupported'] = [
  { id: 'orca:read', label: 'Read', description: 'Read data', risk: 'read' },
  { id: 'orca:write', label: 'Write', description: 'Change data', risk: 'write_reversible' },
  { id: 'orca:exec', label: 'Run', description: 'Run commands', risk: 'exec' },
  { id: 'orca:admin', label: 'Administer', description: 'Admin things', risk: 'admin' }
]

export const makeMcpServerInfo = (o: Partial<McpServerInfo> = {}): McpServerInfo => ({
  enabled: true,
  resourceUrl: 'https://orca.example.com/mcp',
  protocolVersions: ['2025-06-18'],
  authorizationServer: 'https://orca.example.com',
  scopesSupported: MCP_TEST_SCOPES,
  dcrEnabled: true,
  maxTokenDays: 90,
  killSwitch: { active: false },
  ...o
})

export const makeMcpToken = (o: Partial<McpToken> = {}): McpToken => ({
  id: 'tok-1',
  name: 'ci',
  scopes: ['orca:read'],
  createdAt: '2026-10-01T00:00:00Z',
  expiresAt: '2030-10-01T00:00:00Z',
  status: 'active',
  ...o
})

export const makeMcpApproval = (o: Partial<McpApproval> = {}): McpApproval => ({
  id: 'ap-1',
  createdAt: new Date().toISOString(),
  expiresAt: new Date(Date.now() + 120_000).toISOString(),
  status: 'pending',
  tool: { name: 'terminal_send', title: 'Send to terminal', risk: 'exec' },
  clientName: 'Claude Code',
  sessionId: 'session-abcdef',
  argsPreview: { text: 'ls -la /tmp', redacted: false },
  paramsHash: 'hash-ap-1',
  ...o
})

export const makeMcpConsentRequest = (o: Partial<McpConsentRequest> = {}): McpConsentRequest => ({
  requestId: '0b2f6c1e-1111-4222-8333-444455556666',
  clientId: 'client-1',
  clientName: 'Claude Code',
  redirectHost: 'localhost:33418',
  scopes: MCP_TEST_SCOPES.slice(0, 3),
  alreadyGranted: [],
  tenant: { id: 't1', name: 'Acme' },
  isNewClient: true,
  registeredViaDcr: true,
  expiresAt: new Date(Date.now() + 600_000).toISOString(),
  ...o
})

export const makeMcpGrant = (o: Partial<McpGrant> = {}): McpGrant => ({
  id: 'grant-1',
  clientId: 'client-1',
  clientName: 'Claude Code',
  scopes: ['orca:read'],
  createdAt: '2026-10-01T00:00:00Z',
  status: 'active',
  ...o
})

export const makeMcpSession = (o: Partial<McpSessionView> = {}): McpSessionView => ({
  id: 'sess-1',
  clientName: 'Claude Code',
  createdAt: '2026-10-01T00:00:00Z',
  lastSeenAt: '2026-10-01T00:05:00Z',
  protocolVersion: '2025-06-18',
  activeStreams: 1,
  toolCalls: 3,
  ...o
})
