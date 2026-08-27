import { defineMethod, type RpcAnyMethod } from '../core'

// Why: mirrors desktop/src/main/runtime/rpc/methods/platform.ts's Linux
// display-server heuristic. Kept as its own copy rather than a shared
// import — see desktop's file for the same "no shared-import precedent
// across app targets" rationale.
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

// Why: in desktop this describes the user's own machine. In server mode it
// describes the Orca Server HOST — a shared machine, not the browser
// client's device — so callers must not treat this as "the user's OS".
// Kept as the same RPC shape/name as desktop for frontend compatibility;
// only the semantics of what "this machine" means have changed.
export const PLATFORM_METHODS: readonly RpcAnyMethod[] = [
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
