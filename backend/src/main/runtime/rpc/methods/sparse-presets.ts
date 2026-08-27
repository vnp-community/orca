import { randomUUID } from 'node:crypto'
import { z } from 'zod'
import { defineMethod, defineStreamingMethod, type RpcAnyMethod } from '../core'
import { OptionalString, requiredString } from '../schemas'
import type { Store } from '../../../persistence'
import type { SparsePreset } from '../../../../shared/types'
import { getActiveOnboardingStore } from '../../../ipc/onboarding-ipc'
import { normalizeSparseDirectories } from '../../../ipc/sparse-checkout-directories'
import {
  notifySparsePresetsChangedListeners,
  onSparsePresetsChanged
} from '../../../ipc/sparse-presets-change-bus'

// Why: ports desktop/src/main/runtime/rpc/methods/sparse-presets.ts. The
// underlying Store methods (getSparsePresets/saveSparsePreset/
// removeSparsePreset) already exist in backend's persistence.ts — this file
// is the missing RPC layer, plus the list/save/remove/normalize bodies
// desktop keeps in ipc/repos.ts (reproduced here rather than imported since
// backend has no ipc/repos.ts). Note: backend/.../runtime/rpc/methods/repo.ts
// already exposes a DIFFERENT namespace (`repo.sparsePresets` /
// `repo.saveSparsePreset`, no remove/subscribe) backed by the same Store
// methods via OrcaRuntimeService — that pair predates this port and is left
// untouched; this file adds the `sparsePresets.*` namespace desktop's
// frontend actually calls.

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

function requireStore(): Store {
  const store = getActiveOnboardingStore()
  if (!store) {
    throw new Error('runtime_unavailable')
  }
  return store
}

function normalizeSparsePresetName(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) {
    throw new Error('Preset name is required.')
  }
  if (trimmed.length > 80) {
    throw new Error('Preset name is too long.')
  }
  return trimmed
}

function normalizeSparsePresetDirectories(directories: string[]): string[] {
  let normalized: string[]
  try {
    normalized = normalizeSparseDirectories(directories)
  } catch (err) {
    if (
      err instanceof Error &&
      err.message === 'Sparse checkout directories must be repo-relative paths.'
    ) {
      throw new Error('Preset directories must be repo-relative paths.')
    }
    throw err
  }
  if (normalized.length === 0) {
    throw new Error('Preset must have at least one directory.')
  }
  return normalized
}

function listSparsePresetsForRepo(store: Store, repoId: string): SparsePreset[] {
  return store.getSparsePresets(repoId)
}

function saveSparsePresetForRepo(
  store: Store,
  args: { repoId: string; id?: string; name: string; directories: string[] }
): SparsePreset {
  const repo = store.getRepo(args.repoId)
  if (!repo) {
    throw new Error(`Repo "${args.repoId}" not found`)
  }
  const name = normalizeSparsePresetName(args.name)
  const directories = normalizeSparsePresetDirectories(args.directories)
  const now = Date.now()
  const existing = args.id
    ? store.getSparsePresets(args.repoId).find((preset) => preset.id === args.id)
    : undefined
  const preset: SparsePreset = {
    id: existing?.id ?? randomUUID(),
    repoId: args.repoId,
    name,
    directories,
    createdAt: existing?.createdAt ?? now,
    updatedAt: now
  }
  return store.saveSparsePreset(preset)
}

function removeSparsePresetForRepo(store: Store, args: { repoId: string; presetId: string }): void {
  const repo = store.getRepo(args.repoId)
  if (!repo) {
    throw new Error(`Repo "${args.repoId}" not found`)
  }
  store.removeSparsePreset(args.repoId, args.presetId)
}

export const SPARSE_PRESET_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'sparsePresets.list',
    params: SparsePresetsListParams,
    handler: async (params): Promise<SparsePreset[]> =>
      listSparsePresetsForRepo(requireStore(), params.repoId)
  }),
  defineMethod({
    name: 'sparsePresets.save',
    params: SparsePresetsSaveParams,
    handler: async (params): Promise<SparsePreset> => {
      const saved = saveSparsePresetForRepo(requireStore(), params)
      notifySparsePresetsChangedListeners({ repoId: params.repoId })
      return saved
    }
  }),
  defineMethod({
    name: 'sparsePresets.remove',
    params: SparsePresetsRemoveParams,
    handler: async (params): Promise<void> => {
      removeSparsePresetForRepo(requireStore(), params)
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
