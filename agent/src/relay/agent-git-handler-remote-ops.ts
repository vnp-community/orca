// agent/src/relay/agent-git-handler-remote-ops.ts
// TASK-227: re-export handlers for the git.* methods that talk to a
// configured remote — push, pull, fastForward, rebaseFromBase, fetch, and
// upstreamStatus. Same try/catch/JsonRpcResponse shape as
// agent-git-handler-extended.ts's existing re-exports; delegates are Part
// B's already-built ops modules (git-handler.ts's private methods,
// extracted into importable functions). See
// specs/backend-go/bugs/missing-v1/tasks/
// TASK-227-expose-git-status-diff-commit-on-agent-part-a.md.
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { git } from './agent-git-handler-extended'
import { push, pull, fastForward, fetch, upstreamStatus } from './git-handler-remote-sync-ops'
import { rebaseFromBase } from './git-handler-branch-ops'

// ── git.push ────────────────────────────────────────────────────────────
export async function handleGitPush(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await push(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.pull ────────────────────────────────────────────────────────────
export async function handleGitPull(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await pull(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.fastForward ─────────────────────────────────────────────────────
export async function handleGitFastForward(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await fastForward(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.rebaseFromBase ──────────────────────────────────────────────────
export async function handleGitRebaseFromBase(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await rebaseFromBase(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.fetch ────────────────────────────────────────────────────────────
export async function handleGitFetch(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await fetch(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.upstreamStatus ──────────────────────────────────────────────────
export async function handleGitUpstreamStatus(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    const result = await upstreamStatus(git, params)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
