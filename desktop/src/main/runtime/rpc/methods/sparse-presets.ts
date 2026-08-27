import { z } from 'zod'
import { defineMethod, defineStreamingMethod, type RpcAnyMethod } from '../core'
import { OptionalString, requiredString } from '../schemas'
import type { Store } from '../../../persistence'
import type { SparsePreset } from '../../../../shared/types'
import {
  listSparsePresetsForRepo,
  removeSparsePresetForRepo,
  saveSparsePresetForRepo
} from '../../../ipc/repos'
import {
  notifySparsePresetsChangedListeners,
  onSparsePresetsChanged
} from '../../../ipc/sparse-presets-change-bus'

// Why: monotonically increasing per-process counter, matching the
// notifications.subscribe pattern, avoids Date.now() collisions between
// near-simultaneous subscribe calls.
let sparsePresetsSubscriptionSeq = 0

const SparsePresetsListParams = z.object({
  repoId: requiredString('Missing repoId')
})

const SparsePresetsSaveParams = z.object({
  repoId: requiredString('Missing repoId'),
  id: OptionalString,
  name: requiredString('Missing name'),
  directories: z.array(z.string())
})

const SparsePresetsRemoveParams = z.object({
  repoId: requiredString('Missing repoId'),
  presetId: requiredString('Missing presetId')
})

function requireStore(runtime: { getRuntimeStoreForRpc(): unknown }): Store {
  // Why: RuntimeStore only narrows the store's static type — the instance
  // backing it is always the real Store (see getRuntimeStoreForRpc).
  const store = runtime.getRuntimeStoreForRpc() as unknown as Store | null
  if (!store) {
    throw new Error('runtime_unavailable')
  }
  return store
}

export const SPARSE_PRESET_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'sparsePresets.list',
    params: SparsePresetsListParams,
    handler: async (params, { runtime }): Promise<SparsePreset[]> =>
      listSparsePresetsForRepo(requireStore(runtime), params.repoId)
  }),
  defineMethod({
    name: 'sparsePresets.save',
    params: SparsePresetsSaveParams,
    handler: async (params, { runtime }): Promise<SparsePreset> => {
      const saved = saveSparsePresetForRepo(requireStore(runtime), params)
      notifySparsePresetsChangedListeners({ repoId: params.repoId })
      return saved
    }
  }),
  defineMethod({
    name: 'sparsePresets.remove',
    params: SparsePresetsRemoveParams,
    handler: async (params, { runtime }): Promise<void> => {
      removeSparsePresetForRepo(requireStore(runtime), params)
      notifySparsePresetsChangedListeners({ repoId: params.repoId })
    }
  }),
  defineStreamingMethod({
    name: 'sparsePresets.subscribeChanged',
    params: null,
    handler: async (_params, { runtime, connectionId }, emit) => {
      await new Promise<void>((resolve) => {
        const unsubscribe = onSparsePresetsChanged((event) => {
          emit(event)
        })

        const seq = ++sparsePresetsSubscriptionSeq
        const subscriptionId = `sparse-presets-${connectionId ?? 'inproc'}-${seq}`
        runtime.registerSubscriptionCleanup(
          subscriptionId,
          () => {
            unsubscribe()
            emit({ type: 'end' })
            resolve()
          },
          connectionId
        )

        emit({ type: 'ready', subscriptionId })
      })
    }
  })
]
