// BUG-FE-PW-005 / SOL-FE-PW-004: web-compatible replacement for
// window.api.agentOrchestration (an Electron IPC bridge that is undefined on
// web, AND whose main-process handlers were never wired up on desktop
// either — registerAgentOrchestrationHandlers is never called, and
// OrcaRuntimeService never grew the startAgent/stopAgent/resumeAgent methods
// it was written against). This client instead calls the real, already-
// working backend-go agent.*/aiProvider.* wscompat channels
// (channels_agent.go, channels_ai_provider.go) — the only place this feature
// has ever actually been implemented, per TASK-AG-01-07.
import {
  callRuntimeRpc,
  subscribeRuntimeStreamChannel,
  type RuntimeClientTarget
} from './runtime-rpc-client'

export type RuntimeAgentSessionStatus =
  | 'spawning'
  | 'idle'
  | 'running'
  | 'waiting'
  | 'completed'
  | 'error'
  | 'stopped'

// Mirrors agentSessionView (channels_agent.go) exactly.
export type RuntimeAgentSession = {
  id: string
  ptyId: string
  worktreeId: string
  devServerId: string
  userId: string
  modelId: string
  accountId: string
  status: RuntimeAgentSessionStatus
  startedAtUnixMs: number
  lastActiveAtUnixMs: number
}

// agent.start/agent.resume/agent.switchAccount reuse terminal.create's
// AttachPty push mechanism (channels_agent.go's file header) — the shape of
// each push frame's own payload isn't pinned down by this client; callers
// that need pty output should treat these as opaque and route them through
// the existing terminal-output plumbing, not reparse them here.
export type RuntimeAgentPtyEvent = unknown

export type RuntimeAgentStatusEvent = {
  session_id: string
  status: RuntimeAgentSessionStatus
}

// Mirrors providerAccountView (channels_ai_provider.go, BUG-022's fix) —
// only the fields resolveRuntimeAgentModel needs.
type ResolvedProviderAccountView = {
  id: string
  modelHint: string
  models: string[]
}

export type ResolvedAgentProvider = {
  accountId: string
  modelId: string
}

// detectProviderFromModel (ai-provider-service/internal/usecase/
// model_provider_map.go) matches modelHint by PREFIX ONLY, so any string
// carrying the right prefix narrows ResolveProvider's cascade the same way —
// these aren't real model ids, just prefix probes. 'custom' sends no hint at
// all: the cascade runs unfiltered by provider.
const AGENT_TYPE_MODEL_HINT: Partial<Record<'claude' | 'codex' | 'custom', string>> = {
  claude: 'claude-',
  codex: 'gpt-'
}

/**
 * Resolves an accountId + modelId for agent.start/agent.resume, via
 * aiProvider.resolve's user -> project -> server cascade (TASK-AG-01-07 —
 * StartAgentSession itself never calls Resolve; the caller must).
 */
export async function resolveRuntimeAgentProvider(
  target: RuntimeClientTarget,
  args: {
    userId: string
    projectId: string
    devServerId: string
    agentType: 'claude' | 'codex' | 'custom'
  }
): Promise<ResolvedAgentProvider> {
  const modelHint = AGENT_TYPE_MODEL_HINT[args.agentType]
  const account = await callRuntimeRpc<ResolvedProviderAccountView>(target, 'aiProvider.resolve', {
    userId: args.userId,
    projectId: args.projectId,
    devServerId: args.devServerId,
    ...(modelHint ? { modelHint } : {})
  })
  return {
    accountId: account.id,
    modelId: account.modelHint || account.models?.[0] || ''
  }
}

export function startRuntimeAgentSession(
  target: RuntimeClientTarget,
  args: {
    connectionId: string
    worktreeId: string
    userId: string
    cwd: string
    modelId: string
    accountId: string
    trustPreset: string
    cols?: number
    rows?: number
  },
  onEvent: (event: RuntimeAgentPtyEvent) => void
): Promise<{ ack: RuntimeAgentSession; unsubscribe: () => void }> {
  return subscribeRuntimeStreamChannel<RuntimeAgentSession, RuntimeAgentPtyEvent>(
    target,
    'agent.start',
    args,
    onEvent
  )
}

export function resumeRuntimeAgentSession(
  target: RuntimeClientTarget,
  args: {
    connectionId: string
    worktreeId: string
    userId: string
    cwd: string
    cols?: number
    rows?: number
  },
  onEvent: (event: RuntimeAgentPtyEvent) => void
): Promise<{ ack: RuntimeAgentSession; unsubscribe: () => void }> {
  return subscribeRuntimeStreamChannel<RuntimeAgentSession, RuntimeAgentPtyEvent>(
    target,
    'agent.resume',
    args,
    onEvent
  )
}

export async function stopRuntimeAgentSession(
  target: RuntimeClientTarget,
  sessionId: string
): Promise<void> {
  await callRuntimeRpc(target, 'agent.stop', { sessionId })
}

export async function killRuntimeAgentSession(
  target: RuntimeClientTarget,
  sessionId: string,
  signal?: string
): Promise<void> {
  await callRuntimeRpc(target, 'agent.kill', { sessionId, signal: signal ?? '' })
}

/**
 * agent.subscribeStatus is a pure-push channel (RegisterStream, no
 * meaningful ack payload — same shape as accounts.subscribe, see
 * runtime-provider-accounts-client.ts's precedent comment) that delivers
 * every agent session's status change for this tenant; callers must filter
 * by their own tracked sessionId.
 */
export function subscribeRuntimeAgentStatus(
  target: RuntimeClientTarget,
  onEvent: (event: RuntimeAgentStatusEvent) => void
): Promise<{ unsubscribe: () => void }> {
  return subscribeRuntimeStreamChannel<unknown, RuntimeAgentStatusEvent>(
    target,
    'agent.subscribeStatus',
    {},
    onEvent
  )
}
