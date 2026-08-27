// Why: `sparsePresets:changed` is delivered to the desktop renderer via
// `mainWindow.webContents.send`, which only reaches Electron's own window.
// The sparsePresets.subscribeChanged RPC method needs the same events, so
// this tiny bus lets repos.ts's notifySparsePresetsChanged fan the event out
// to both destinations without RPC methods reaching into BrowserWindow internals.
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
