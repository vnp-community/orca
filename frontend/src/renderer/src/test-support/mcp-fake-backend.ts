// Tests-only in-memory MCP backend speaking the CONTRACT-mcp-ui-api.md WS channels through the
// same `window.api.mcp` bridge the real hooks use. Unimplemented channels fail like the gateway.
import type { McpBridgeApi } from '../../../preload/api-types'
import type {
  McpApproval,
  McpConsentRequest,
  McpErrorCode,
  McpEvent,
  McpGrant,
  McpRpcMethod,
  McpRpcParams,
  McpRpcResult,
  McpServerInfo,
  McpSessionView,
  McpToken
} from '../../../shared/mcp-types'
import { makeMcpServerInfo } from './mcp-fixtures'

type Handler<M extends McpRpcMethod> = (p: McpRpcParams<M>) => McpRpcResult<M>
type Handlers = { [M in McpRpcMethod]?: Handler<M> }
export type FakeToolCall = { approvalId: string; outcome: Promise<{ isError: boolean }> }

const OK = { ok: true } as const
export const mcpError = (code: McpErrorCode, msg = 'failed'): Error => new Error(`${code}: ${msg}`)

export function createFakeMcpBackend(init: { role?: 'admin' | 'user' } = {}) {
  let role: 'admin' | 'user' = init.role ?? 'user'
  let info: McpServerInfo = makeMcpServerInfo()
  let seq = 0
  const nextId = (p: string): string => `${p}-${++seq}`
  const tokens: McpToken[] = []
  const secrets = new Map<string, string>() // tokenId -> secret; test-only, never exposed via RPC
  const grants: McpGrant[] = []
  const sessions: McpSessionView[] = []
  const consents = new Map<string, McpConsentRequest>()
  const approvals = new Map<string, McpApproval>()
  const waiters = new Map<string, (r: { isError: boolean }) => void>()
  const decisions: { requestId: string; decision: string; scopes: string[] }[] = []
  const calls: { method: string; params: unknown }[] = []
  const failures = new Map<string, Error[]>()
  const listeners = new Set<{ onEvent: (e: McpEvent) => void; onClose?: () => void }>()
  let redirectUrl = 'http://localhost:33418/callback?code=abc&state=xyz'

  const emit = (e: McpEvent): void => {
    for (const l of Array.from(listeners)) {
      l.onEvent(e)
    }
  }
  const requireAdmin = (): void => {
    if (role !== 'admin') {
      throw mcpError('MCP_NOT_ADMIN', 'administrator role required')
    }
  }
  const assertWritable = (): void => {
    if (info.killSwitch.active) {
      throw mcpError('MCP_KILL_SWITCH_ACTIVE', 'paused')
    }
  }
  const setKillSwitch = (active: boolean, reason?: string): void => {
    info = { ...info, killSwitch: active ? { active, reason } : { active } }
    emit({ type: 'killswitch.changed', active, reason })
  }

  const handlers: Handlers = {
    'mcp.server.info': () => {
      if (!info.enabled) {
        throw mcpError('MCP_DISABLED', 'MCP is turned off')
      }
      return info
    },
    'mcp.session.list': () => sessions.slice(),
    'mcp.session.close': ({ sessionId }) => {
      sessions.splice(0, sessions.length, ...sessions.filter((s) => s.id !== sessionId))
      emit({ type: 'session.closed', sessionId })
      return OK
    },
    'mcp.consent.get': ({ requestId }) => {
      const c = consents.get(requestId)
      if (!c) {
        throw mcpError('MCP_CONSENT_NOT_FOUND', 'unknown request')
      }
      if (Date.parse(c.expiresAt) < Date.now()) {
        throw mcpError('MCP_CONSENT_EXPIRED', 'expired')
      }
      return c
    },
    'mcp.consent.decide': ({ requestId, decision, scopes }) => {
      const c = consents.get(requestId)
      if (!c) {
        throw mcpError('MCP_CONSENT_NOT_FOUND', 'unknown request')
      }
      if (scopes.some((s) => !c.scopes.some((d) => d.id === s))) {
        throw mcpError('MCP_SCOPE_INVALID', 'scope not requested')
      }
      decisions.push({ requestId, decision, scopes })
      consents.delete(requestId)
      return { redirectUrl }
    },
    'mcp.grant.list': () => grants.slice(),
    'mcp.grant.revoke': ({ grantId }) => {
      const g = grants.find((x) => x.id === grantId)
      if (!g) {
        throw mcpError('MCP_NOT_FOUND', 'no such grant')
      }
      g.status = 'revoked'
      emit({ type: 'grant.revoked', grantId })
      return OK
    },
    'mcp.token.list': () => tokens.map((t) => ({ ...t })),
    'mcp.token.create': ({ name, scopes, expiresInDays }) => {
      assertWritable()
      if (expiresInDays > info.maxTokenDays) {
        throw mcpError('MCP_TOKEN_TOO_LONG', `max ${info.maxTokenDays} days`)
      }
      if (role !== 'admin' && scopes.includes('orca:admin')) {
        throw mcpError('MCP_SCOPE_NOT_ALLOWED', 'role cannot grant scope')
      }
      const id = nextId('tok')
      const now = Date.now()
      const token: McpToken = {
        id,
        name,
        scopes,
        createdAt: new Date(now).toISOString(),
        expiresAt: new Date(now + expiresInDays * 86_400_000).toISOString(),
        status: 'active'
      }
      tokens.unshift(token)
      const secret = `orca_pat_${Math.random().toString(36).slice(2)}${Math.random().toString(36).slice(2)}`
      secrets.set(id, secret)
      return { token: { ...token }, secret }
    },
    'mcp.token.revoke': ({ tokenId }) => {
      const t = tokens.find((x) => x.id === tokenId)
      if (!t) {
        throw mcpError('MCP_NOT_FOUND', 'no such token')
      }
      t.status = 'revoked'
      return OK
    },
    'mcp.approval.list': ({ status } = {}) => ({
      approvals: [...approvals.values()].filter(
        (a) => status !== 'pending' || a.status === 'pending'
      )
    }),
    'mcp.approval.decide': ({ approvalId, decision, paramsHash }) => {
      const a = approvals.get(approvalId)
      if (!a) {
        throw mcpError('MCP_NOT_FOUND', 'no such approval')
      }
      assertWritable()
      if (a.status !== 'pending') {
        throw mcpError('MCP_APPROVAL_ALREADY_DECIDED', 'done')
      }
      if (Date.parse(a.expiresAt) < Date.now()) {
        throw mcpError('MCP_APPROVAL_EXPIRED', 'expired')
      }
      if (paramsHash !== a.paramsHash) {
        throw mcpError('MCP_APPROVAL_HASH_MISMATCH', 'params changed')
      }
      a.status = decision === 'approve' ? 'approved' : 'denied'
      waiters.get(a.id)?.({ isError: decision !== 'approve' })
      emit({ type: 'approval.resolved', id: a.id, status: a.status })
      return { ...a }
    },
    'mcp.admin.settings.get': () => {
      requireAdmin()
      return {
        enabled: info.enabled,
        dcrEnabled: info.dcrEnabled,
        maxTokenDays: info.maxTokenDays,
        approvalTtlSeconds: 300,
        killSwitch: info.killSwitch
      }
    },
    'mcp.admin.killswitch.set': ({ scope, active, reason }) => {
      requireAdmin()
      if (scope === 'tenant') {
        setKillSwitch(active, reason)
      }
      return OK
    },
    'mcp.admin.killswitch.list': () => {
      requireAdmin()
      return info.killSwitch.active
        ? [{ scope: 'tenant', reason: info.killSwitch.reason ?? '', at: '', by: 'admin' }]
        : []
    },
    'mcp.admin.session.list': () => {
      requireAdmin()
      return sessions.slice()
    },
    'mcp.admin.grant.list': () => {
      requireAdmin()
      return grants.slice()
    }
  }

  const bridge: McpBridgeApi = {
    call: async (method, ...params) => {
      calls.push({ method, params: params[0] })
      const queued = failures.get(method)?.shift()
      if (queued) {
        throw queued
      }
      const h = handlers[method] as ((p: unknown) => unknown) | undefined
      if (!h) {
        throw new Error(`channel "${method}" is not yet implemented in backend-go`)
      }
      return h(params[0] ?? {}) as never
    },
    subscribeEvents: (onEvent, onClose) => {
      const l = { onEvent, onClose }
      listeners.add(l)
      return () => void listeners.delete(l)
    }
  }

  return {
    bridge,
    calls,
    decisions,
    handlerChannels: Object.keys(handlers),
    emit,
    setKillSwitch,
    setRole: (r: 'admin' | 'user') => void (role = r),
    setInfo: (patch: Partial<McpServerInfo>) => void (info = { ...info, ...patch }),
    setRedirectUrl: (u: string) => void (redirectUrl = u),
    failNext: (method: McpRpcMethod, e: Error) =>
      void failures.set(method, [...(failures.get(method) ?? []), e]),
    addConsent: (c: McpConsentRequest) => void consents.set(c.requestId, c),
    addGrant: (g: McpGrant) => void grants.push(g),
    addSession: (s: McpSessionView) => void sessions.push(s),
    streamCount: () => listeners.size,
    closeStreams: () => {
      for (const l of Array.from(listeners)) {
        listeners.delete(l)
        l.onClose?.()
      }
    },
    /** Test-only: would an MCP client presenting this PAT still be accepted by the server? */
    patAccepted: (secret: string): boolean => {
      const id = [...secrets].find(([, s]) => s === secret)?.[0]
      return tokens.find((t) => t.id === id)?.status === 'active'
    },
    allSecrets: (): string[] => [...secrets.values()],
    /** An agent's tools/call that needs approval: pushes the event and parks until decided. */
    requestToolCall: (a: McpApproval): FakeToolCall => {
      approvals.set(a.id, { ...a }) // own copy: the UI must not alias server state
      const outcome = new Promise<{ isError: boolean }>((r) => waiters.set(a.id, r))
      emit({ type: 'approval.requested', approval: a })
      return { approvalId: a.id, outcome }
    },
    /** Server-side change after the user saw the prompt (forces HASH_MISMATCH on decide). */
    mutateApprovalHash: (id: string, hash: string): void => {
      const a = approvals.get(id)
      if (a) {
        a.paramsHash = hash
      }
    },
    install: (): (() => void) => {
      const w = window as unknown as { api?: Record<string, unknown> }
      const prev = w.api
      w.api = { ...prev, mcp: bridge }
      return () => void (w.api = prev)
    }
  }
}

export type FakeMcpBackend = ReturnType<typeof createFakeMcpBackend>
