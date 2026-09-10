// WorkspaceContext.tsx — Project-scoped state and micro event bus (TDD-FE-12)
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode
} from 'react'
import type { OrcaProject, FileNode, GitStatus, Worktree } from '../types/workspace-types'
import type { ResolvedProfile } from '../types/profile-types'
import type { GitStatusResult } from '../../../shared/git-status-types'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { toRuntimeWorktreeSelector } from '../runtime/runtime-worktree-selector'
import { branchName } from '../lib/git-utils'
import { useAppStore } from '../store'

// Why (CR-PW-001): git.status returns the real GitStatusResult shape (entries/head/branch as a
// raw ref/upstreamStatus), not the flat GitStatus shape GitPanel reads — mapping explicitly here
// (instead of a bare type-cast) strips the refs/heads/ prefix, pulls ahead/behind from
// upstreamStatus, and distinguishes detached HEAD (result.head set) from the underlying `git
// status` call failing outright (both head and branch empty — status.ts still returns
// "successfully" with branch: undefined when gitStreamStdout throws, see status.ts:291-308).
function toWorkspaceGitStatus(result: GitStatusResult): GitStatus {
  const entries = result.entries ?? []
  return {
    branch: result.branch ? branchName(result.branch) : undefined,
    branchUnavailable: result.branch
      ? undefined
      : result.head
        ? 'detached-head'
        : 'status-unavailable',
    aheadBy: result.upstreamStatus?.ahead ?? 0,
    behindBy: result.upstreamStatus?.behind ?? 0,
    hasUncommitted: entries.length > 0,
    staged: entries.filter((e) => e.area === 'staged').length,
    unstaged: entries.filter((e) => e.area === 'unstaged').length
  }
}

// ─── Types ────────────────────────────────────────────────────────────────────

type EventHandler = (payload: unknown) => void

// ─── Backend↔FileNode adapter ──────────────────────────────────────────────
// `workspace.refreshFileTree` returns a flat array of the requested dir's
// entries (`isDir`-shaped), not a single rooted `FileNode` (`type`-shaped) —
// map each entry and synthesize a root node for the requested path.

type BackendFileTreeNode = {
  name: string
  path: string
  isDir: boolean
  children?: BackendFileTreeNode[]
}

function mapBackendFileTreeNode(node: BackendFileTreeNode): FileNode {
  return {
    name: node.name,
    path: node.path,
    type: node.isDir ? 'directory' : 'file',
    children: node.children?.map(mapBackendFileTreeNode)
  }
}

function toFileTree(nodes: BackendFileTreeNode[], rootPath: string): FileNode {
  return {
    name: rootPath,
    path: rootPath,
    type: 'directory',
    children: nodes.map(mapBackendFileTreeNode)
  }
}

export type WorkspaceContextValue = {
  // ── State ──────────────────────────────────────────────────────────────────
  project: OrcaProject | null
  isOffline: boolean
  isInitializing: boolean
  gitStatus: GitStatus | null
  /** True when the last `git.status` RPC itself threw (network/relay failure) — distinct from a
   *  successful response with no branch (see GitStatus.branchUnavailable). */
  gitStatusError: boolean
  fileTree: FileNode | null
  resolvedProfile: ResolvedProfile | null
  activeAgentSessionId: string | null
  currentWorktree: Worktree | null

  // ── Actions ────────────────────────────────────────────────────────────────
  switchProject: (projectId: string) => Promise<void>
  refreshGitStatus: () => Promise<void>
  refreshFileTree: (dirPath?: string) => Promise<void>
  setCurrentWorktree: (worktree: Worktree | null) => void

  // ── Micro event bus ────────────────────────────────────────────────────────
  /** Fire an event. All subscribers in the same WorkspaceProvider receive it. */
  emit: (event: string, payload?: unknown) => void
  /**
   * Subscribe to an event. Returns an unsubscribe function.
   * Call the returned function (or call it inside useEffect cleanup) to avoid leaks.
   */
  on: (event: string, handler: EventHandler) => () => void
}

// ─── Context ──────────────────────────────────────────────────────────────────

export const WorkspaceContext = createContext<WorkspaceContextValue>(null!)

// ─── Provider ─────────────────────────────────────────────────────────────────

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const [project, setProject] = useState<OrcaProject | null>(null)
  const [isOffline, setIsOffline] = useState(false)
  const [isInitializing, setIsInitializing] = useState(false)
  const [gitStatus, setGitStatus] = useState<GitStatus | null>(null)
  const [gitStatusError, setGitStatusError] = useState(false)
  const [fileTree, setFileTree] = useState<FileNode | null>(null)
  const [resolvedProfile, setResolvedProfile] = useState<ResolvedProfile | null>(null)
  const [activeAgentSessionId, _setActiveAgent] = useState<string | null>(null)
  const [currentWorktree, setCurrentWorktree] = useState<Worktree | null>(null)

  // ── Micro event bus (stable ref — never re-renders on subscription change) ──
  const handlers = useRef<Map<string, Set<EventHandler>>>(new Map())

  const emit = useCallback((event: string, payload?: unknown) => {
    handlers.current.get(event)?.forEach((h) => {
      try {
        h(payload)
      } catch {
        /* prevent one bad handler from blocking others */
      }
    })
  }, [])

  const on = useCallback((event: string, handler: EventHandler): (() => void) => {
    if (!handlers.current.has(event)) {
      handlers.current.set(event, new Set())
    }
    handlers.current.get(event)!.add(handler)
    return () => {
      handlers.current.get(event)?.delete(handler)
    }
  }, [])

  // ── switchProject ──────────────────────────────────────────────────────────

  const switchProject = useCallback(
    async (projectId: string) => {
      setIsInitializing(true)
      setIsOffline(false)

      try {
        const settings = useAppStore.getState().settings
        const target = getActiveRuntimeTarget(settings)

        // `git.status` needs a real `worktree` selector, which isn't known yet on
        // project switch (worktree selection happens separately via the sidebar) —
        // reset to null here and let the `currentWorktree` effect below fetch it.
        const [proj, tree, profile] = await Promise.all([
          callRuntimeRpc<OrcaProject>(target, 'project.get', { projectId }),
          callRuntimeRpc<BackendFileTreeNode[]>(target, 'workspace.refreshFileTree', {
            projectId,
            path: '.'
          })
            .then((nodes) => toFileTree(nodes ?? [], '.'))
            .catch(() => null),
          callRuntimeRpc<ResolvedProfile | null>(target, 'profile.getResolved', {}).catch(
            () => null
          )
        ])

        setProject(proj)
        setGitStatus(null)
        setGitStatusError(false)
        setFileTree(tree ?? null)
        setResolvedProfile(profile ?? null)

        emit('project.switched', { projectId })
      } catch (err: unknown) {
        const code = (err as { code?: string })?.code
        if (code === 'DEV_SERVER_UNREACHABLE' || code === 'RUNTIME_UNAVAILABLE') {
          setIsOffline(true)
        } else {
          throw err
        }
      } finally {
        setIsInitializing(false)
      }
    },
    [emit]
  )

  // ── refreshGitStatus ──────────────────────────────────────────────────────
  // `git.status` is worktree-scoped (schema: { worktree: string }), not project-scoped —
  // requires a selected worktree (F38 roadmap 2c.2).

  const refreshGitStatus = useCallback(async () => {
    if (!currentWorktree) {
      return
    }
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const status = await callRuntimeRpc<GitStatusResult>(target, 'git.status', {
        worktree: toRuntimeWorktreeSelector(currentWorktree.id)
      })
      setGitStatus(toWorkspaceGitStatus(status))
      setGitStatusError(false)
    } catch {
      // CR-PW-001: no longer swallowed completely silently — GitPanel needs to tell "the RPC
      // itself failed" apart from "status succeeded but reported no branch" (branchUnavailable).
      setGitStatusError(true)
    }
  }, [currentWorktree])

  // Re-fetch git status whenever the selected worktree changes (sidebar sync, TDD-19c §2c.1).
  useEffect(() => {
    if (currentWorktree) {
      void refreshGitStatus()
    } else {
      setGitStatus(null)
      setGitStatusError(false)
    }
  }, [currentWorktree, refreshGitStatus])

  // ── refreshFileTree ───────────────────────────────────────────────────────

  const refreshFileTree = useCallback(
    async (dirPath?: string) => {
      if (!project) {
        return
      }
      try {
        const target = getActiveRuntimeTarget(useAppStore.getState().settings)
        const resolvedPath = dirPath ?? '.'
        const nodes = await callRuntimeRpc<BackendFileTreeNode[]>(
          target,
          'workspace.refreshFileTree',
          { projectId: project.id, path: resolvedPath }
        )
        setFileTree(toFileTree(nodes ?? [], resolvedPath))
      } catch {
        // Silently fail — stale tree remains visible
      }
    },
    [project]
  )

  // ── Context value ─────────────────────────────────────────────────────────
  // Why useMemo: a plain object literal here would give every consumer a new
  // `value` reference on every WorkspaceProvider render, defeating context
  // consumers' own memoization (e.g. React.memo children re-rendering
  // unconditionally).

  const value: WorkspaceContextValue = useMemo(
    () => ({
      project,
      isOffline,
      isInitializing,
      gitStatus,
      gitStatusError,
      fileTree,
      resolvedProfile,
      activeAgentSessionId,
      currentWorktree,
      switchProject,
      refreshGitStatus,
      refreshFileTree,
      setCurrentWorktree,
      emit,
      on
    }),
    [
      project,
      isOffline,
      isInitializing,
      gitStatus,
      gitStatusError,
      fileTree,
      resolvedProfile,
      activeAgentSessionId,
      currentWorktree,
      switchProject,
      refreshGitStatus,
      refreshFileTree,
      setCurrentWorktree,
      emit,
      on
    ]
  )

  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

/**
 * Access the WorkspaceContext value.
 * Throws if called outside <WorkspaceProvider>.
 */
export function useWorkspace(): WorkspaceContextValue {
  const ctx = useContext(WorkspaceContext)
  if (ctx === null) {
    throw new Error('useWorkspace must be used within <WorkspaceProvider>')
  }
  return ctx
}
