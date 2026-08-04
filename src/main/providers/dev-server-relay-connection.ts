/**
 * Minimal shape the Dev Server providers need from a relay connection.
 *
 * Deliberately NOT typed against the concrete `DevServerRelayBridge` class:
 * in multi-user mode, per-user child processes only ever see
 * `GatewayDevServerManagerProxy.getRelay()`, which returns a plain
 * `{ call }` object that forwards over IPC to the real bridge in the parent
 * process (see src/main/dev-server/gateway-proxy.ts). Both shapes satisfy
 * this interface structurally, so the providers work unmodified in either
 * process.
 */
export type DevServerRelayConnection = {
  call<T = unknown>(method: string, params?: Record<string, unknown>, timeoutMs?: number): Promise<T>
  /**
   * Subscribe to one-way notifications pushed by the agent (pty.data, pty.exit,
   * fs.changed). Optional: older agent binaries and any relay shape that
   * predates this addition simply never call the handler — callers must fall
   * back to polling when this is absent.
   */
  onNotification?(handler: (method: string, params: Record<string, unknown>) => void): () => void
}
