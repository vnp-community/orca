export type CodeIntelPerf = {
  totalMs: number
  queueWaitMs: number
  cliCalls: number
  cli: Array<{ tool: string; command: string; ms: number; stdoutBytes: number; rssPeakKb?: number }>
  parseMs: number
  truncated: boolean
}

export const CODEINTEL_CLI_COMMANDS = [
  'cypher', 'context', 'impact', 'query', 'status', 'callers', 'callees', 'files', 'affected', 'detect-changes'
]

export class PerfRecorder {
  private startTimeMs: number
  private queueWaitMs = 0
  private parseMs = 0
  private cli: Array<{ tool: string; command: string; ms: number; stdoutBytes: number; rssPeakKb?: number }> = []
  private truncated = false
  private now: () => number

  constructor(nowFn: () => number = Date.now) {
    this.now = nowFn
    this.startTimeMs = this.now()
  }

  public start() {
    this.startTimeMs = this.now()
  }

  public recordQueueWait(ms: number) {
    this.queueWaitMs += ms
  }

  public recordParse(ms: number) {
    this.parseMs += ms
  }

  public recordCli(opts: { tool: string; command: string; ms: number; stdoutBytes: number; rssPeakKb?: number }) {
    if (!CODEINTEL_CLI_COMMANDS.includes(opts.command)) {
      throw new Error(`Invalid perf command: ${opts.command}`)
    }
    this.cli.push({ ...opts })
  }

  public setTruncated(val = true) {
    this.truncated = val
  }

  public build(): CodeIntelPerf {
    return {
      totalMs: this.now() - this.startTimeMs,
      queueWaitMs: this.queueWaitMs,
      cliCalls: this.cli.length,
      cli: [...this.cli],
      parseMs: this.parseMs,
      truncated: this.truncated
    }
  }
}

export function perfForCacheHit(): CodeIntelPerf {
  return {
    totalMs: 0,
    queueWaitMs: 0,
    cliCalls: 0,
    cli: [],
    parseMs: 0,
    truncated: false
  }
}
