import { CodeIntelError } from './codeintel-errors'

export type CodeIntelSource = {
  tool: 'gitnexus' | 'codegraph'
  version: string | null
  indexedAt: number | null
  commit: string | null
  lineBase: 1
}

export type CodeIntelResultPerf = {
  totalMs: number
  queueWaitMs: number
  cliCalls: number
  cli: Array<{ tool: string; command: string; ms: number; stdoutBytes: number }>
  parseMs: number
  truncated: boolean
}

export type CodeIntelResultEnvelope = {
  sources: CodeIntelSource[]
  headCommit: string | null
  stale: boolean
  truncated: boolean
  totalCount: number | null
  warnings?: string[]
  perf: CodeIntelResultPerf
  data: any
}

export function buildCodeIntelResult(params: {
  sources: CodeIntelSource[]
  headCommit: string | null
  stale: boolean
  truncated: boolean
  totalCount: number | null
  warnings?: string[]
  perf: CodeIntelResultPerf
  data: any
}): CodeIntelResultEnvelope {
  const res: CodeIntelResultEnvelope = {
    sources: params.sources,
    headCommit: params.headCommit,
    stale: params.stale,
    truncated: params.truncated,
    totalCount: params.totalCount,
    perf: params.perf,
    data: params.data
  }
  if (params.warnings && params.warnings.length > 0) {
    res.warnings = params.warnings
  }
  return res
}

export function computeStale(
  sources: CodeIntelSource[],
  headCommit: string | null,
  flags: { rootMismatch?: boolean; worktreeMismatch?: boolean; pendingChanges?: number }
): boolean {
  if (flags.rootMismatch || flags.worktreeMismatch || (flags.pendingChanges ?? 0) > 0) {
    return true
  }
  for (const src of sources) {
    if (src.commit !== headCommit && src.commit !== null) {
      return true
    }
  }
  return false
}

export class PerfCollector {
  private startTime: number
  private queueWaitMs = 0
  private cli: Array<{ tool: string; command: string; ms: number; stdoutBytes: number }> = []
  private parseMs = 0
  private isTruncated = false

  constructor() {
    this.startTime = Date.now()
  }

  start() {
    this.startTime = Date.now()
  }

  recordQueueWait(ms: number) {
    this.queueWaitMs += ms
  }

  recordCli(opts: { tool: string; command: string; ms: number; stdoutBytes: number }) {
    const validCommands = [
      'cypher', 'context', 'impact', 'query', 'status', 'callers', 'callees', 'files', 'affected', 'detect-changes'
    ]
    if (!validCommands.includes(opts.command)) {
      throw new Error(`Invalid perf command: ${opts.command}`)
    }
    this.cli.push(opts)
  }

  recordParse(ms: number) {
    this.parseMs += ms
  }

  setTruncated() {
    this.isTruncated = true
  }

  build(): CodeIntelResultPerf {
    return {
      totalMs: Date.now() - this.startTime,
      queueWaitMs: this.queueWaitMs,
      cliCalls: this.cli.length,
      cli: this.cli,
      parseMs: this.parseMs,
      truncated: this.isTruncated
    }
  }
}

export function fitResultToLimit(
  result: CodeIntelResultEnvelope,
  opts: { prune: Array<{ path: string[]; minKeep: number }> },
  maxBytes = 8 * 1024 * 1024
): CodeIntelResultEnvelope {
  let str = JSON.stringify(result)
  let len = Buffer.byteLength(str)

  if (len <= maxBytes) return result

  result.truncated = true
  result.perf.truncated = true

  for (const rule of opts.prune) {
    let target: any = result.data
    for (let i = 0; i < rule.path.length; i++) {
      if (target) target = target[rule.path[i]]
    }
    if (Array.isArray(target) && target.length > rule.minKeep) {
      while (target.length > rule.minKeep) {
        const dropCount = Math.max(1, Math.floor((target.length - rule.minKeep) / 2))
        target.splice(target.length - dropCount, dropCount)
        
        str = JSON.stringify(result)
        len = Buffer.byteLength(str)
        if (len <= maxBytes) return result
      }
    }
  }

  if (len > maxBytes) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Output too large even after truncation', {
      reason: 'OUTPUT_TOO_LARGE',
      bytes: len,
      limit: maxBytes
    })
  }

  return result
}
