// Why: platform.* is native/OS-only with no remote-runtime-environment
// equivalent. Unlike shell/app/updater, window.api.platform.get() is
// synchronous today (no IPC round trip) and its one call site
// (terminal-webgl-auto-policy.ts) needs that value before first paint —
// converting it to this async RPC form would regress that timing, so it is
// intentionally NOT wired into that call site. This wrapper exists so other,
// already-async callers (e.g. future mobile/CLI-style RPC consumers) have a
// uniform platform.get RPC path on desktop; the web build keeps using
// window.api.platform.get() directly, unchanged.
import { isWebClientLocation } from '../lib/web-client-location'
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export type RuntimePlatformInfo = {
  platform: string
  osRelease: string
  displayServer: 'wayland' | 'x11' | null
}

export async function platformGet(): Promise<RuntimePlatformInfo> {
  if (isWebClientLocation()) {
    return window.api.platform.get()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'platform.get')
}
