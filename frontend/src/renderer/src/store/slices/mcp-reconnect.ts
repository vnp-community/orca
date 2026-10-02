// Module-level singleton for the MCP event stream: web unsubscribe() only drops
// the local callback (server keeps the goroutine until socket close), so the app
// must hold exactly ONE subscription regardless of how many callers start it.

export const MCP_RECONNECT_BASE_MS = 1_000
export const MCP_RECONNECT_MAX_MS = 30_000
export const MCP_RECONNECT_SLOW_MAX_MS = 5 * 60_000
// A stream that lived this long before closing counts as healthy (resets backoff).
export const MCP_HEALTHY_STREAM_MS = 30_000
const SLOW_AFTER_FAILURES = 6

export function nextMcpReconnectDelayMs(consecutiveFailures: number): number {
  const cap =
    consecutiveFailures >= SLOW_AFTER_FAILURES ? MCP_RECONNECT_SLOW_MAX_MS : MCP_RECONNECT_MAX_MS
  return Math.min(MCP_RECONNECT_BASE_MS * 2 ** consecutiveFailures, cap)
}
