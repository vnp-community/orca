/**
 * DevServerGitProvider — IGitProvider backed by a Dev Server's existing
 * agent-WebSocket connection (see dev-server-provider-lifecycle.ts).
 *
 * The dev-server agent exposes one generic `git.exec({args, cwd})` method
 * (whitelisted subcommands only — see src/relay/agent-git-handler.ts's
 * ALLOWED_GIT_SUBCOMMANDS) plus dedicated `git.worktree.list/add/remove`.
 * Unlike SshGitProvider (which forwards to dozens of fine-grained relay
 * methods like git.status/git.stage/git.push), every method here composes
 * from that narrow surface, reusing the same StatusPorcelainParser the
 * local (non-SSH) git path uses so status parsing stays consistent across
 * all three execution-host kinds. Methods with no reasonable composition
 * (submodules, history, branch/commit compare, fork sync) throw a clear
 * "not supported" error for v1 rather than a half-correct approximation.
 */
import { StatusPorcelainParser } from '../git/status-porcelain-parser'
import { isBinaryBuffer } from '../../shared/binary-buffer'
import { buildHostedRemoteCommitUrl, buildHostedRemoteFileUrl } from '../git/hosted-remote-url'
import type {
  GitBranchCompareResult,
  GitCommitCompareResult,
  GitConflictOperation,
  GitDiffResult,
  GitForkSyncExpectedUpstream,
  GitForkSyncResult,
  GitPushTarget,
  GitUpstreamStatus,
  GitWorktreeInfo,
  RemoveWorktreeResult
} from '../../shared/types'
import type { GitConflictKind, GitStatusResult } from '../../shared/git-status-types'
import type { GitHistoryResult } from '../../shared/git-history'
import type { CommitMessageDraftContext } from '../../shared/commit-message-generation'
import type { DevServerRelayConnection } from './dev-server-relay-connection'
import type { GitProviderStatusOptions, IGitProvider } from './types'

const NOT_SUPPORTED = (op: string): Error =>
  new Error(`${op} is not supported for Dev Server hosts yet.`)

type ExecResult = { stdout: string; stderr: string; exitCode: number }

type AgentWorktreeInfo = {
  path: string
  head: string
  branch: string
  bare: boolean
  detached: boolean
  prunable: boolean
  locked: boolean
  lockedReason?: string
}

function parseConflictKind(xy: string): GitConflictKind | null {
  switch (xy) {
    case 'UU':
      return 'both_modified'
    case 'AA':
      return 'both_added'
    case 'DD':
      return 'both_deleted'
    case 'AU':
      return 'added_by_us'
    case 'UA':
      return 'added_by_them'
    case 'DU':
      return 'deleted_by_us'
    case 'UD':
      return 'deleted_by_them'
    default:
      return null
  }
}

export class DevServerGitProvider implements IGitProvider {
  constructor(
    private readonly devServerId: string,
    private readonly relay: DevServerRelayConnection
  ) {}

  getConnectionId(): string {
    return this.devServerId
  }

  async exec(
    args: string[],
    cwd: string,
    options?: { timeoutMs?: number }
  ): Promise<{ stdout: string; stderr: string }> {
    const result = await this.relay.call<ExecResult>(
      'git.exec',
      { args, cwd },
      options?.timeoutMs
    )
    if (result.exitCode !== 0) {
      throw new Error(result.stderr.trim() || `git ${args.join(' ')} exited with code ${result.exitCode}`)
    }
    return { stdout: result.stdout, stderr: result.stderr }
  }

  // ── Status ───────────────────────────────────────────────────────────────

  async getStatus(worktreePath: string, options?: GitProviderStatusOptions): Promise<GitStatusResult> {
    const conflictOperation = await this.detectConflictOperation(worktreePath)
    const args = [
      '-c',
      'core.quotePath=false',
      'status',
      '--porcelain=v2',
      '--branch',
      '--untracked-files=all'
    ]
    if (options?.includeIgnored) {
      args.push('--ignored=matching')
    }

    const parser = new StatusPorcelainParser()
    let statusSucceeded = true
    try {
      const { stdout } = await this.exec(args, worktreePath)
      parser.update(stdout, 0)
      parser.finish()
    } catch {
      statusSucceeded = false
    }

    // Why: porcelain v2 `u` records need per-file resolution to fully classify
    // (see local git/status.ts's parseUnmergedEntry). Approximating the
    // compatibility `status` field as 'modified' is within the documented
    // tolerance (git-status-types.ts) — consumers must gate on conflictStatus,
    // not status, for conflict-aware behavior.
    for (const line of parser.unmergedLines) {
      const parts = line.split(' ')
      const xy = parts[1] ?? ''
      const filePath = parts.slice(10).join(' ')
      const conflictKind = parseConflictKind(xy)
      if (!filePath || !conflictKind) {continue}
      parser.entries.push({
        path: filePath,
        area: 'unstaged',
        status: 'modified',
        conflictKind,
        conflictStatus: 'unresolved'
      })
    }

    const { head, branch, upstreamName, upstreamAheadBehind } = parser.branch

    return {
      entries: parser.entries,
      conflictOperation,
      head,
      branch,
      ...(options?.includeIgnored ? { ignoredPaths: parser.ignoredPaths } : {}),
      ...(statusSucceeded
        ? {
            upstreamStatus: upstreamName
              ? ({
                  hasUpstream: true,
                  upstreamName,
                  ahead: upstreamAheadBehind?.ahead ?? 0,
                  behind: upstreamAheadBehind?.behind ?? 0
                } satisfies GitUpstreamStatus)
              : ({ hasUpstream: false, ahead: 0, behind: 0 } satisfies GitUpstreamStatus)
          }
        : {})
    }
  }

  async getUpstreamStatus(worktreePath: string): Promise<GitUpstreamStatus> {
    const status = await this.getStatus(worktreePath)
    return status.upstreamStatus ?? { hasUpstream: false, ahead: 0, behind: 0 }
  }

  async getSubmoduleStatus(): Promise<GitStatusResult> {
    throw NOT_SUPPORTED('Submodule status')
  }

  async checkIgnoredPaths(): Promise<string[]> {
    throw NOT_SUPPORTED('Ignored-path checks')
  }

  async getHistory(): Promise<GitHistoryResult> {
    throw NOT_SUPPORTED('Commit history')
  }

  async getStagedCommitContext(): Promise<CommitMessageDraftContext | null> {
    throw NOT_SUPPORTED('AI commit-message context')
  }

  /**
   * Detects merge/rebase/cherry-pick in progress by checking for the same
   * marker files the local git path checks (status.ts's detectConflictOperation),
   * via fs.stat instead of a local existsSync.
   */
  async detectConflictOperation(worktreePath: string): Promise<GitConflictOperation> {
    const gitDir = await this.resolveGitDir(worktreePath)
    const exists = async (relPath: string): Promise<boolean> => {
      try {
        await this.relay.call('fs.stat', { path: `${gitDir}/${relPath}` })
        return true
      } catch {
        return false
      }
    }
    if (await exists('MERGE_HEAD')) {return 'merge'}
    if ((await exists('rebase-merge')) || (await exists('rebase-apply'))) {return 'rebase'}
    if (await exists('CHERRY_PICK_HEAD')) {return 'cherry-pick'}
    return 'unknown'
  }

  private async resolveGitDir(worktreePath: string): Promise<string> {
    try {
      const { stdout } = await this.exec(['rev-parse', '--git-dir'], worktreePath)
      const dir = stdout.trim()
      if (!dir) {return `${worktreePath}/.git`}
      return dir.startsWith('/') ? dir : `${worktreePath}/${dir}`
    } catch {
      return `${worktreePath}/.git`
    }
  }

  async abortMerge(worktreePath: string): Promise<void> {
    await this.exec(['merge', '--abort'], worktreePath)
  }

  async abortRebase(worktreePath: string): Promise<void> {
    await this.exec(['rebase', '--abort'], worktreePath)
  }

  // ── Diff ─────────────────────────────────────────────────────────────────

  async getDiff(
    worktreePath: string,
    filePath: string,
    staged: boolean,
    compareAgainstHead = false
  ): Promise<GitDiffResult> {
    const readBlob = async (ref: string): Promise<{ content: string; isBinary: boolean }> => {
      try {
        const { stdout } = await this.exec(['show', `${ref}:${filePath}`], worktreePath)
        return { content: stdout, isBinary: isBinaryBuffer(Buffer.from(stdout, 'utf-8')) }
      } catch {
        return { content: '', isBinary: false }
      }
    }
    const readWorkingTree = async (): Promise<{ content: string; isBinary: boolean }> => {
      // Why: the working tree copy isn't a git object — read it straight off
      // disk via the agent's fs.readFile rather than a git blob lookup.
      const result = await this.relay.call<{ content: string; isBinary: boolean }>('fs.readFile', {
        path: `${worktreePath}/${filePath}`
      })
      return { content: result.content, isBinary: result.isBinary }
    }

    // Why: `git show :<path>` (empty ref before the colon) reads the index
    // (staged) blob — this is the "before" side for an unstaged diff and the
    // "after" side for a staged diff, matching the local git path's semantics.
    let original = { content: '', isBinary: false }
    let modified = { content: '', isBinary: false }
    if (staged) {
      original = await readBlob('HEAD')
      modified = await readBlob('')
    } else if (compareAgainstHead) {
      original = await readBlob('HEAD')
      modified = await readWorkingTree()
    } else {
      original = await readBlob('')
      modified = await readWorkingTree()
    }

    if (original.isBinary || modified.isBinary) {
      return {
        kind: 'binary',
        originalContent: original.content,
        modifiedContent: modified.content,
        originalIsBinary: original.isBinary,
        modifiedIsBinary: modified.isBinary
      } as GitDiffResult
    }
    return {
      kind: 'text',
      originalContent: original.content,
      modifiedContent: modified.content,
      originalIsBinary: false,
      modifiedIsBinary: false
    }
  }

  async getBranchDiff(): Promise<GitDiffResult[]> {
    throw NOT_SUPPORTED('Branch diff')
  }

  async getCommitDiff(): Promise<GitDiffResult> {
    throw NOT_SUPPORTED('Commit diff')
  }

  async getBranchCompare(): Promise<GitBranchCompareResult> {
    throw NOT_SUPPORTED('Branch comparison')
  }

  async getCommitCompare(): Promise<GitCommitCompareResult> {
    throw NOT_SUPPORTED('Commit comparison')
  }

  async syncForkDefaultBranch(
    _worktreePath: string,
    _expectedUpstream: GitForkSyncExpectedUpstream
  ): Promise<GitForkSyncResult> {
    throw NOT_SUPPORTED('Fork sync')
  }

  // ── Staging / commit ─────────────────────────────────────────────────────

  async stageFile(worktreePath: string, filePath: string): Promise<void> {
    await this.exec(['add', '--', filePath], worktreePath)
  }

  async unstageFile(worktreePath: string, filePath: string): Promise<void> {
    await this.exec(['restore', '--staged', '--', filePath], worktreePath)
  }

  async bulkStageFiles(worktreePath: string, filePaths: string[]): Promise<void> {
    if (filePaths.length === 0) {return}
    await this.exec(['add', '--', ...filePaths], worktreePath)
  }

  async bulkUnstageFiles(worktreePath: string, filePaths: string[]): Promise<void> {
    if (filePaths.length === 0) {return}
    await this.exec(['restore', '--staged', '--', ...filePaths], worktreePath)
  }

  async discardChanges(worktreePath: string, filePath: string): Promise<void> {
    await this.discardOne(worktreePath, filePath)
  }

  async bulkDiscardChanges(worktreePath: string, filePaths: string[]): Promise<void> {
    for (const filePath of filePaths) {
      await this.discardOne(worktreePath, filePath)
    }
  }

  private async discardOne(worktreePath: string, filePath: string): Promise<void> {
    // Why: `restore --worktree` only reverts tracked-file edits; it never
    // removes untracked files. Detect untracked via status and delete
    // through fs.rmdir instead — mirrors the two-path behavior of the SSH
    // relay's bulkDiscard (tracked vs untracked).
    const status = await this.getStatus(worktreePath)
    const entry = status.entries.find((e) => e.path === filePath)
    if (entry?.status === 'untracked') {
      // Why: recursive:true routes through the agent's `rm(path, {recursive,
      // force})`, which (unlike plain rmdir) also removes plain files —
      // untracked discard targets are usually files, not directories.
      await this.relay.call('fs.rmdir', { path: `${worktreePath}/${filePath}`, recursive: true })
      return
    }
    await this.exec(['restore', '--worktree', '--', filePath], worktreePath)
  }

  async commit(worktreePath: string, message: string): Promise<{ success: boolean; error?: string }> {
    try {
      await this.exec(['commit', '-m', message], worktreePath)
      return { success: true }
    } catch (err) {
      return { success: false, error: err instanceof Error ? err.message : String(err) }
    }
  }

  // ── Branches / remotes ───────────────────────────────────────────────────

  async checkoutBranch(worktreePath: string, branch: string): Promise<void> {
    await this.exec(['checkout', branch], worktreePath)
  }

  async listLocalBranches(
    worktreePath: string
  ): Promise<{ current: string | null; branches: string[] }> {
    const { stdout } = await this.exec(['branch', '--list'], worktreePath)
    const branches: string[] = []
    let current: string | null = null
    for (const rawLine of stdout.split('\n')) {
      const line = rawLine.trim()
      if (!line) {continue}
      const isCurrent = line.startsWith('* ')
      const name = isCurrent ? line.slice(2).trim() : line
      branches.push(name)
      if (isCurrent) {current = name}
    }
    return { current, branches }
  }

  private targetArgs(pushTarget?: GitPushTarget): string[] {
    if (!pushTarget) {return []}
    return [pushTarget.remoteName ?? 'origin', pushTarget.branchName].filter((v): v is string =>
      Boolean(v)
    )
  }

  async pushBranch(
    worktreePath: string,
    _publish?: boolean,
    pushTarget?: GitPushTarget,
    options?: { forceWithLease?: boolean }
  ): Promise<void> {
    const args = ['push', ...this.targetArgs(pushTarget)]
    if (options?.forceWithLease) {args.push('--force-with-lease')}
    await this.exec(args, worktreePath)
  }

  async pullBranch(worktreePath: string, pushTarget?: GitPushTarget): Promise<void> {
    await this.exec(['pull', ...this.targetArgs(pushTarget)], worktreePath)
  }

  async fastForwardBranch(worktreePath: string, pushTarget?: GitPushTarget): Promise<void> {
    await this.exec(['pull', '--ff-only', ...this.targetArgs(pushTarget)], worktreePath)
  }

  async fetchRemote(worktreePath: string, pushTarget?: GitPushTarget): Promise<void> {
    await this.exec(['fetch', pushTarget?.remoteName ?? 'origin'], worktreePath)
  }

  async rebaseFromBase(worktreePath: string, baseRef: string): Promise<void> {
    await this.exec(['rebase', baseRef], worktreePath)
  }

  // ── Worktrees ────────────────────────────────────────────────────────────

  async listWorktrees(repoPath: string): Promise<GitWorktreeInfo[]> {
    const result = await this.relay.call<{ worktrees: AgentWorktreeInfo[] }>('git.worktree.list', {
      cwd: repoPath
    })
    return result.worktrees.map((wt, index) => ({
      path: wt.path,
      head: wt.head,
      branch: wt.branch,
      isBare: wt.bare,
      locked: wt.locked,
      lockReason: wt.lockedReason,
      isMainWorktree: index === 0
    }))
  }

  async addWorktree(
    repoPath: string,
    branchName: string,
    targetDir: string,
    options?: { checkoutExistingBranch?: boolean }
  ): Promise<void> {
    const result = await this.relay.call<ExecResult>('git.worktree.add', {
      path: targetDir,
      branch: branchName,
      createBranch: options?.checkoutExistingBranch !== true,
      cwd: repoPath
    })
    if (result.exitCode !== 0) {
      throw new Error(result.stderr.trim() || `git worktree add exited with code ${result.exitCode}`)
    }
  }

  async removeWorktree(worktreePath: string, force?: boolean): Promise<RemoveWorktreeResult> {
    const result = await this.relay.call<ExecResult>('git.worktree.remove', {
      path: worktreePath,
      force: force === true
    })
    if (result.exitCode !== 0) {
      throw new Error(result.stderr.trim() || `git worktree remove exited with code ${result.exitCode}`)
    }
    return {}
  }

  async worktreeIsClean(
    worktreePath: string,
    options?: { includeUntracked?: boolean }
  ): Promise<{ clean: boolean; stdout?: string }> {
    const status = await this.getStatus(worktreePath)
    const entries = options?.includeUntracked
      ? status.entries
      : status.entries.filter((e) => e.status !== 'untracked')
    return { clean: entries.length === 0 }
  }

  // ── Misc ─────────────────────────────────────────────────────────────────

  isGitRepo(): boolean {
    // Why: this must be synchronous per the interface, but a remote check
    // inherently isn't. Dev-Server-backed repos are only ever activated
    // after isGitRepoAsync already confirmed this, so a permissive default
    // here is safe — callers on this path use isGitRepoAsync as the guard.
    return true
  }

  async isGitRepoAsync(dirPath: string): Promise<{ isRepo: boolean; rootPath: string | null }> {
    try {
      const { stdout } = await this.exec(['rev-parse', '--show-toplevel'], dirPath)
      const rootPath = stdout.trim()
      return { isRepo: Boolean(rootPath), rootPath: rootPath || null }
    } catch {
      return { isRepo: false, rootPath: null }
    }
  }

  private async readOriginRemoteUrl(worktreePath: string): Promise<string | null> {
    try {
      const { stdout } = await this.exec(['remote', 'get-url', 'origin'], worktreePath)
      return stdout.trim() || null
    } catch {
      return null
    }
  }

  async getRemoteFileUrl(
    worktreePath: string,
    relativePath: string,
    line: number
  ): Promise<string | null> {
    const remoteUrl = await this.readOriginRemoteUrl(worktreePath)
    if (!remoteUrl) {return null}

    let defaultBranch = 'main'
    try {
      const { stdout } = await this.exec(['rev-parse', '--abbrev-ref', 'origin/HEAD'], worktreePath)
      const ref = stdout.trim()
      if (ref) {defaultBranch = ref.replace(/^origin\//, '')}
    } catch {
      // Fall back to 'main'
    }
    return buildHostedRemoteFileUrl(remoteUrl, relativePath, defaultBranch, line)
  }

  async getRemoteCommitUrl(worktreePath: string, sha: string): Promise<string | null> {
    const remoteUrl = await this.readOriginRemoteUrl(worktreePath)
    if (!remoteUrl) {return null}
    return buildHostedRemoteCommitUrl(remoteUrl, sha)
  }
}
