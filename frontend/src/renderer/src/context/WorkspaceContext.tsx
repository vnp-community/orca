// WorkspaceContext.tsx — Project-scoped state and micro event bus (TDD-FE-12)
import {
  createContext,
  useCallback,
  useContext,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import type { OrcaProject, FileNode, GitStatus, Worktree } from '../types/workspace-types'
import type { ResolvedProfile } from '../types/profile-types'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { useAppStore } from '../store'

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
    children: node.children?.map(mapBackendFileTreeNode),
  }
}

function toFileTree(nodes: BackendFileTreeNode[], rootPath: string): FileNode {
  return {
    name: rootPath,
    path: rootPath,
    type: 'directory',
    children: nodes.map(mapBackendFileTreeNode),
  }
}

export type WorkspaceContextValue = {
  // ── State ──────────────────────────────────────────────────────────────────
  project:              OrcaProject | null
  isOffline:            boolean
  isInitializing:       boolean
  gitStatus:            GitStatus | null
  fileTree:             FileNode | null
  resolvedProfile:      ResolvedProfile | null
  activeAgentSessionId: string | null
  currentWorktree:      Worktree | null

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
  const [project,              setProject]          = useState<OrcaProject | null>(null)
  const [isOffline,            setIsOffline]        = useState(false)
  const [isInitializing,       setIsInitializing]   = useState(false)
  const [gitStatus,            setGitStatus]        = useState<GitStatus | null>(null)
  const [fileTree,             setFileTree]         = useState<FileNode | null>(null)
  const [resolvedProfile,      setResolvedProfile]  = useState<ResolvedProfile | null>(null)
  const [activeAgentSessionId, _setActiveAgent]      = useState<string | null>(null)
  const [currentWorktree,      setCurrentWorktree]  = useState<Worktree | null>(null)

  // ── Micro event bus (stable ref — never re-renders on subscription change) ──
  const handlers = useRef<Map<string, Set<EventHandler>>>(new Map())

  const emit = useCallback((event: string, payload?: unknown) => {
    handlers.current.get(event)?.forEach(h => {
      try { h(payload) } catch { /* prevent one bad handler from blocking others */ }
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

  const switchProject = useCallback(async (projectId: string) => {
    setIsInitializing(true)
    setIsOffline(false)

    try {
      const settings = useAppStore.getState().settings
      const target = getActiveRuntimeTarget(settings)

      const [proj, gitSt, tree, profile] = await Promise.all([
        callRuntimeRpc<OrcaProject>(target, 'project.get', { projectId }),
        callRuntimeRpc<GitStatus | null>(target, 'git.status', { projectId }).catch(() => null),
        callRuntimeRpc<BackendFileTreeNode[]>(target, 'workspace.refreshFileTree', { projectId, path: '.' })
          .then(nodes => toFileTree(nodes ?? [], '.'))
          .catch(() => null),
        callRuntimeRpc<ResolvedProfile | null>(target, 'profile.getResolved', {}).catch(() => null),
      ])

      setProject(proj)
      setGitStatus(gitSt ?? null)
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
  }, [emit])

  // ── refreshGitStatus ──────────────────────────────────────────────────────

  const refreshGitStatus = useCallback(async () => {
    if (!project) {return}
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const status = await callRuntimeRpc<GitStatus>(target, 'git.status', { projectId: project.id })
      setGitStatus(status)
    } catch {
      // Silently fail — status bar will show stale data
    }
  }, [project])

  // ── refreshFileTree ───────────────────────────────────────────────────────

  const refreshFileTree = useCallback(async (dirPath?: string) => {
    if (!project) {return}
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
  }, [project])


  // ── Context value ─────────────────────────────────────────────────────────

  const value: WorkspaceContextValue = {
    project,
    isOffline,
    isInitializing,
    gitStatus,
    fileTree,
    resolvedProfile,
    activeAgentSessionId,
    currentWorktree,
    switchProject,
    refreshGitStatus,
    refreshFileTree,
    setCurrentWorktree,
    emit,
    on,
  }

  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  )
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
