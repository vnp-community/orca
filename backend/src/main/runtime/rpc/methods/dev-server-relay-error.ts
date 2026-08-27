// ─── Dev Server relay error classification ───────────────────────────────────
// Split out of dev-server.ts so both it and dev-server-shell.ts can import it
// without a circular dependency between the two method files.

/**
 * Classifies and re-throws errors from relay.call() with a clear prefix so
 * the UI can distinguish between:
 *   - Connection / backend errors  → "Dev server not connected"
 *   - Agent-side method errors     → "[Agent: fs.readDir] No such file..."
 *   - Transport timeouts           → "[Agent: fs.readDir] timed out..."
 *
 * The prefix is intentionally human-readable: it appears verbatim in the
 * RemoteFileBrowser error pane and any toast notifications.
 */
export function wrapAgentError(agentMethod: string, err: unknown, devServerId: string): never {
  const raw = err instanceof Error ? err.message : String(err)
  const rawLower = raw.toLowerCase()

  // Relay / transport errors — not agent logic errors.
  // Why toLowerCase(): SshChannelMultiplexer may emit 'SSH connection lost...' or
  // 'Connection lost...' depending on caller context; we must catch both variants.
  const isConnectionError =
    raw === 'Not connected' ||
    rawLower.includes('connection lost') ||
    rawLower.includes('timed out') ||
    rawLower.includes('multiplexer disposed') ||
    rawLower.includes('ipc channel not available') ||
    rawLower.includes('ipc request timeout')

  const message = isConnectionError
    ? `Dev server '${devServerId}' connection error: ${raw}`
    : `[Agent: ${agentMethod}] ${raw}`

  throw new Error(message)
}
