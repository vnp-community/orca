// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, act, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import {
  WorkspaceProvider,
  WorkspaceContext,
  type WorkspaceContextValue
} from '../WorkspaceContext'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { useContext } from 'react'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
}))

vi.mock('../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

function renderContext(callback: (val: WorkspaceContextValue) => void) {
  const TestComponent = () => {
    const val = useContext(WorkspaceContext)
    callback(val)
    return null
  }
  return render(
    <WorkspaceProvider>
      <TestComponent />
    </WorkspaceProvider>
  )
}

describe('WorkspaceContext', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(callRuntimeRpc).mockImplementation(async (_, method) => {
      if (method === 'project.get') {
        return { id: 'p1', name: 'Proj 1' }
      }
      if (method === 'git.status') {
        return { branch: 'main' }
      }
      if (method === 'workspace.refreshFileTree') {
        return []
      }
      if (method === 'profile.getResolved') {
        return { security: {} }
      }
      return {}
    })
  })

  afterEach(cleanup)

  it('switchProject() calls multiple RPCs and sets project state', async () => {
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })

    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.get', {
      projectId: 'p1'
    })
    expect(ctxValue.project).toEqual({ id: 'p1', name: 'Proj 1' })
    // git.status is worktree-scoped, not project-scoped — no worktree is selected yet
    // right after switchProject(), so gitStatus stays null until setCurrentWorktree().
    expect(ctxValue.gitStatus).toBeNull()
    expect(callRuntimeRpc).not.toHaveBeenCalledWith(
      expect.anything(),
      'git.status',
      expect.anything()
    )
  })

  it('switchProject() sets isOffline=true on DEV_SERVER_UNREACHABLE error', async () => {
    vi.mocked(callRuntimeRpc).mockRejectedValue({ code: 'DEV_SERVER_UNREACHABLE' })
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    await act(async () => {
      await ctxValue.switchProject('p1').catch(() => {})
    })

    expect(ctxValue.isOffline).toBe(true)
  })

  it('refreshGitStatus() is a no-op without a selected worktree', async () => {
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })

    await act(async () => {
      await ctxValue.refreshGitStatus()
    })

    expect(callRuntimeRpc).not.toHaveBeenCalledWith(
      expect.anything(),
      'git.status',
      expect.anything()
    )
    expect(ctxValue.gitStatus).toBeNull()
  })

  it('setCurrentWorktree() fetches git.status with the worktree selector and updates gitStatus', async () => {
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })

    // Why the full shape, not just { branch: 'feature' }: CR-PW-001 maps the real
    // GitStatusResult (entries/head/branch raw ref/upstreamStatus) into GitStatus explicitly
    // instead of a bare type-cast — the mocked RPC response only needs `branch`, but the
    // normalized context value always carries every GitStatus field.
    vi.mocked(callRuntimeRpc).mockResolvedValue({ branch: 'feature', entries: [] })
    await act(async () => {
      ctxValue.setCurrentWorktree({
        id: 'repo1::/wt',
        repoId: 'repo1',
        projectId: 'p1',
        hostId: 'local',
        isMainWorktree: true
      })
    })

    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'git.status', {
      worktree: 'id:repo1::/wt'
    })
    expect(ctxValue.gitStatus).toEqual({
      branch: 'feature',
      branchUnavailable: undefined,
      aheadBy: 0,
      behindBy: 0,
      hasUncommitted: false,
      staged: 0,
      unstaged: 0
    })
  })

  it('clearing currentWorktree resets gitStatus to null', async () => {
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })
    vi.mocked(callRuntimeRpc).mockResolvedValue({ branch: 'feature', entries: [] })
    await act(async () => {
      ctxValue.setCurrentWorktree({
        id: 'repo1::/wt',
        repoId: 'repo1',
        projectId: 'p1',
        hostId: 'local',
        isMainWorktree: true
      })
    })
    expect(ctxValue.gitStatus?.branch).toEqual('feature')

    await act(async () => {
      ctxValue.setCurrentWorktree(null)
    })
    expect(ctxValue.gitStatus).toBeNull()
  })

  it('sets gitStatusError=true when git.status itself throws, distinct from a successful no-branch response', async () => {
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })

    vi.mocked(callRuntimeRpc).mockRejectedValue(new Error('relay unreachable'))
    await act(async () => {
      ctxValue.setCurrentWorktree({
        id: 'repo1::/wt',
        repoId: 'repo1',
        projectId: 'p1',
        hostId: 'local',
        isMainWorktree: true
      })
    })

    expect(ctxValue.gitStatusError).toBe(true)
  })

  it('refreshFileTree() calls workspace.refreshFileTree and maps the flat entry list into a rooted fileTree', async () => {
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })

    vi.mocked(callRuntimeRpc).mockResolvedValue([
      { name: 'updated.txt', path: 'updated.txt', isDir: false }
    ])
    await act(async () => {
      await ctxValue.refreshFileTree()
    })

    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'workspace.refreshFileTree', {
      projectId: 'p1',
      path: '.'
    })
    expect(ctxValue.fileTree).toEqual({
      name: '.',
      path: '.',
      type: 'directory',
      children: [{ name: 'updated.txt', path: 'updated.txt', type: 'file', children: undefined }]
    })
  })

  it('emit(event, data) + on(event, handler) delivers data to handler', () => {
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    const handler = vi.fn()
    act(() => {
      ctxValue.on('test.event', handler)
      ctxValue.emit('test.event', { foo: 'bar' })
    })

    expect(handler).toHaveBeenCalledWith({ foo: 'bar' })
  })

  it('on() returns cleanup function that unsubscribes handler', () => {
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    const handler = vi.fn()
    act(() => {
      const unsub = ctxValue.on('test.event', handler)
      unsub()
      ctxValue.emit('test.event', { foo: 'bar' })
    })

    expect(handler).not.toHaveBeenCalled()
  })

  it('agent.complete event registered listener is called', () => {
    // We already tested emit/on, this satisfies the requirement of testing event bus mechanism
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    const handler = vi.fn()
    act(() => {
      ctxValue.on('agent.complete', handler)
      ctxValue.emit('agent.complete', { agentId: 'a1' })
    })

    expect(handler).toHaveBeenCalledWith({ agentId: 'a1' })
  })

  it('isInitializing is false after successful switchProject', async () => {
    let ctxValue!: WorkspaceContextValue
    renderContext((val) => {
      ctxValue = val
    })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })

    expect(ctxValue.isInitializing).toBe(false)
  })
})
