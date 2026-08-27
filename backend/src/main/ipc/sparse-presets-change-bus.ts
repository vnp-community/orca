// Why: ports desktop/src/main/ipc/sparse-presets-change-bus.ts verbatim.
// Desktop fans `sparsePresets:changed` out to both `mainWindow.webContents`
// and this bus; backend has no BrowserWindow, so the RPC method
// (sparsePresets.subscribeChanged) is the bus's only consumer here.
type SparsePresetsChangedEvent = { repoId: string }

const listeners = new Set<(event: SparsePresetsChangedEvent) => void>()

export function onSparsePresetsChanged(
  listener: (event: SparsePresetsChangedEvent) => void
): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function notifySparsePresetsChangedListeners(event: SparsePresetsChangedEvent): void {
  for (const listener of listeners) {
    listener(event)
  }
}
