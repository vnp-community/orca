import { describe, expect, it, vi, beforeEach } from 'vitest'
import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'

const {
  listSparsePresetsForRepoMock,
  removeSparsePresetForRepoMock,
  saveSparsePresetForRepoMock,
  notifySparsePresetsChangedListenersMock,
  onSparsePresetsChangedMock
} = vi.hoisted(() => ({
  listSparsePresetsForRepoMock: vi.fn(),
  removeSparsePresetForRepoMock: vi.fn(),
  saveSparsePresetForRepoMock: vi.fn(),
  notifySparsePresetsChangedListenersMock: vi.fn(),
  onSparsePresetsChangedMock: vi.fn()
}))

vi.mock('../../../ipc/repos', () => ({
  listSparsePresetsForRepo: listSparsePresetsForRepoMock,
  removeSparsePresetForRepo: removeSparsePresetForRepoMock,
  saveSparsePresetForRepo: saveSparsePresetForRepoMock
}))

vi.mock('../../../ipc/sparse-presets-change-bus', () => ({
  notifySparsePresetsChangedListeners: notifySparsePresetsChangedListenersMock,
  onSparsePresetsChanged: onSparsePresetsChangedMock
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

describe('sparse presets RPC methods', () => {
  beforeEach(() => {
    listSparsePresetsForRepoMock.mockReset()
    removeSparsePresetForRepoMock.mockReset()
    saveSparsePresetForRepoMock.mockReset()
    notifySparsePresetsChangedListenersMock.mockReset()
    onSparsePresetsChangedMock.mockReset()
  })

  it('lists presets for a repo using the RPC-scoped store', async () => {
    listSparsePresetsForRepoMock.mockReturnValue([{ id: 'p-1' }])
    const runtime = makeStoreRuntime()
    const { SPARSE_PRESET_METHODS } = await import('./sparse-presets')
    const dispatcher = new RpcDispatcher({ runtime, methods: SPARSE_PRESET_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('sparsePresets.list', { repoId: 'repo-1' })
    )

    expect(listSparsePresetsForRepoMock).toHaveBeenCalledWith({}, 'repo-1')
    expect(response).toMatchObject({ ok: true, result: [{ id: 'p-1' }] })
  })

  it('saves a preset and notifies subscribers', async () => {
    saveSparsePresetForRepoMock.mockReturnValue({ id: 'p-1', repoId: 'repo-1' })
    const runtime = makeStoreRuntime()
    const { SPARSE_PRESET_METHODS } = await import('./sparse-presets')
    const dispatcher = new RpcDispatcher({ runtime, methods: SPARSE_PRESET_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('sparsePresets.save', {
        repoId: 'repo-1',
        name: 'preset',
        directories: ['src']
      })
    )

    expect(response).toMatchObject({ ok: true, result: { id: 'p-1' } })
    expect(notifySparsePresetsChangedListenersMock).toHaveBeenCalledWith({ repoId: 'repo-1' })
  })

  it('removes a preset and notifies subscribers', async () => {
    const runtime = makeStoreRuntime()
    const { SPARSE_PRESET_METHODS } = await import('./sparse-presets')
    const dispatcher = new RpcDispatcher({ runtime, methods: SPARSE_PRESET_METHODS })

    await dispatcher.dispatch(
      makeRequest('sparsePresets.remove', { repoId: 'repo-1', presetId: 'p-1' })
    )

    expect(removeSparsePresetForRepoMock).toHaveBeenCalledWith(
      {},
      { repoId: 'repo-1', presetId: 'p-1' }
    )
    expect(notifySparsePresetsChangedListenersMock).toHaveBeenCalledWith({ repoId: 'repo-1' })
  })

  it('streams changed events until cleaned up', async () => {
    let emitChanged: ((event: unknown) => void) | undefined
    onSparsePresetsChangedMock.mockImplementation((listener: (event: unknown) => void) => {
      emitChanged = listener
      return vi.fn()
    })
    const cleanups = new Map<string, () => void>()
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      registerSubscriptionCleanup: vi.fn((id, cleanup) => cleanups.set(id, cleanup))
    } as unknown as OrcaRuntimeService
    const { SPARSE_PRESET_METHODS } = await import('./sparse-presets')
    const dispatcher = new RpcDispatcher({ runtime, methods: SPARSE_PRESET_METHODS })
    const replies: unknown[] = []

    dispatcher.dispatchStreaming(makeRequest('sparsePresets.subscribeChanged'), (response) =>
      replies.push(JSON.parse(response))
    )

    await vi.waitFor(() => {
      expect(replies).toHaveLength(1)
    })

    emitChanged?.({ repoId: 'repo-1' })
    expect(replies).toHaveLength(2)
    expect(replies[1]).toMatchObject({ result: { repoId: 'repo-1' } })
  })
})
