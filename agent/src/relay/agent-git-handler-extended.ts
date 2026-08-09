// agent/src/relay/agent-git-handler-extended.ts (NEW)
// Handlers for the Dev Server WS agent's git RPC surface that SSH relay's
// GitHandler class already has (git-handler.ts) but agent-rpc-dispatch.ts's
// narrow git.exec-only router does not. Reuses the same decoupled ops
// functions GitHandler calls internally — no logic duplicated.
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { loadGitHistoryFromExecutor } from '../shared/git-history'
import { branchCompare as branchCompareOp, branchDiffEntries } from './git-handler-ops'
import { commitCompare as commitCompareOp, commitDiffEntry } from './git-handler-commit-diff-ops'
import { checkIgnoredPathsOp } from './git-handler-check-ignore'
import { syncForkDefaultBranch, validateGitForkSyncExpectedUpstream } from '../shared/git-fork-sync'
import { parseBranchDiff } from './git-handler-utils'
import { parseNumstat } from '../shared/git-uncommitted-line-stats'
import {
  resolveSubmoduleWorktreePath,
  resolveSubmoduleCommitRange,
  computeSubmoduleRangeEntries
} from './git-handler-submodule-ops'
import { getStatusOp } from './git-handler-status-ops'

const execFileAsync = promisify(execFile)
const MAX_GIT_BUFFER = 10 * 1024 * 1024

// Why: internal executor for the fixed-shape ops below (branch compare,
// history, ...) — NOT exposed as free-form exec, so it does not need
// agent-git-handler.ts's ALLOWED_GIT_SUBCOMMANDS whitelist (that whitelist
// guards the generic git.exec passthrough only). Mirrors GitHandler.git()
// in git-handler.ts, minus SSH-specific env knobs not needed here.
async function git(
  args: string[],
  cwd: string,
  opts?: { stdin?: string; timeout?: number }
): Promise<{ stdout: string; stderr: string }> {
  const { stdout, stderr } = await execFileAsync('git', args, {
    cwd,
    encoding: 'utf-8',
    maxBuffer: MAX_GIT_BUFFER,
    timeout: opts?.timeout
  })
  return { stdout: String(stdout), stderr: String(stderr) }
}

async function gitBuffer(args: string[], cwd: string): Promise<Buffer> {
  const { stdout } = (await execFileAsync('git', args, {
    cwd,
    encoding: 'buffer',
    maxBuffer: MAX_GIT_BUFFER
  })) as unknown as { stdout: Buffer }
  return stdout
}

// ── git.history ─────────────────────────────────────────────────────────
export async function handleGitHistory(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const worktreePath = params.worktreePath as string
  try {
    const result = await loadGitHistoryFromExecutor(git, worktreePath, {
      limit: typeof params.limit === 'number' ? params.limit : undefined,
      baseRef: typeof params.baseRef === 'string' ? params.baseRef : null
    })
    log.info(`git.history: worktreePath=${worktreePath} items=${result.items.length}`)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.branchCompare ──────────────────────────────────────────────────
export async function handleGitBranchCompare(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const baseRef = params.baseRef as string
  if (baseRef.startsWith('-')) {
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Base ref must not start with "-"' }
    }
  }
  const result = await branchCompareOp(git, worktreePath, baseRef, async (mergeBase, headOid) => {
    const { stdout } = await git(
      ['-c', 'core.quotePath=false', 'diff', '--name-status', '-M', '-C', mergeBase, headOid],
      worktreePath
    )
    const { stdout: numstat } = await git(
      ['-c', 'core.quotePath=false', 'diff', '--numstat', '-M', '-C', mergeBase, headOid],
      worktreePath
    )
    return parseBranchDiff(stdout, parseNumstat(numstat))
  })
  return { jsonrpc: '2.0', id, result }
}

// ── git.commitCompare ──────────────────────────────────────────────────
export async function handleGitCommitCompare(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const commitId = params.commitId as string
  const result = await commitCompareOp(git, worktreePath, commitId)
  return { jsonrpc: '2.0', id, result }
}

// ── git.branchDiff ─────────────────────────────────────────────────────
export async function handleGitBranchDiff(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const baseRef = params.baseRef as string
  if (baseRef.startsWith('-')) {
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Base ref must not start with "-"' }
    }
  }
  const result = await branchDiffEntries(git, gitBuffer, worktreePath, baseRef, {
    includePatch: params.includePatch as boolean | undefined,
    filePath: params.filePath as string | undefined,
    oldPath: params.oldPath as string | undefined
  })
  return { jsonrpc: '2.0', id, result }
}

// ── git.commitDiff ─────────────────────────────────────────────────────
export async function handleGitCommitDiff(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const result = await commitDiffEntry(gitBuffer, worktreePath, {
    commitOid: params.commitOid as string,
    parentOid: params.parentOid as string | null | undefined,
    filePath: params.filePath as string,
    oldPath: params.oldPath as string | undefined
  })
  return { jsonrpc: '2.0', id, result }
}

// ── git.checkIgnored ───────────────────────────────────────────────────
export async function handleGitCheckIgnored(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const result = await checkIgnoredPathsOp(git, params)
  return { jsonrpc: '2.0', id, result }
}

// ── git.forkSync ───────────────────────────────────────────────────────
export async function handleGitForkSync(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  try {
    const expectedUpstream = validateGitForkSyncExpectedUpstream(params.expectedUpstream, {
      required: true
    })
    const result = await syncForkDefaultBranch(
      (args) => git(args, worktreePath, { timeout: 60_000 }),
      { expectedUpstream }
    )
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.submoduleStatus ────────────────────────────────────────────────
export async function handleGitSubmoduleStatus(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const submodulePath = params.submodulePath as string
  const area = params.area === 'staged' || params.area === 'untracked' ? params.area : 'unstaged'
  const staged = area === 'staged'
  const resolved = resolveSubmoduleWorktreePath(worktreePath, submodulePath)
  const workingResult = await getStatusOp(git, { ...params, worktreePath: resolved })
  const { fromOid, toOid } = await resolveSubmoduleCommitRange(
    git,
    worktreePath,
    submodulePath,
    staged
  )
  if (fromOid && toOid && fromOid !== toOid) {
    const rangeEntries = await computeSubmoduleRangeEntries(git, resolved, fromOid, toOid)
    const result = staged
      ? { ...workingResult, entries: rangeEntries }
      : {
          ...workingResult,
          entries: [
            ...rangeEntries,
            ...workingResult.entries.filter(
              // Why: entries are Record<string, unknown> at this layer (getStatusOp/
              // computeSubmoduleRangeEntries's declared return shape) — cast to read
              // `.path` rather than narrowing the filter callback's param type, which
              // TS rejects as an unsound overload against Record<string, unknown>[].
              (e) => !rangeEntries.some((r) => (r as { path: string }).path === (e as { path: string }).path)
            )
          ]
        }
    return { jsonrpc: '2.0', id, result }
  }
  const result = staged ? { ...workingResult, entries: [] } : workingResult
  return { jsonrpc: '2.0', id, result }
}
