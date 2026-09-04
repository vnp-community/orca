// src/relay/agent-connection-stdio.ts
// stdio connection mode: infra-fleet-service (the Go backend) SSH-deploys
// this same agent.js bundle to a remote host and launches it as
// `node agent.js --stdio`, wiring its stdin/stdout to an SSH exec channel.
//
// There is no WebSocket dial or listen at all — the SSH connection itself is
// both the transport and the trust boundary (no token, no reconnect loop,
// no keepalive ping: SSH already has its own liveness).
//
// agent-session.ts (NOT modified — see its own header) is written against
// the 'ws' package's WebSocket interface. StdioWebSocketAdapter below is a
// duck-typed stand-in for exactly the subset it actually uses:
// readyState/send()/close()/once('open')/on('message'|'close'|'error').

import { EventEmitter } from 'node:events'
import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import type { AgentLogger } from './agent-logger'
import { createSession } from './agent-session'
import { HEADER_SIZE } from 'orca-dev-agent-transport'

/**
 * Reassembles the raw bytes of Agent Wire Protocol frames (agent-wire.ts,
 * "Stack A": `[TYPE u8][SEQ u32BE][ACK u32BE][LENGTH u32BE][PAYLOAD]`,
 * HEADER_SIZE=13, LENGTH at byte offset 9) off an arbitrarily-chunked byte
 * stream. process.stdin's 'data' events do NOT respect frame boundaries the
 * way a single WebSocket 'message' event does — a chunk may contain zero,
 * one, or many complete frames, or a partial frame split across chunks.
 *
 * Technique mirrors protocol.ts's FrameDecoder (a chunk-list instead of
 * repeated Buffer.concat, so a large payload isn't re-copied on every
 * incoming chunk) but only extracts raw byte slices — one full
 * header+payload slice per complete frame — and hands each one to `onFrame`
 * unmodified. agent-session.ts's own decodeFrame() (agent-wire.ts, Stack A)
 * does the actual field parsing from there, exactly as it already does for
 * the WS modes; this class never decodes or re-encodes anything itself.
 */
class StdioFrameAccumulator {
  private chunks: Buffer[] = []
  private bufferedLength = 0

  feed(chunk: Buffer, onFrame: (raw: Buffer) => void): void {
    this.chunks.push(chunk)
    this.bufferedLength += chunk.length

    while (this.bufferedLength >= HEADER_SIZE) {
      const header = this.peek(HEADER_SIZE)
      const length = header.readUInt32BE(9) // LENGTH field, see agent-wire.ts frame layout
      const totalLength = HEADER_SIZE + length

      if (this.bufferedLength < totalLength) {
        break // full frame not buffered yet — wait for more chunks
      }

      onFrame(this.take(totalLength))
    }
  }

  /** View of the first `count` buffered bytes without consuming them. */
  private peek(count: number): Buffer {
    const first = this.chunks[0]!
    if (first.length >= count) {
      return first
    }
    const out = Buffer.allocUnsafe(count)
    let copied = 0
    for (const part of this.chunks) {
      copied += part.copy(out, copied, 0, Math.min(part.length, count - copied))
      if (copied >= count) {
        break
      }
    }
    return out
  }

  /** Consume and return the first `count` buffered bytes. */
  private take(count: number): Buffer {
    const first = this.chunks[0]!
    if (first.length === count) {
      this.chunks.shift()
      this.bufferedLength -= count
      return first
    }
    if (first.length > count) {
      this.chunks[0] = first.subarray(count)
      this.bufferedLength -= count
      return first.subarray(0, count)
    }
    const out = Buffer.allocUnsafe(count)
    let copied = 0
    while (copied < count) {
      const part = this.chunks[0]!
      const take = Math.min(part.length, count - copied)
      part.copy(out, copied, 0, take)
      copied += take
      if (take === part.length) {
        this.chunks.shift()
      } else {
        this.chunks[0] = part.subarray(take)
      }
    }
    this.bufferedLength -= count
    return out
  }
}

/**
 * Duck-typed adapter backing agent-session.ts's `start(ws)` with a stdio-like
 * byte stream pair instead of a real WebSocket. Since stdio is connected the
 * instant the process starts, readyState reports OPEN (1) synchronously —
 * there is no dial/handshake delay to wait for, unlike a real WS.
 *
 * `input`/`output` default to process.stdin/process.stdout (production use);
 * tests inject a real connected stream pair (net.Socket or PassThrough)
 * instead, per this file's test suite.
 */
export class StdioWebSocketAdapter extends EventEmitter {
  readyState = 1 // WebSocket.OPEN

  private readonly accumulator = new StdioFrameAccumulator()
  private closed = false

  constructor(
    private readonly log: AgentLogger,
    private readonly input: NodeJS.ReadableStream = process.stdin,
    private readonly output: NodeJS.WritableStream = process.stdout
  ) {
    super()

    this.input.on('data', (chunk: Buffer) => {
      this.accumulator.feed(chunk, (raw) => this.emit('message', raw))
    })
    this.input.on('end', () => this.handleClose(1000, 'stdin EOF'))
    this.input.on('error', (err: Error) => {
      this.log.error(`stdin error: ${err.message}`)
      this.emit('error', err)
      this.handleClose(1006, 'stdin error')
    })
    this.output.on('error', (err: Error) => {
      this.log.error(`stdout error: ${err.message}`)
      this.emit('error', err)
      this.handleClose(1006, 'stdout error')
    })

    // Fire 'open' asynchronously for symmetry with a real WebSocket. In
    // practice agent-session.ts's start() never waits on it — readyState is
    // already OPEN synchronously above, so it takes the immediate-handshake
    // branch instead of registering a once('open') listener.
    process.nextTick(() => this.emit('open'))
  }

  send(data: Buffer): void {
    if (this.closed) {
      return
    }
    this.output.write(data)
  }

  close(code?: number, reason?: string): void {
    this.handleClose(code ?? 1000, reason ?? '')
  }

  private handleClose(code: number, reason: string): void {
    if (this.closed) {
      return
    }
    this.closed = true
    this.readyState = 3 // WebSocket.CLOSED
    this.input.pause?.()
    this.emit('close', code, Buffer.from(reason, 'utf8'))
  }
}

/**
 * Run the agent session over stdio. Resolves once the channel closes — via
 * stdin EOF (the SSH exec channel/peer went away) or agent-session.ts itself
 * calling ws.close() (e.g. idle-watchdog timeout, handshake error). There is
 * no reconnect loop and no close-code concept to speak of: stdio only has
 * "still open" or "EOF", mirrored here as a single settle-once promise.
 */
export async function connectStdio(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<void> {
  const adapter = new StdioWebSocketAdapter(log)
  const session = createSession(config, tools, log)

  session.start(adapter as unknown as WebSocket)

  await new Promise<void>((resolve) => {
    adapter.once('close', () => resolve())
  })

  log.info('stdio channel closed — exiting')
}
