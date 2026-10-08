// Tests-only in-memory code-intel backend speaking CONTRACT-codeintel-ui-api.md (46 channels).
// Single fake for unit, integration and Playwright (web) tests; channels without data or a handler
// fail like the gateway ("is not yet implemented"), never with silent empty success.
import type { CodeIntelBridgeDeps, CodeIntelRawEnvelope } from '../../../shared/code-intel-bridge'
import { CODE_INTEL_ERROR_CODES } from '../../../shared/code-intel-error-codes'
import {
  CODE_INTEL_ENVELOPE_METHODS,
  CODE_INTEL_RPC_METHODS,
  getMethodMaxArgsBytes
} from '../../../shared/code-intel-rpc-methods'
import type { CodeIntelSettings, IndexStatus, ReviewState } from '../../../shared/code-intel-types'
import { defaultFakeSettings as defaultSettings, withDerivedEffective } from './code-intel-fake-settings'
import { INDEX_STATUS_FIXTURES, REVIEW_STATE_INITIAL } from './code-intel-fixtures'

const M = CODE_INTEL_RPC_METHODS
const E = CODE_INTEL_ERROR_CODES

export const ALL_CODE_INTEL_CHANNELS: readonly string[] = Object.values(M)

// Why: contract U3 forbids identity/command parameters; the gateway rejects them as invalid params.
const FORBIDDEN_KEYS = [
  'tenantId',
  'userId',
  'deviceId',
  'role',
  'devServerId',
  'workspaceRoot',
  'repo',
  'args',
  'command',
  'cypher'
]
const SETTINGS_ONLY = new Set<string>([M.SETTINGS_GET, M.SETTINGS_SET])
const NO_SELECTOR = new Set<string>([M.SETTINGS_GET, M.SETTINGS_SET, M.SUBSCRIBE])
const SETTINGS_FIELDS = [
  'codeIntelEnabled',
  'qualityGateEnabled',
  'qualitySecurityScanEnabled',
  'indexPolicy',
  'aiReviewLevel',
  'aiReviewModel',
  'agentTurnStorePromptExcerpt',
  'agentClaimTextEnabled',
  'hotspotWindowDays'
]
const ADMIN_ONLY = new Set<string>([M.SETTINGS_SET])
const AI_CHANNELS = new Set<string>([M.QUALITY_SUMMARY])

export type FakeSettingsPatch = {
  codeIntelEnabled?: boolean
  qualityGateEnabled?: boolean
  qualitySecurityScanEnabled?: boolean
  aiReviewEnabled?: boolean
}

export type FakeChannelHandler = (params: Record<string, unknown>) => unknown | Promise<unknown>

type StreamListener = { onEvent: (frame: unknown) => void; onClose: () => void }

export const codeIntelError = (code: string, msg = 'failed', data?: unknown): Error =>
  new Error(`${code}: ${msg}${data === undefined ? '' : ` | ${JSON.stringify(data)}`}`)

export function createFakeCodeIntelBackend(init: { role?: 'admin' | 'user' } = {}) {
  let role: 'admin' | 'user' = init.role ?? 'user'
  let settings = defaultSettings()
  let index: IndexStatus = INDEX_STATUS_FIXTURES.ready
  let reviewState: ReviewState = { ...REVIEW_STATE_INITIAL }
  const data = new Map<string, unknown>()
  const overrides = new Map<string, FakeChannelHandler>()
  const failures = new Map<string, Error[]>()
  const calls: { method: string; params: unknown }[] = []
  const listeners = new Set<StreamListener>()
  let etagSeq = 0

  const setEffective = (): void => {
    settings = withDerivedEffective(settings)
  }

  const emit = (frame: Record<string, unknown>): void => {
    const full = { occurredAt: new Date().toISOString(), ...frame }
    for (const l of Array.from(listeners)) {
      l.onEvent(full)
    }
  }

  const envelope = (method: string, params: Record<string, unknown>, payload: unknown): unknown => ({
    worktreeId: String(params.worktreeId ?? ''),
    view: method.replace('codeIntel.', ''),
    sources: [{ tool: 'gitnexus', version: '1.0.0', indexedAt: null, commit: null }],
    headCommit: null,
    stale: false,
    truncated: false,
    totalCount: Array.isArray(payload) ? payload.length : 0,
    etag: `etag-${++etagSeq}`,
    fromCache: false,
    generatedAt: new Date().toISOString(),
    data: payload
  })

  function validate(method: string, args: unknown[]): Record<string, unknown> {
    if (args.length > 1) {
      throw codeIntelError(E.INVALID_PARAMS, 'exactly one params object is allowed')
    }
    const params = args[0] ?? {}
    if (typeof params !== 'object' || params === null || Array.isArray(params)) {
      throw codeIntelError(E.INVALID_PARAMS, 'params must be an object')
    }
    const p = params as Record<string, unknown>
    const forbidden = FORBIDDEN_KEYS.find((k) => k in p)
    if (forbidden) {
      throw codeIntelError(E.INVALID_PARAMS, `unknown field ${forbidden}`)
    }
    if (new TextEncoder().encode(JSON.stringify(p)).length > getMethodMaxArgsBytes(method)) {
      throw codeIntelError(E.PAYLOAD_TOO_LARGE, 'params too large')
    }
    if (method === M.SETTINGS_GET || method === M.SUBSCRIBE) {
      return p
    }
    if (method === M.SETTINGS_SET) {
      const keys = Object.keys(p)
      if (keys.length === 0 || keys.some((k) => !SETTINGS_FIELDS.includes(k))) {
        throw codeIntelError(E.INVALID_PARAMS, 'settings.set needs known fields only')
      }
      return p
    }
    if (!NO_SELECTOR.has(method) && (typeof p.projectId !== 'string' || typeof p.worktreeId !== 'string')) {
      throw codeIntelError(E.INVALID_PARAMS, 'projectId and worktreeId are required')
    }
    return p
  }

  function gate(method: string): void {
    if (SETTINGS_ONLY.has(method)) {
      return
    }
    const eff = settings.effective
    if (!eff.codeIntelEnabled) {
      throw codeIntelError(E.DISABLED, 'code intelligence is turned off')
    }
    if (method.startsWith('codeIntel.quality.') && !eff.qualityGateEnabled) {
      throw codeIntelError(E.QUALITY_GATE_DISABLED, 'quality gate is turned off')
    }
    if (AI_CHANNELS.has(method) && !eff.aiReviewEnabled) {
      throw codeIntelError(E.AI_REVIEW_DISABLED, 'AI review is turned off')
    }
  }

  const builtIn: Record<string, FakeChannelHandler> = {
    [M.SETTINGS_GET]: () => settings,
    [M.SETTINGS_SET]: (p) => {
      settings = { ...settings, tenant: { ...settings.tenant, ...p } as CodeIntelSettings['tenant'] }
      setEffective()
      return settings
    },
    [M.STATUS]: () => index,
    [M.BIND_REPO]: () => ({ binding: { state: 'bound' }, status: index }),
    [M.REINDEX]: (p) => {
      emit({ event: 'reindexProgress', projectId: p.projectId, worktreeId: p.worktreeId, percent: null, running: true })
      return { jobId: 'job-1' }
    },
    [M.REINDEX_STATUS]: () => ({ running: false, percent: null }),
    [M.REVIEW_STATE_GET]: () => reviewState,
    [M.REVIEW_STATE_SAVE]: (p) => {
      if (p.expectedVersion !== reviewState.version) {
        throw codeIntelError(E.VERSION_CONFLICT, 'expectedVersion is stale', {
          currentVersion: reviewState.version
        })
      }
      reviewState = {
        ...reviewState,
        ...(p as Partial<ReviewState>),
        version: reviewState.version + 1,
        updatedAt: new Date().toISOString()
      }
      delete (reviewState as Record<string, unknown>).expectedVersion
      return reviewState
    },
    [M.SUBSCRIBE]: () => null
  }

  async function call(method: string, ...args: unknown[]): Promise<unknown> {
    calls.push({ method, params: args[0] })
    const queued = failures.get(method)
    if (queued?.length) {
      throw queued.shift() as Error
    }
    if (!ALL_CODE_INTEL_CHANNELS.includes(method)) {
      throw new Error(`method_not_found: ${method}`)
    }
    const params = validate(method, args)
    gate(method)
    if (ADMIN_ONLY.has(method) && role !== 'admin') {
      throw codeIntelError(E.NOT_AUTHORIZED, 'administrator role required')
    }
    const handler = overrides.get(method) ?? builtIn[method]
    if (handler) {
      return handler(params)
    }
    if (data.has(method)) {
      const payload = data.get(method)
      return CODE_INTEL_ENVELOPE_METHODS.has(method) ? envelope(method, params, payload) : payload
    }
    throw new Error(`${method} is not yet implemented`)
  }

  const asEnvelope = async (method: string, params: unknown): Promise<CodeIntelRawEnvelope> => {
    try {
      const args = params === undefined ? [] : [params]
      return { ok: true, result: await call(method, ...args) }
    } catch (e) {
      const message = (e as Error).message
      return { ok: false, error: { code: message.startsWith('method_not_found') ? 'method_not_found' : 'internal', message } }
    }
  }

  const subscribe = (cb: { onEvent: (frame: unknown) => void; onClose: () => void }): (() => void) => {
    const l: StreamListener = { onEvent: cb.onEvent, onClose: cb.onClose }
    listeners.add(l)
    return () => {
      listeners.delete(l)
    }
  }

  const setData = (method: string, value: unknown): void => {
    data.set(method, value)
  }
  const sel = { projectId: 'project-1', worktreeId: 'worktree-1' }

  return {
    /** Throws `CODE: msg` errors (message carries the code, contract §2.3). */
    call,
    /** Raw-envelope variant, the shape the bridge `callEnvironment` returns. */
    callEnvelope: asEnvelope,
    /** Drop-in deps for `createCodeIntelBridge`. */
    bridgeDeps: {
      callLocal: (m, p) => asEnvelope(m, p),
      callEnvironment: (_env, m, p) => asEnvelope(m, p),
      subscribeEnvironment: (_env, _m, _p, cb) => {
        const off = subscribe(cb)
        return off
      }
    } satisfies CodeIntelBridgeDeps,
    subscribe,
    selector: sel,
    setRole: (r: 'admin' | 'user'): void => {
      role = r
    },
    setSettings: (patch: FakeSettingsPatch): void => {
      const { aiReviewEnabled, ...rest } = patch
      settings = { ...settings, tenant: { ...settings.tenant, ...rest } }
      if (aiReviewEnabled !== undefined) {
        settings = { ...settings, tenant: { ...settings.tenant, aiReviewLevel: aiReviewEnabled ? 'metadata' : 'off' } }
      }
      setEffective()
    },
    getSettings: (): CodeIntelSettings => settings,
    setIndex: (status: IndexStatus): void => {
      index = status
    },
    setOverlay: (v: unknown): void => setData(M.CHANGE_OVERLAY, v),
    setErd: (v: unknown): void => setData(M.ERD, v),
    setStorage: (v: unknown): void => setData(M.STORAGE, v),
    setFindings: (v: unknown): void => setData(M.FINDINGS, v),
    setContractDiff: (v: unknown): void => setData(M.CONTRACT_DIFF, v),
    setReviewState: (v: ReviewState): void => {
      reviewState = v
    },
    /** Generic data for any channel (envelope channels are wrapped in the §2.2 envelope). */
    setChannelData: setData,
    /** Replace a channel's behavior; lens tasks use this for non-static channels. */
    setHandler: (method: string, handler: FakeChannelHandler): void => {
      overrides.set(method, handler)
    },
    /** The next call to `method` rejects once with `CODE: msg`. */
    failNext: (method: string, message: string): void => {
      failures.set(method, [...(failures.get(method) ?? []), new Error(message)])
    },
    pushChanged: (worktreeId = sel.worktreeId, reason = 'commit', resync = false): void =>
      emit({ event: 'changed', projectId: sel.projectId, worktreeId, reason, resync }),
    pushProgress: (worktreeId = sel.worktreeId, percent: number | null = null, running = true): void =>
      emit({ event: 'reindexProgress', projectId: sel.projectId, worktreeId, percent, running }),
    pushQuality: (frame: Record<string, unknown>): void => emit({ projectId: sel.projectId, worktreeId: sel.worktreeId, ...frame }),
    /** Gateway behavior on a broken stream: resync frame, then close. */
    dropStream: (): void => {
      emit({ event: 'changed', projectId: sel.projectId, worktreeId: sel.worktreeId, reason: 'resync', resync: true })
      for (const l of Array.from(listeners)) {
        listeners.delete(l)
        l.onClose()
      }
    },
    streamCount: (): number => listeners.size,
    calls,
    callsTo: (method: string) => calls.filter((c) => c.method === method),
    reset: (): void => {
      settings = defaultSettings()
      index = INDEX_STATUS_FIXTURES.ready
      reviewState = { ...REVIEW_STATE_INITIAL }
      data.clear()
      overrides.clear()
      failures.clear()
      calls.length = 0
      listeners.clear()
      role = 'user'
    }
  }
}

export type FakeCodeIntelBackend = ReturnType<typeof createFakeCodeIntelBackend>
