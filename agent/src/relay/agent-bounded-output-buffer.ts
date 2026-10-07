// src/relay/agent-bounded-output-buffer.ts
// Bounded output buffer for agent.execPrompt: collects the TAIL of stdout/stderr
// within a byte budget, preserving valid UTF-8 at truncation boundaries.
// Also provides budget splitting and frame-size safety trimming.

// ── BoundedOutputBuffer ────────────────────────────────────────────────────────

export class BoundedOutputBuffer {
  private readonly chunks: Buffer[] = []
  private total = 0
  private _truncated = false

  constructor(private readonly maxBytes: number) {}

  append(chunk: Buffer): void {
    if (chunk.length === 0) { return }

    // If a single chunk exceeds the budget, keep only its tail
    if (chunk.length >= this.maxBytes) {
      this.chunks.length = 0
      this.total = 0
      this._truncated = true
      this.chunks.push(chunk.subarray(chunk.length - this.maxBytes))
      this.total = this.maxBytes
      return
    }

    this.chunks.push(chunk)
    this.total += chunk.length

    // Drop from the front until we fit within budget
    while (this.total > this.maxBytes && this.chunks.length > 0) {
      const head = this.chunks[0]!
      const excess = this.total - this.maxBytes
      if (head.length <= excess) {
        this.chunks.shift()
        this.total -= head.length
        this._truncated = true
      } else {
        // Partial drop from front of first chunk
        this.chunks[0] = head.subarray(excess)
        this.total -= excess
        this._truncated = true
        break
      }
    }
  }

  get truncated(): boolean {
    return this._truncated
  }

  get byteLength(): number {
    return this.total
  }

  toString(): string {
    if (this.chunks.length === 0) { return '' }
    const buf = this.chunks.length === 1
      ? this.chunks[0]!
      : Buffer.concat(this.chunks)

    // If truncation cut mid-UTF-8 sequence, skip leading continuation bytes
    // (0b10xxxxxx = 0x80..0xBF) at the very start to avoid a replacement char.
    let start = 0
    if (this._truncated) {
      while (start < buf.length && (buf[start]! & 0xc0) === 0x80) {
        start++
      }
    }
    return buf.subarray(start).toString('utf8')
  }
}

// ── Budget split ───────────────────────────────────────────────────────────────

export function splitOutputBudget(total: number): {
  stdoutBytes: number
  stderrBytes: number
} {
  const stdoutBytes = Math.floor((total * 3) / 4)
  return { stdoutBytes, stderrBytes: total - stdoutBytes }
}

// ── Frame size safety ──────────────────────────────────────────────────────────

type TruncatableResult = Record<string, unknown> & {
  stdout?: string
  stderr?: string
  truncated?: { stdout: boolean; stderr: boolean }
}

/**
 * Ensures the JSON-serialised result fits within the websocket frame limit.
 * Iteratively sheds 25% from stdout then stderr until it fits, up to 8 rounds.
 * Kept in this file because it operates on the same size concerns as the buffer.
 */
export function fitResultToFrame(
  result: TruncatableResult,
  frameLimitBytes = 15 * 1024 * 1024
): TruncatableResult {
  let out = { ...result }

  for (let round = 0; round < 8; round++) {
    if (Buffer.byteLength(JSON.stringify(out)) <= frameLimitBytes) {
      break
    }

    const truncated = { stdout: out.truncated?.stdout ?? false, stderr: out.truncated?.stderr ?? false }

    if (typeof out['stdout'] === 'string' && out['stdout'].length > 0) {
      const cut = Math.floor(out['stdout'].length * 0.25)
      out = { ...out, stdout: out['stdout'].slice(cut) }
      truncated.stdout = true
    } else if (typeof out['stderr'] === 'string' && out['stderr'].length > 0) {
      const cut = Math.floor(out['stderr'].length * 0.25)
      out = { ...out, stderr: out['stderr'].slice(cut) }
      truncated.stderr = true
    } else {
      break
    }

    if (truncated.stdout || truncated.stderr) {
      out = { ...out, truncated }
    }
  }

  return out
}
