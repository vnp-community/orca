/**
 * desktop-only-rpc-error-suppressor.ts — swallows the console-red
 * "Uncaught (in promise) RuntimeRpcCallError: Unknown method: X" noise for
 * RPC namespaces that only make sense in Electron desktop mode and are
 * deliberately NOT implemented in server/web mode (native file dialogs,
 * OS auto-updater, local CLI install, mobile LAN pairing, OS permission
 * prompts...). See specs/backend/api/desktop-only-rpc-parity-gaps.md
 * "Group A" for the full list + per-namespace rationale.
 *
 * Why a single global listener instead of guarding ~99 individual call
 * sites: those call sites are scattered across dozens of files, most only
 * reachable from Settings screens/features that may not even render in web
 * mode — retrofitting every one is a large, low-value change for a problem
 * that's really "this class of error is expected and not actionable here."
 * One `unhandledrejection` listener that recognizes the shape
 * (`RuntimeRpcCallError` + `code: 'method_not_found'` + a known desktop-only
 * namespace) and calls `event.preventDefault()` covers all of them at once,
 * without changing behavior for any OTHER unhandled rejection (those still
 * log normally — this only touches the specific "known, expected,
 * feature-not-available-here" case).
 *
 * NOT a substitute for actually porting a namespace: if a namespace moves
 * from "desktop-only" to "ported", remove it from `DESKTOP_ONLY_NAMESPACES`
 * below so a real regression (the newly-ported method itself going missing)
 * surfaces again instead of being silently swallowed.
 *
 * @module runtime/desktop-only-rpc-error-suppressor
 */

// Keep in sync with "Group A" in specs/backend/api/desktop-only-rpc-parity-gaps.md.
const DESKTOP_ONLY_NAMESPACES: ReadonlySet<string> = new Set([
  'shell',
  'ephemeralVm',
  'mobile',
  'app',
  'cli',
  'updater',
  'pet',
  'ui',
  'computerUsePermissions',
  'developerPermissions',
  'e2e',
  'export',
  'agentTrust',
  'localhostWorktreeLabels'
])

const UNKNOWN_METHOD_PATTERN = /^Unknown method: ([a-zA-Z0-9_]+)\./

let installed = false

export function installDesktopOnlyRpcErrorSuppressor(): void {
  if (installed || typeof window === 'undefined') {
    return
  }
  installed = true
  window.addEventListener('unhandledrejection', handleUnhandledRejection)
}

export function _uninstallDesktopOnlyRpcErrorSuppressorForTests(): void {
  if (!installed || typeof window === 'undefined') {
    return
  }
  installed = false
  window.removeEventListener('unhandledrejection', handleUnhandledRejection)
}

function handleUnhandledRejection(event: PromiseRejectionEvent): void {
  const namespace = desktopOnlyNamespaceFor(event.reason)
  if (!namespace) {
    return
  }
  // Why preventDefault(), not stopImmediatePropagation(): other listeners
  // (e.g. crash-diagnostics.ts's breadcrumb recorder) should still run
  // normally — this only suppresses the browser's own default "log this to
  // console as an error" behavior for the specific known-safe case.
  event.preventDefault()
  console.debug(
    `[desktop-only-rpc] ${namespace}.* not available in this runtime mode (expected) — suppressed console error for:`,
    event.reason instanceof Error ? event.reason.message : event.reason
  )
}

function desktopOnlyNamespaceFor(reason: unknown): string | null {
  if (!(reason instanceof Error) || reason.name !== 'RuntimeRpcCallError') {
    return null
  }
  const code = (reason as Error & { code?: unknown }).code
  if (code !== 'method_not_found') {
    return null
  }
  const match = UNKNOWN_METHOD_PATTERN.exec(reason.message)
  const namespace = match?.[1]
  if (!namespace || !DESKTOP_ONLY_NAMESPACES.has(namespace)) {
    return null
  }
  return namespace
}
