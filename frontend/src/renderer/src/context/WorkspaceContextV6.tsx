/**
 * WorkspaceContext — Project workspace state + event bus (TDD-19 §4)
 *
 * Provides:
 * - switchProject(projectId): init workspace, teardown previous
 * - gitStatus, worktrees, fileTree, pendingTasks
 * - Micro event bus: emit/on for agent.complete, git.push, file.change events
 * - isInitializing, isOffline flags
 *
 * @module renderer/context/WorkspaceContextV6
 */

import { createContext, useContext, useState, useCallback, useRef, useEffect, type ReactNode } from 'react'
import type { GitStatus, GitWorktree, FileTreeNode } from '../../../main/workspace/WorkspaceService'
import type { OrcaTask } from '../../../shared/task-types'

// ── Event bus types ────────────────────────────────────────────────────────────

export type WorkspaceEvent =
  | { type: 'agent.complete'; sessionId: string; taskId?: string }
  | { type: 'git.push'; projectId: string }
  | { type: 'git.commit'; projectId: string; message: string }
  | { type: 'file.change'; path: string }

export type EventHandler<T extends WorkspaceEvent = WorkspaceEvent> = (event: T) => void

// ── Context interface ──────────────────────────────────────────────────────────

export type WorkspaceStateV6 = {
  projectId: string | null
  gitStatus: GitStatus | null
  worktrees: GitWorktree[]
  fileTree: FileTreeNode[]
  pendingTasks: OrcaTask[]
  isInitializing: boolean
  isOffline: boolean
}

export type WorkspaceContextV6Value = {
  switchProject: (projectId: string) => Promise<void>
  refreshGitStatus: (worktreePath?: string) => Promise<void>
  refreshFileTree: (path?: string) => Promise<void>
  emit: <T extends WorkspaceEvent>(event: T) => void
  on: <T extends WorkspaceEvent>(
    eventType: T['type'],
    handler: EventHandler<T>
  ) => () => void // returns unsubscribe
} & WorkspaceStateV6

// ── Context ────────────────────────────────────────────────────────────────────

export const WorkspaceContextV6 = createContext<WorkspaceContextV6Value | null>(null)

// ── Provider ───────────────────────────────────────────────────────────────────

type WorkspaceProviderV6Props = {
  children: ReactNode
  /** Injected RPC call function (for testability) */
  rpcCall?: (method: string, params: unknown) => Promise<unknown>
}

export function WorkspaceProviderV6({ children, rpcCall }: WorkspaceProviderV6Props) {
  const [state, setState] = useState<WorkspaceStateV6>({
    projectId: null,
    gitStatus: null,
    worktrees: [],
    fileTree: [],
    pendingTasks: [],
    isInitializing: false,
    isOffline: false,
  })

  // Micro event bus
  const handlersRef = useRef(new Map<string, Set<EventHandler>>())
  const currentProjectRef = useRef<string | null>(null)

  // Use injected rpcCall or window.__orcaRpc (Electron IPC bridge)
  const call = useCallback(async (method: string, params: unknown) => {
    if (rpcCall) {return rpcCall(method, params)}
    const bridge = (window as any).__orcaRpc
    if (!bridge) {throw new Error('RPC bridge not available')}
    return bridge.call(method, params)
  }, [rpcCall])

  // ── Event bus ───────────────────────────────────────────────────────────────

  const emit = useCallback(<T extends WorkspaceEvent>(event: T) => {
    const handlers = handlersRef.current.get(event.type)
    if (handlers) {
      for (const handler of handlers) {
        handler(event)
      }
    }
  }, [])

  const on = useCallback(<T extends WorkspaceEvent>(
    eventType: T['type'],
    handler: EventHandler<T>
  ): (() => void) => {
    if (!handlersRef.current.has(eventType)) {
      handlersRef.current.set(eventType, new Set())
    }
    handlersRef.current.get(eventType)!.add(handler as EventHandler)
    // Return unsubscribe function
    return () => {
      handlersRef.current.get(eventType)?.delete(handler as EventHandler)
    }
  }, [])

  // ── switchProject ────────────────────────────────────────────────────────────

  const switchProject = useCallback(async (projectId: string) => {
    // Teardown previous project
    if (currentProjectRef.current && currentProjectRef.current !== projectId) {
      await call('workspace.teardown', { projectId: currentProjectRef.current }).catch(() => {})
    }

    setState(s => ({ ...s, isInitializing: true, isOffline: false, projectId }))
    currentProjectRef.current = projectId

    try {
      const result = await call('workspace.init', { projectId }) as {
        gitStatus: GitStatus | null
        worktrees: GitWorktree[]
        fileTree: FileTreeNode[]
        pendingTasks: OrcaTask[]
      }
      setState(s => ({
        ...s,
        isInitializing: false,
        gitStatus: result.gitStatus,
        worktrees: result.worktrees,
        fileTree: result.fileTree,
        pendingTasks: result.pendingTasks,
      }))
    } catch (err) {
      const isOffline = (err as Error).message?.includes('DEV_SERVER_UNREACHABLE')
      setState(s => ({ ...s, isInitializing: false, isOffline }))
    }
  }, [call])

  // ── refreshGitStatus ─────────────────────────────────────────────────────────

  const refreshGitStatus = useCallback(async (worktreePath?: string) => {
    if (!currentProjectRef.current) {return}
    const result = await call('workspace.refreshGitStatus', {
      projectId: currentProjectRef.current,
      worktreePath,
    }).catch(() => null) as GitStatus | null
    if (result) {setState(s => ({ ...s, gitStatus: result }))}
  }, [call])

  // ── refreshFileTree ──────────────────────────────────────────────────────────

  const refreshFileTree = useCallback(async (path?: string) => {
    if (!currentProjectRef.current) {return}
    const result = await call('workspace.refreshFileTree', {
      projectId: currentProjectRef.current,
      path,
    }).catch(() => []) as FileTreeNode[]
    setState(s => ({ ...s, fileTree: result }))
  }, [call])

  // ── Auto-refresh on agent.complete ────────────────────────────────────────────

  useEffect(() => {
    const unsub = on('agent.complete', async () => {
      await refreshGitStatus()
    })
    return unsub
  }, [on, refreshGitStatus])

  const value: WorkspaceContextV6Value = {
    ...state,
    switchProject,
    refreshGitStatus,
    refreshFileTree,
    emit,
    on,
  }

  return <WorkspaceContextV6.Provider value={value}>{children}</WorkspaceContextV6.Provider>
}

// ── Hook ───────────────────────────────────────────────────────────────────────

export function useWorkspaceV6(): WorkspaceContextV6Value {
  const ctx = useContext(WorkspaceContextV6)
  if (!ctx) {throw new Error('useWorkspaceV6 must be used within WorkspaceProviderV6')}
  return ctx
}
