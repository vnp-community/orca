// @vitest-environment happy-dom
/**
 * Tests for WorkspaceContextV6 + useWorkspaceV6 (TDD-19) — T17
 *
 * Uses Testing Library + vi.fn() for RPC call injection.
 * ≥ 10 tests.
 *
 * Note: Tests use WorkspaceProviderV6 and useWorkspaceV6 (not v5 WorkspaceContext)
 */

import { describe, it, expect, vi } from 'vitest'
import { act, waitFor } from '@testing-library/react'
import { renderHook } from '@testing-library/react'
import { WorkspaceProviderV6, useWorkspaceV6 } from '../WorkspaceContextV6'
import type { ReactNode } from 'react'

// ── Helpers ───────────────────────────────────────────────────────────────────

const FAKE_INIT_RESULT = {
  gitStatus: { branch: 'main', ahead: 0, behind: 0, staged: 0, unstaged: 0, untracked: 0, files: [] },
  worktrees: [{ path: '/repo', branch: 'main', head: 'abc', isMain: true, isLocked: false }],
  fileTree: [{ name: 'src', path: 'src', isDir: true, children: [] }],
  pendingTasks: [{ id: 't1', status: 'todo', title: 'Task A', type: 'task', priority: 'high', reporterId: 'u1', visibility: 'team', progressPercent: 0, labels: [], createdAt: new Date(), updatedAt: new Date() }],
}

function makeRpcCall(overrides: Record<string, unknown> = {}) {
  return vi.fn().mockImplementation(async (method: string) => {
    if (method === 'workspace.init') {return overrides['workspace.init'] ?? FAKE_INIT_RESULT}
    if (method === 'workspace.teardown') {return overrides['workspace.teardown'] ?? undefined}
    if (method === 'workspace.refreshGitStatus') {return overrides['workspace.refreshGitStatus'] ?? FAKE_INIT_RESULT.gitStatus}
    if (method === 'workspace.refreshFileTree') {return overrides['workspace.refreshFileTree'] ?? FAKE_INIT_RESULT.fileTree}
    return null
  })
}

function makeWrapper(rpcCall: ReturnType<typeof makeRpcCall>) {
  return ({ children }: { children: ReactNode }) => (
    <WorkspaceProviderV6 rpcCall={rpcCall}>{children}</WorkspaceProviderV6>
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('WorkspaceContextV6', () => {

  // ── switchProject ───────────────────────────────────────────────────────────
  describe('switchProject', () => {
    it('calls workspace.init RPC and populates gitStatus/worktrees/fileTree/tasks', async () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspaceV6(), { wrapper: makeWrapper(rpc) })
      await act(async () => {
        await result.current.switchProject('proj-001')
      })
      expect(result.current.gitStatus?.branch).toBe('main')
      expect(result.current.worktrees).toHaveLength(1)
      expect(result.current.pendingTasks).toHaveLength(1)
    })

    it('sets projectId after switch', async () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspaceV6(), { wrapper: makeWrapper(rpc) })
      await act(async () => { await result.current.switchProject('proj-ABC') })
      expect(result.current.projectId).toBe('proj-ABC')
    })

    it('sets isOffline=true on DEV_SERVER_UNREACHABLE error', async () => {
      const rpc = vi.fn().mockRejectedValue(new Error('DEV_SERVER_UNREACHABLE'))
      const { result } = renderHook(() => useWorkspaceV6(), { wrapper: makeWrapper(rpc) })
      await act(async () => {
        await result.current.switchProject('proj-offline')
      })
      expect(result.current.isOffline).toBe(true)
    })

    it('sets isInitializing=true during RPC call', async () => {
      let resolveInit!: (value: unknown) => void
      const rpc = vi.fn().mockReturnValue(new Promise(r => { resolveInit = r }))
      const { result } = renderHook(() => useWorkspaceV6(), { wrapper: makeWrapper(rpc) })

      act(() => { void result.current.switchProject('proj-001') })
      expect(result.current.isInitializing).toBe(true)

      await act(async () => { resolveInit(FAKE_INIT_RESULT) })
      expect(result.current.isInitializing).toBe(false)
    })

    it('tears down previous project before switching to new one', async () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspaceV6(), { wrapper: makeWrapper(rpc) })
      await act(async () => { await result.current.switchProject('proj-A') })
      await act(async () => { await result.current.switchProject('proj-B') })
      expect(rpc).toHaveBeenCalledWith('workspace.teardown', { projectId: 'proj-A' })
    })
  })

  // ── Event bus (emit + on) ───────────────────────────────────────────────────
  describe('event bus', () => {
    it('on() registers handler and emit() calls it with correct event', () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspaceV6(), { wrapper: makeWrapper(rpc) })
      const handler = vi.fn()

      act(() => { result.current.on('agent.complete', handler) })
      act(() => { result.current.emit({ type: 'agent.complete', sessionId: 'sess-1' }) })

      expect(handler).toHaveBeenCalledWith({ type: 'agent.complete', sessionId: 'sess-1' })
    })

    it('on() returns unsubscribe function — handler NOT called after unsub', () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspaceV6(), { wrapper: makeWrapper(rpc) })
      const handler = vi.fn()

      let unsub!: () => void
      act(() => { unsub = result.current.on('git.push', handler) })
      act(() => { unsub() }) // unsubscribe
      act(() => { result.current.emit({ type: 'git.push', projectId: 'proj-001' }) })

      expect(handler).not.toHaveBeenCalled()
    })

    it('multiple handlers for same event type all called', () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspaceV6(), { wrapper: makeWrapper(rpc) })
      const h1 = vi.fn()
      const h2 = vi.fn()

      act(() => {
        result.current.on('file.change', h1)
        result.current.on('file.change', h2)
        result.current.emit({ type: 'file.change', path: 'src/index.ts' })
      })

      expect(h1).toHaveBeenCalledTimes(1)
      expect(h2).toHaveBeenCalledTimes(1)
    })
  })

  // ── auto-refresh on agent.complete ──────────────────────────────────────────
  describe('auto-refresh effects', () => {
    it('agent.complete event triggers refreshGitStatus call', async () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspaceV6(), { wrapper: makeWrapper(rpc) })
      await act(async () => { await result.current.switchProject('proj-001') })
      rpc.mockClear()

      await act(async () => {
        result.current.emit({ type: 'agent.complete', sessionId: 'sess-001' })
      })

      await waitFor(() => {
        expect(rpc).toHaveBeenCalledWith('workspace.refreshGitStatus', expect.objectContaining({ projectId: 'proj-001' }))
      })
    })
  })

  // ── useWorkspaceV6 guard ──────────────────────────────────────────────────
  it('throws when used outside WorkspaceProviderV6', () => {
    // Suppress React's console.error for uncaught thrown errors in renderHook
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    try {
      const { result } = renderHook(() => useWorkspaceV6())
      // If we reach here, the error was captured in result.error
      expect(result.error?.message).toContain('WorkspaceProviderV6')
    } catch (err) {
      // renderHook may throw directly in some versions
      expect((err as Error).message).toContain('WorkspaceProviderV6')
    } finally {
      spy.mockRestore()
    }
  })
})
