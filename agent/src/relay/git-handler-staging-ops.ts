/**
 * Staging-area mutations extracted from git-handler.ts (GitHandler.stage/
 * unstage/bulkStage/bulkUnstage) so the Dev Server WS agent's Part A
 * dispatcher (agent-git-handler-extended.ts) can re-export them without
 * depending on the GitHandler class.
 *
 * Why the cache-clearing calls are gone: GitHandler.stage() etc. bracket
 * every mutation with `this.clearGitMutationReadCaches()` to invalidate the
 * SSH relay's persistent per-connection diff/submodule read caches (see
 * git-handler.ts). Part A has no such cache — each RPC call is a fresh,
 * stateless git invocation — so there is nothing to clear here.
 */
import type { GitExec } from './git-handler-ops'

const BULK_CHUNK_SIZE = 100

function literalPathspec(filePath: string): string {
  // Why: source-control selections are concrete paths, not user-authored Git globs.
  return `:(literal)${filePath}`
}

export async function stage(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  const filePath = params.filePath as string
  await git(['add', '--', literalPathspec(filePath)], worktreePath)
}

export async function unstage(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  const filePath = params.filePath as string
  await git(['restore', '--staged', '--', literalPathspec(filePath)], worktreePath)
}

export async function bulkStage(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  const filePaths = params.filePaths as string[]
  for (let i = 0; i < filePaths.length; i += BULK_CHUNK_SIZE) {
    const chunk = filePaths.slice(i, i + BULK_CHUNK_SIZE)
    await git(['add', '--', ...chunk.map((filePath) => literalPathspec(filePath))], worktreePath)
  }
}

export async function bulkUnstage(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  const filePaths = params.filePaths as string[]
  for (let i = 0; i < filePaths.length; i += BULK_CHUNK_SIZE) {
    const chunk = filePaths.slice(i, i + BULK_CHUNK_SIZE)
    await git(
      ['restore', '--staged', '--', ...chunk.map((filePath) => literalPathspec(filePath))],
      worktreePath
    )
  }
}
