// agent/src/relay/agent-git-handler-local-ops.ts
// TASK-227: re-export handlers for the git.* methods that only touch the
// local worktree/index (no configured remote involved) — status, diff,
// commit, staging, checkout/branches, and conflict abort/detect. Same
// try/catch/JsonRpcResponse shape as agent-git-handler-extended.ts's
// existing re-exports; delegates are Part B's already-built ops modules
// (git-handler.ts's private methods, extracted into importable functions).
// See specs/backend-go/bugs/missing-v1/tasks/
// TASK-227-expose-git-status-diff-commit-on-agent-part-a.md.
import * as path from 'node:path'
import type WebSocket from 'ws'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { git, gitBuffer } from './agent-git-handler-extended'
import { computeDiff } from './git-handler-ops'
import { detectConflictOperation, getStatusOp } from './git-handler-status-ops'
import { commitChangesRelay } from './git-handler-worktree-ops'
import { stage, unstage, bulkStage, bulkUnstage } from './git-handler-staging-ops'
import {
  checkout,
  localBranches,
  abortMerge,
  abortRebase,
  discard,
  bulkDiscard
} from './git-handler-branch-ops'
import { getConnectionGitIdentity, buildGitIdentityEnv } from './git-identity-registry'

// ── git.status ──────────────────────────────────────────────────────────
export async function handleGitStatus(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    const result = await getStatusOp(git, params)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.diff ────────────────────────────────────────────────────────────
export async function handleGitDiff(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    const worktreePath = params.worktreePath as string
    const filePath = params.filePath as string
    // Why: filePath is relative to worktreePath and used to read blobs/working
    // files by path.join. Without validation, ../../etc/passwd traverses
    // outside the worktree (mirrors GitHandler.getDiff's guard).
    const resolved = path.resolve(worktreePath, filePath)
    const rel = path.relative(path.resolve(worktreePath), resolved)
    if (rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)) {
      throw new Error(`Path "${filePath}" resolves outside the worktree`)
    }
    const staged = params.staged as boolean
    const compareAgainstHead = params.compareAgainstHead as boolean | undefined
    const result = await computeDiff(gitBuffer, worktreePath, filePath, staged, compareAgainstHead)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.commit ──────────────────────────────────────────────────────────
export async function handleGitCommit(
  id: string | number | null,
  params: Record<string, unknown>,
  ws: WebSocket
): Promise<object> {
  try {
    const worktreePath = params.worktreePath as string
    const message = params.message as string
    // Why: BUG-AG-HLD-003 parity — author/committer come from this
    // connection's preflight.setGitIdentity call (if any), never from global
    // git config. Part A has no numeric clientId (see git-identity-registry.ts
    // "Part A variant"), so identity is keyed by the WebSocket connection.
    const identityEnv = buildGitIdentityEnv(getConnectionGitIdentity(ws))
    const result = await commitChangesRelay(
      (args, cwd) => git(args, cwd, { extraEnv: identityEnv }),
      worktreePath,
      message
    )
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.checkout ────────────────────────────────────────────────────────
export async function handleGitCheckout(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    const result = await checkout(git, params)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.localBranches ───────────────────────────────────────────────────
export async function handleGitLocalBranches(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    const result = await localBranches(git, params)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.abortRebase ─────────────────────────────────────────────────────
export async function handleGitAbortRebase(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await abortRebase(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.abortMerge ──────────────────────────────────────────────────────
export async function handleGitAbortMerge(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await abortMerge(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.conflictOperation ───────────────────────────────────────────────
// Detector only — returns 'merge'|'rebase'|'cherry-pick'|'unknown'. Does NOT
// take path/operation params. See SOL-032 §0 open question #2 for why the
// Go-side RPC design around this method needs rework, not this task.
export async function handleGitConflictOperation(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    const worktreePath = params.worktreePath as string
    const result = await detectConflictOperation(worktreePath)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.discard ─────────────────────────────────────────────────────────
export async function handleGitDiscard(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await discard(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.bulkDiscard ─────────────────────────────────────────────────────
export async function handleGitBulkDiscard(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await bulkDiscard(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.stage ────────────────────────────────────────────────────────────
export async function handleGitStage(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await stage(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.unstage ──────────────────────────────────────────────────────────
export async function handleGitUnstage(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await unstage(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.bulkStage ────────────────────────────────────────────────────────
export async function handleGitBulkStage(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await bulkStage(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.bulkUnstage ─────────────────────────────────────────────────────
export async function handleGitBulkUnstage(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  try {
    await bulkUnstage(git, params)
    return { jsonrpc: '2.0', id, result: { success: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
