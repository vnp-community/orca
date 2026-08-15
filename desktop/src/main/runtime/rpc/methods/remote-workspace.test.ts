import { describe, expect, it, vi, beforeEach } from 'vitest'
import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'

const {
  getRemoteWorkspaceClientIdMock,
  getRemoteWorkspaceForTargetMock,
  listConnectedRemoteWorkspaceClientsMock,
  listEnabledConnectedRemoteWorkspaceTargetsMock,
  setRemoteWorkspaceForConnectedTargetsMock,
  onRemoteWorkspaceChangedMock
} = vi.hoisted(() => ({
  getRemoteWorkspaceClientIdMock: vi.fn(),
  getRemoteWorkspaceForTargetMock: vi.fn(),
  listConnectedRemoteWorkspaceClientsMock: vi.fn(),
  listEnabledConnectedRemoteWorkspaceTargetsMock: vi.fn(),
  setRemoteWorkspaceForConnectedTargetsMock: vi.fn(),
  onRemoteWorkspaceChangedMock: vi.fn()
}))

vi.mock('../../../ipc/remote-workspace', () => ({
  getRemoteWorkspaceClientId: getRemoteWorkspaceClientIdMock,
  getRemoteWorkspaceForTarget: getRemoteWorkspaceForTargetMock,
  listConnectedRemoteWorkspaceClients: listConnectedRemoteWorkspaceClientsMock,
  listEnabledConnectedRemoteWorkspaceTargets: listEnabledConnectedRemoteWorkspaceTargetsMock,
  setRemoteWorkspaceForConnectedTargets: setRemoteWorkspaceForConnectedTargetsMock
}))

vi.mock('../../../ipc/remote-workspace-change-bus', () => ({
  onRemoteWorkspaceChanged: onRemoteWorkspaceChangedMock
}))

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

function makeStoreRuntime(): OrcaRuntimeService {
  return {
    getRuntimeId: () => 'test-runtime',
    getRuntimeStoreForRpc: vi.fn().mockReturnValue({}),
    registerSubscriptionCleanup: vi.fn()
  } as unknown as OrcaRuntimeService
}

describe('remote workspace RPC methods', () => {
  beforeEach(() => {
    getRemoteWorkspaceClientIdMock.mockReset()
    getRemoteWorkspaceForTargetMock.mockReset()
    listConnectedRemoteWorkspaceClientsMock.mockReset()
    listEnabledConnectedRemoteWorkspaceTargetsMock.mockReset()
    setRemoteWorkspaceForConnectedTargetsMock.mockReset()
    onRemoteWorkspaceChangedMock.mockReset()
  })

  it('gets a remote workspace snapshot for a target', async () => {
    getRemoteWorkspaceForTargetMock.mockResolvedValue({ namespace: 'ns', revision: 1 })
    const runtime = makeStoreRuntime()
    const { REMOTE_WORKSPACE_METHODS } = await import('./remote-workspace')
    const dispatcher = new RpcDispatcher({ runtime, methods: REMOTE_WORKSPACE_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('remoteWorkspace.get', { targetId: 'target-1' })
    )

    expect(getRemoteWorkspaceForTargetMock).toHaveBeenCalledWith({ targetId: 'target-1' })
    expect(response).toMatchObject({ ok: true, result: { revision: 1 } })
  })

  it('sets the workspace session for connected targets using the RPC-scoped store', async () => {
    setRemoteWorkspaceForConnectedTargetsMock.mockResolvedValue([])
    const runtime = makeStoreRuntime()
    const { REMOTE_WORKSPACE_METHODS } = await import('./remote-workspace')
    const dispatcher = new RpcDispatcher({ runtime, methods: REMOTE_WORKSPACE_METHODS })

    await dispatcher.dispatch(
      makeRequest('remoteWorkspace.setForConnectedTargets', { hydratedTargetIds: ['t-1'] })
    )

    expect(setRemoteWorkspaceForConnectedTargetsMock).toHaveBeenCalledWith(
      {},
      expect.objectContaining({ hydratedTargetIds: ['t-1'] })
    )
  })

  it('lists enabled connected targets', async () => {
    listEnabledConnectedRemoteWorkspaceTargetsMock.mockResolvedValue(['t-1'])
    const runtime = makeStoreRuntime()
    const { REMOTE_WORKSPACE_METHODS } = await import('./remote-workspace')
    const dispatcher = new RpcDispatcher({ runtime, methods: REMOTE_WORKSPACE_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('remoteWorkspace.listEnabledConnectedTargets')
    )

    expect(response).toMatchObject({ ok: true, result: ['t-1'] })
  })

  it('lists connected clients', async () => {
    listConnectedRemoteWorkspaceClientsMock.mockResolvedValue([{ targetId: 't-1', clients: [] }])
    const runtime = makeStoreRuntime()
    const { REMOTE_WORKSPACE_METHODS } = await import('./remote-workspace')
    const dispatcher = new RpcDispatcher({ runtime, methods: REMOTE_WORKSPACE_METHODS })

    const response = await dispatcher.dispatch(makeRequest('remoteWorkspace.listConnectedClients'))

    expect(response).toMatchObject({ ok: true, result: [{ targetId: 't-1' }] })
  })

  it('returns the local remote-workspace client id', async () => {
    getRemoteWorkspaceClientIdMock.mockReturnValue('client-1')
    const runtime = makeStoreRuntime()
    const { REMOTE_WORKSPACE_METHODS } = await import('./remote-workspace')
    const dispatcher = new RpcDispatcher({ runtime, methods: REMOTE_WORKSPACE_METHODS })

    const response = await dispatcher.dispatch(makeRequest('remoteWorkspace.clientId'))

    expect(response).toMatchObject({ ok: true, result: 'client-1' })
  })

  it('streams changed events until cleaned up', async () => {
    let emitChanged: ((event: unknown) => void) | undefined
    onRemoteWorkspaceChangedMock.mockImplementation((listener: (event: unknown) => void) => {
      emitChanged = listener
      return vi.fn()
    })
    const cleanups = new Map<string, () => void>()
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      registerSubscriptionCleanup: vi.fn((id, cleanup) => cleanups.set(id, cleanup))
    } as unknown as OrcaRuntimeService
    const { REMOTE_WORKSPACE_METHODS } = await import('./remote-workspace')
    const dispatcher = new RpcDispatcher({ runtime, methods: REMOTE_WORKSPACE_METHODS })
    const replies: unknown[] = []

    const dispatch = dispatcher.dispatchStreaming(
      makeRequest('remoteWorkspace.subscribeChanged'),
      (response) => replies.push(JSON.parse(response))
    )

    await vi.waitFor(() => {
      expect(replies).toHaveLength(1)
    })
    expect(replies[0]).toMatchObject({ result: { type: 'ready' } })

    emitChanged?.({ targetId: 't-1', snapshot: { revision: 1 } })
    expect(replies).toHaveLength(2)
    expect(replies[1]).toMatchObject({ result: { targetId: 't-1' } })

    const subscriptionId = (cleanups.keys().next().value ?? '') as string
    cleanups.get(subscriptionId)?.()
    await dispatch
    expect(replies).toHaveLength(3)
    expect(replies[2]).toMatchObject({ result: { type: 'end' } })
  })
})
