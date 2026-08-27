// Why: additive local-desktop namespace mirroring ipc/rate-limits.ts's
// 'rateLimits:*' ipcMain channels 1:1 through the RPC dispatcher -- same
// RateLimitService instance, reached via runtime.getRateLimitService() on
// the desktop side. NOT the accounts.* mobile-bridge namespace (that one
// stays read+switch+remove-only).
//
// Push updates: desktop's local `window.api.runtime.call` transport is a
// plain one-shot ipcRenderer.invoke -- RpcDispatcher.dispatch() rejects
// streaming methods on that path ("requires a streaming transport"), so the
// existing window.api.rateLimits.onUpdate() direct IPC event channel remains
// the correct push mechanism for local desktop; it is untouched by this
// migration. rateLimits.subscribe/.unsubscribe were still added to the
// backend RPC namespace (mirroring accounts.subscribe/notifications.subscribe)
// for parity with the streaming-capable remote-environment transport, which
// is out of scope for this desktop-only migration.
import type {
  CodexRateLimitResetResult,
  RateLimitRuntimeTarget,
  RateLimitState
} from '../../../shared/rate-limit-types'
import { callRuntimeRpc } from './runtime-rpc-client'

export async function getRateLimitState(): Promise<RateLimitState> {
  return callRuntimeRpc<RateLimitState>({ kind: 'local' }, 'rateLimits.get')
}

export async function refreshRateLimits(): Promise<RateLimitState> {
  return callRuntimeRpc<RateLimitState>({ kind: 'local' }, 'rateLimits.refresh')
}

export async function refreshCodexRateLimitsForTarget(
  target: RateLimitRuntimeTarget
): Promise<RateLimitState> {
  return callRuntimeRpc<RateLimitState>({ kind: 'local' }, 'rateLimits.refreshCodexForTarget', target)
}

export async function consumeCodexRateLimitResetCredit(): Promise<CodexRateLimitResetResult> {
  return callRuntimeRpc<CodexRateLimitResetResult>(
    { kind: 'local' },
    'rateLimits.consumeCodexResetCredit'
  )
}

export async function refreshClaudeRateLimitsForTarget(
  target: RateLimitRuntimeTarget
): Promise<RateLimitState> {
  return callRuntimeRpc<RateLimitState>({ kind: 'local' }, 'rateLimits.refreshClaudeForTarget', target)
}

export async function setRateLimitPollingInterval(ms: number): Promise<void> {
  await callRuntimeRpc<void>({ kind: 'local' }, 'rateLimits.setPollingInterval', { ms })
}

export async function fetchInactiveClaudeRateLimitAccounts(): Promise<void> {
  await callRuntimeRpc<void>({ kind: 'local' }, 'rateLimits.fetchInactiveClaudeAccounts')
}

export async function fetchInactiveCodexRateLimitAccounts(): Promise<void> {
  await callRuntimeRpc<void>({ kind: 'local' }, 'rateLimits.fetchInactiveCodexAccounts')
}

export async function refreshMiniMaxRateLimits(): Promise<RateLimitState> {
  return callRuntimeRpc<RateLimitState>({ kind: 'local' }, 'rateLimits.refreshMiniMax')
}

export async function refreshGrokRateLimits(): Promise<RateLimitState> {
  return callRuntimeRpc<RateLimitState>({ kind: 'local' }, 'rateLimits.refreshGrok')
}
