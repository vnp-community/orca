/**
 * pty-daemon-protocol.ts — Wire format for the agent ↔ pty-daemon Unix socket.
 *
 * Why a separate, simpler protocol than agent-wire.ts's 13-byte binary frames:
 * that format exists to survive an unreliable network link (seq/ack, keepalive
 * timeouts). A Unix domain socket between two processes on the same machine is
 * a reliable, ordered, local stream — newline-delimited JSON (NDJSON) is
 * simpler to implement correctly and there is nothing here worth the extra
 * complexity budget.
 */

export type DaemonRequest = {
  id: number
  method: string
  params?: Record<string, unknown>
}

export type DaemonResponse = {
  id: number
  result?: unknown
  error?: { message: string }
}

export type DaemonNotification = {
  method: string
  params: Record<string, unknown>
}

export type DaemonMessage = DaemonRequest | DaemonResponse | DaemonNotification

export function isDaemonRequest(msg: DaemonMessage): msg is DaemonRequest {
  return 'id' in msg && 'method' in msg
}

export function isDaemonResponse(msg: DaemonMessage): msg is DaemonResponse {
  return 'id' in msg && !('method' in msg)
}

export function encodeDaemonMessage(msg: DaemonMessage): string {
  return `${JSON.stringify(msg)}\n`
}

/**
 * Incremental NDJSON line reader. Feed it raw socket chunks; it emits one
 * parsed message per complete line. Malformed lines are dropped silently
 * (a corrupt line on a local, same-machine socket indicates a bug, not a
 * transient network issue worth surfacing per-line).
 */
export class DaemonMessageDecoder {
  private buffer = ''

  constructor(private readonly onMessage: (msg: DaemonMessage) => void) {}

  feed(chunk: string): void {
    this.buffer += chunk
    let newlineIndex = this.buffer.indexOf('\n')
    while (newlineIndex !== -1) {
      const line = this.buffer.slice(0, newlineIndex)
      this.buffer = this.buffer.slice(newlineIndex + 1)
      if (line.trim()) {
        try {
          this.onMessage(JSON.parse(line) as DaemonMessage)
        } catch {
          // drop malformed line
        }
      }
      newlineIndex = this.buffer.indexOf('\n')
    }
  }
}
