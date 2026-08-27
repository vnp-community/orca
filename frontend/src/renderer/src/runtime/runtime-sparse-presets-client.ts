import type { GlobalSettings, SparsePreset } from '../../../shared/types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

type SparsePresetsSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

export function listSparsePresets(
  settings: SparsePresetsSettings | null | undefined,
  args: { repoId: string }
): Promise<SparsePreset[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.sparsePresets.list(args)
  }
  return callRuntimeRpc<SparsePreset[]>(target, 'sparsePresets.list', args)
}

export function saveSparsePreset(
  settings: SparsePresetsSettings | null | undefined,
  args: { repoId: string; id?: string; name: string; directories: string[] }
): Promise<SparsePreset> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.sparsePresets.save(args)
  }
  return callRuntimeRpc<SparsePreset>(target, 'sparsePresets.save', args)
}

export function removeSparsePreset(
  settings: SparsePresetsSettings | null | undefined,
  args: { repoId: string; presetId: string }
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.sparsePresets.remove(args)
  }
  return callRuntimeRpc<void>(target, 'sparsePresets.remove', args)
}

function isSparsePresetsChangedEvent(value: unknown): value is { repoId: string } {
  return (
    typeof value === 'object' &&
    value !== null &&
    'repoId' in value &&
    typeof (value as { repoId: unknown }).repoId === 'string'
  )
}

// Why: local mode keeps the existing always-on preload listener. An
// environment target subscribes to sparsePresets.subscribeChanged, which
// forwards the exact same { repoId } events repos.ts's notifySparsePresetsChanged
// pushes locally (see desktop/src/main/ipc/sparse-presets-change-bus.ts).
export function subscribeToSparsePresetsChanged(
  settings: SparsePresetsSettings | null | undefined,
  callback: (data: { repoId: string }) => void
): () => void {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.sparsePresets.onChanged(callback)
  }
  let handle: { unsubscribe: () => void } | null = null
  let cancelled = false
  window.api.runtimeEnvironments
    .subscribe(
      { selector: target.environmentId, method: 'sparsePresets.subscribeChanged' },
      {
        onResponse: (response) => {
          const payload = response.ok ? (response as { result: unknown }).result : undefined
          if (isSparsePresetsChangedEvent(payload)) {
            callback(payload)
          }
        }
      }
    )
    .then((subscription) => {
      handle = subscription
      if (cancelled) {
        subscription.unsubscribe()
      }
    })
    .catch(() => {})
  return () => {
    cancelled = true
    handle?.unsubscribe()
  }
}
