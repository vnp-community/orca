import { useEffect, useState } from 'react'

/**
 * Bounded window a brand-new, still-uncorrelated local terminal tab is allowed
 * to sit un-mounted while its Dev-Server host mirror is expected to arrive.
 *
 * Why: bounded by the WS handshake + `terminal.create` round trip observed
 * during the BUG-FE-PTY-001 investigation (usually well under a second, with
 * some headroom for a busy host). If the mirror hasn't landed by then, we
 * stop deferring and mount with whatever id the tab currently has — the same
 * behavior as before this fix, so the fallback path can only reproduce the
 * pre-existing bug, never something new.
 */
export const GRACE_MOUNT_DEFER_MS = 4000

export type TerminalMountGateTab = {
  pendingActivationSpawn?: boolean | number
  ptyId: string | null
  createdAt: number
}

/**
 * Whether `TerminalPane` should defer mounting for this tab.
 *
 * A newly created local terminal tab on a Dev-Server-mirrored worktree starts
 * life with a throwaway client-side uuid `id`. Once the host echoes back its
 * own session surface, `web-session-tabs-sync.ts` replaces that tab with a
 * mirror tab whose `id` is a different string (`web-terminal-<hostTabId>`),
 * which forces React to fully unmount/remount `TerminalPane` — tearing down
 * the live PaneManager/xterm and PTY transport mid-flight (BUG-FE-PTY-001
 * #10: terminal accepts no keystrokes and echoes nothing after the swap).
 *
 * Deferring the mount until the tab's id has settled (mirror arrived, or the
 * grace window lapsed) means `TerminalPane` is only ever mounted once per
 * logical terminal, under its final id — the swap never happens under a live
 * pane.
 */
export function shouldDeferTerminalPaneMount(
  tab: TerminalMountGateTab,
  isHostMirroredEnvironment: boolean,
  nowMs: number
): boolean {
  if (!isHostMirroredEnvironment) {
    return false
  }
  if (!tab.pendingActivationSpawn || tab.ptyId !== null) {
    return false
  }
  return nowMs - tab.createdAt < GRACE_MOUNT_DEFER_MS
}

/**
 * React-hook wrapper around `shouldDeferTerminalPaneMount` that also forces a
 * re-render once the grace window lapses, so a tab whose mirror never shows
 * up (host never responds) reliably falls through to mounting instead of
 * being stuck on the placeholder with nothing left to trigger a re-check.
 */
export function useShouldDeferTerminalPaneMount(
  tab: TerminalMountGateTab,
  isHostMirroredEnvironment: boolean
): boolean {
  const shouldDefer = shouldDeferTerminalPaneMount(tab, isHostMirroredEnvironment, Date.now())
  const [, forceRecheck] = useState(0)
  useEffect(() => {
    if (!shouldDefer) {
      return
    }
    const remainingMs = Math.max(0, GRACE_MOUNT_DEFER_MS - (Date.now() - tab.createdAt))
    // Why: +50ms so the recheck lands after GRACE_MOUNT_DEFER_MS has
    // definitely elapsed, not exactly on the boundary.
    const timerId = window.setTimeout(() => forceRecheck((tick) => tick + 1), remainingMs + 50)
    return () => window.clearTimeout(timerId)
  }, [shouldDefer, tab.createdAt])
  return shouldDefer
}
