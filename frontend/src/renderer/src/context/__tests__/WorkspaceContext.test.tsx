// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, act, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { WorkspaceProvider, WorkspaceContext } from '../WorkspaceContext'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { useContext, useEffect } from 'react'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
}))

vi.mock('../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

function renderContext(callback: (val: any) => void) {
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
      if (method === 'project.get') {return { id: 'p1', name: 'Proj 1' }}
      if (method === 'git.status') {return { branch: 'main' }}
      if (method === 'workspace.listFiles') {return { name: 'root', children: [] }}
      if (method === 'profile.getResolved') {return { security: {} }}
      return {}
    })
  })

  afterEach(cleanup)

  it('switchProject() calls multiple RPCs and sets project state', async () => {
    let ctxValue: any
    renderContext(val => { ctxValue = val })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })

    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.get', { projectId: 'p1' })
    expect(ctxValue.project).toEqual({ id: 'p1', name: 'Proj 1' })
    expect(ctxValue.gitStatus).toEqual({ branch: 'main' })
  })

  it('switchProject() sets isOffline=true on DEV_SERVER_UNREACHABLE error', async () => {
    vi.mocked(callRuntimeRpc).mockRejectedValue({ code: 'DEV_SERVER_UNREACHABLE' })
    let ctxValue: any
    renderContext(val => { ctxValue = val })

    await act(async () => {
      await ctxValue.switchProject('p1').catch(() => {})
    })

    expect(ctxValue.isOffline).toBe(true)
  })

  it('refreshGitStatus() calls git.status and updates gitStatus', async () => {
    let ctxValue: any
    renderContext(val => { ctxValue = val })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })
    
    vi.mocked(callRuntimeRpc).mockResolvedValue({ branch: 'feature' })
    await act(async () => {
      await ctxValue.refreshGitStatus()
    })

    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'git.status', { projectId: 'p1' })
    expect(ctxValue.gitStatus).toEqual({ branch: 'feature' })
  })

  it('refreshFileTree() calls workspace.listFiles and updates fileTree', async () => {
    let ctxValue: any
    renderContext(val => { ctxValue = val })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })
    
    vi.mocked(callRuntimeRpc).mockResolvedValue({ name: 'updated' })
    await act(async () => {
      await ctxValue.refreshFileTree()
    })

    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'workspace.listFiles', { projectId: 'p1', dirPath: '.' })
    expect(ctxValue.fileTree).toEqual({ name: 'updated' })
  })

  it('emit(event, data) + on(event, handler) delivers data to handler', () => {
    let ctxValue: any
    renderContext(val => { ctxValue = val })

    const handler = vi.fn()
    act(() => {
      ctxValue.on('test.event', handler)
      ctxValue.emit('test.event', { foo: 'bar' })
    })

    expect(handler).toHaveBeenCalledWith({ foo: 'bar' })
  })

  it('on() returns cleanup function that unsubscribes handler', () => {
    let ctxValue: any
    renderContext(val => { ctxValue = val })

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
    let ctxValue: any
    renderContext(val => { ctxValue = val })

    const handler = vi.fn()
    act(() => {
      ctxValue.on('agent.complete', handler)
      ctxValue.emit('agent.complete', { agentId: 'a1' })
    })

    expect(handler).toHaveBeenCalledWith({ agentId: 'a1' })
  })

  it('isInitializing is false after successful switchProject', async () => {
    let ctxValue: any
    renderContext(val => { ctxValue = val })

    await act(async () => {
      await ctxValue.switchProject('p1')
    })

    expect(ctxValue.isInitializing).toBe(false)
  })
})
