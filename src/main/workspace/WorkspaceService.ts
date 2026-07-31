/**
 * WorkspaceService — Project workspace initialization and state aggregation (TDD-19)
 *
 * Provides a single call to hydrate all workspace state in parallel:
 * - Git status (porcelain v2)
 * - Git worktrees list
 * - File tree (depth 2)
 * - Pending tasks (todo/in_progress/blocked)
 *
 * All relay calls are offline-tolerant (catch → return empty/null).
 *
 * @module main/workspace/WorkspaceService
 */

import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { ProfileResolver } from '../profile/ProfileResolver'
import type { TaskService } from '../task/TaskService'
import type { WorkflowOrchestrator } from '../workflow/WorkflowOrchestrator'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'
import type { OrcaTask } from '../../shared/task-types'

// ── Public types ──────────────────────────────────────────────────────────────

export interface GitStatus {
  branch: string
  upstream?: string
  ahead: number
  behind: number
  staged: number
  unstaged: number
  untracked: number
  files: GitStatusFile[]
}

export interface GitStatusFile {
  path: string
  x: string  // staged status char
  y: string  // unstaged status char
}

export interface GitWorktree {
  path: string
  branch: string
  head: string
  isMain: boolean
  isLocked: boolean
}

export interface FileTreeNode {
  name: string
  path: string
  isDir: boolean
  children?: FileTreeNode[]
}

export interface WorkspaceInitResult {
  gitStatus: GitStatus | null
  worktrees: GitWorktree[]
  fileTree: FileTreeNode[]
  pendingTasks: OrcaTask[]
}

// ── WorkspaceService ──────────────────────────────────────────────────────────

export class WorkspaceService {
  constructor(
    private readonly router: ProjectServerRouter,
    private readonly profileResolver: ProfileResolver,
    private readonly taskService: TaskService,
    private readonly workflowOrchestrator: WorkflowOrchestrator,
    private readonly relayPool: RelayConnectionPool
  ) {}

  /**
   * Initialize workspace: parallel fetch of git status, worktrees, file tree,
   * and pending tasks. All relay calls are offline-tolerant.
   */
  async initWorkspace(projectId: string, userId: string): Promise<WorkspaceInitResult> {
    const relay = await this.router.getRelayForProject(projectId, userId).catch(() => null)

    const [gitStatusRaw, worktreeRaw, fileTreeRaw, pendingTasks] = await Promise.all([
      relay
        ? relay.call('git.exec', { args: ['status', '--porcelain=v2', '--branch'] })
            .catch(() => null) as Promise<{ stdout?: string } | null>
        : Promise.resolve(null),

      relay
        ? relay.call('git.exec', { args: ['worktree', 'list', '--porcelain'] })
            .catch(() => null) as Promise<{ stdout?: string } | null>
        : Promise.resolve(null),

      relay
        ? relay.call('fs.readDir', { path: '.', depth: 2 })
            .catch(() => null) as Promise<FileTreeNode[] | null>
        : Promise.resolve(null),

      this.taskService
        .list({ projectId, limit: 100 })
        .then(tasks => tasks.filter(t => ['todo', 'in_progress', 'blocked'].includes(t.status)))
        .catch((): OrcaTask[] => []),
    ])

    const gitStatus = gitStatusRaw?.stdout
      ? this.parseGitStatus(gitStatusRaw.stdout)
      : null

    const worktrees = worktreeRaw?.stdout
      ? this.parseWorktreeList(worktreeRaw.stdout)
      : []

    const fileTree = Array.isArray(fileTreeRaw) ? fileTreeRaw : []

    return { gitStatus, worktrees, fileTree, pendingTasks }
  }

  /**
   * Teardown workspace: release relay connection for the project.
   * Looks up project.devServerId and calls relayPool.release().
   */
  async teardownWorkspace(projectId: string): Promise<void> {
    try {
      const project = await this.router.getProject(projectId).catch(() => null)
      if (project?.devServerId) {
        this.relayPool.release(project.devServerId)
      }
      console.log(`[WorkspaceService] teardownWorkspace: project=${projectId}`)
    } catch {
      // Non-fatal
    }
  }

  /**
   * Refresh file tree for a given path.
   */
  async refreshFileTree(
    projectId: string,
    userId: string,
    path?: string
  ): Promise<FileTreeNode[]> {
    const relay = await this.router.getRelayForProject(projectId, userId).catch(() => null)
    if (!relay) return []

    const result = await relay
      .call('fs.readDir', { path: path ?? '.', depth: 2 })
      .catch(() => null) as FileTreeNode[] | null

    return Array.isArray(result) ? result : []
  }

  /**
   * Refresh git status for a specific worktree path.
   */
  async refreshGitStatus(
    projectId: string,
    userId: string,
    worktreePath: string
  ): Promise<GitStatus | null> {
    const relay = await this.router.getRelayForProject(projectId, userId).catch(() => null)
    if (!relay) return null

    const raw = await relay
      .call('git.exec', {
        cwd: worktreePath,
        args: ['status', '--porcelain=v2', '--branch'],
      })
      .catch(() => null) as { stdout?: string } | null

    return raw?.stdout ? this.parseGitStatus(raw.stdout) : null
  }

  // ── Private helpers ────────────────────────────────────────────────────────

  /**
   * Parse `git status --porcelain=v2 --branch` output into GitStatus.
   *
   * Branch header lines start with `# branch.`. Entry lines:
   * - `1 XY ...  path` for ordinary changes
   * - `2 XY ...  path` for renamed/copied
   * - `u ...      path` for unmerged
   * - `? path` for untracked
   */
  parseGitStatus(stdout: string): GitStatus {
    const lines = stdout.split('\n').filter(Boolean)
    let branch = 'HEAD'
    let upstream: string | undefined
    let ahead = 0
    let behind = 0
    const files: GitStatusFile[] = []

    for (const line of lines) {
      if (line.startsWith('# branch.oid')) continue
      if (line.startsWith('# branch.head')) {
        branch = line.split(' ').slice(2).join(' ')
      } else if (line.startsWith('# branch.upstream')) {
        upstream = line.split(' ').slice(2).join(' ')
      } else if (line.startsWith('# branch.ab')) {
        const parts = line.split(' ')
        ahead = Math.abs(parseInt(parts[2] ?? '0', 10))
        behind = Math.abs(parseInt(parts[3] ?? '0', 10))
      } else if (line.startsWith('1 ') || line.startsWith('2 ')) {
        const parts = line.split(' ')
        const xy = parts[1] ?? '..'
        const path = parts.slice(8).join(' ')
        files.push({ path, x: xy[0] ?? '.', y: xy[1] ?? '.' })
      } else if (line.startsWith('? ')) {
        files.push({ path: line.slice(2), x: '?', y: '?' })
      }
    }

    const staged = files.filter(f => f.x !== '.' && f.x !== '?').length
    const unstaged = files.filter(f => f.y !== '.' && f.y !== '?').length
    const untracked = files.filter(f => f.x === '?').length

    return { branch, upstream, ahead, behind, staged, unstaged, untracked, files }
  }

  /**
   * Parse `git worktree list --porcelain` output into GitWorktree[].
   *
   * Each block (blank line separated) has:
   * - `worktree /path`
   * - `HEAD <sha>`
   * - `branch refs/heads/<name>` or `detached`
   * - optionally `locked [reason]`
   */
  parseWorktreeList(stdout: string): GitWorktree[] {
    const worktrees: GitWorktree[] = []
    const blocks = stdout.split('\n\n').filter(Boolean)
    let isFirst = true

    for (const block of blocks) {
      const lines = block.split('\n').filter(Boolean)
      let path = ''
      let head = ''
      let branch = 'HEAD'
      let isLocked = false

      for (const line of lines) {
        if (line.startsWith('worktree ')) path = line.slice(9)
        else if (line.startsWith('HEAD ')) head = line.slice(5)
        else if (line.startsWith('branch ')) branch = line.slice(7).replace('refs/heads/', '')
        else if (line.startsWith('locked')) isLocked = true
        else if (line === 'detached') branch = 'HEAD (detached)'
      }

      if (path) {
        worktrees.push({ path, branch, head, isMain: isFirst, isLocked })
        isFirst = false
      }
    }

    return worktrees
  }
}
