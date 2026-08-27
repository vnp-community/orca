import type { RemoteWorkspaceChangedEvent } from '../../shared/remote-workspace-types'

// Why: `remoteWorkspace:changed` is delivered to the desktop renderer via
// `mainWindow.webContents.send`, which only reaches Electron's own window.
// The remoteWorkspace.subscribeChanged RPC method needs the same events, so
// this tiny bus lets `handleRemoteWorkspaceNotification` fan the event out to
// both destinations without RPC methods reaching into BrowserWindow internals.
const listeners = new Set<(event: RemoteWorkspaceChangedEvent) => void>()

export function onRemoteWorkspaceChanged(
  listener: (event: RemoteWorkspaceChangedEvent) => void
): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function notifyRemoteWorkspaceChangedListeners(event: RemoteWorkspaceChangedEvent): void {
  for (const listener of listeners) {
    listener(event)
  }
}
