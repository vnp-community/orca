import { defineMethod, type RpcMethod } from '../core'

// Why: mirrors desktop/src/preload/index.ts's getLinuxDisplayServer(). Preload
// and main are separate Electron bundles with no shared-import precedent in
// this codebase, so this tiny env-only check (no state, no I/O) is
// intentionally duplicated here rather than reached into across that
// boundary — keep both copies in sync if the Wayland/X11 heuristic changes.
function getLinuxDisplayServer(): 'wayland' | 'x11' | null {
  if (process.platform !== 'linux') {
    return null
  }
  if (
    process.env.WAYLAND_DISPLAY ||
    process.env.XDG_SESSION_TYPE?.toLowerCase() === 'wayland' ||
    process.env.ELECTRON_OZONE_PLATFORM_HINT?.toLowerCase() === 'wayland'
  ) {
    return 'wayland'
  }
  return process.env.DISPLAY ? 'x11' : null
}

type PlatformInfo = {
  platform: NodeJS.Platform
  osRelease: string
  displayServer: 'wayland' | 'x11' | null
}

// Why: platform.* is native/OS-only. Unlike the preload's window.api.platform
// (synchronous, no IPC round trip — used at terminal-pane-mount time by
// terminal-webgl-auto-policy.ts, which needs the value before the first
// paint), this RPC form is for callers that already talk to the runtime
// dispatcher asynchronously; it is intentionally NOT wired into that
// synchronous call site — see report for why.
export const PLATFORM_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'platform.get',
    params: null,
    handler: (): PlatformInfo => ({
      platform: process.platform,
      osRelease:
        (process as NodeJS.Process & { getSystemVersion?: () => string }).getSystemVersion?.() ??
        '',
      displayServer: getLinuxDisplayServer()
    })
  })
]
