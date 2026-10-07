export const CODEINTEL_CLI_COMMANDS = [
  'cypher',
  'context',
  'impact',
  'query',
  'status',
  'callers',
  'callees',
  'files',
  'affected',
  'detect-changes'
] as const

export type CodeIntelCliCommand = typeof CODEINTEL_CLI_COMMANDS[number]

export interface CliPerfEntry {
  tool: 'gitnexus' | 'codegraph' | 'git'
  command: CodeIntelCliCommand
  ms: number
  stdoutBytes: number
  rssPeakKb?: number
}

export interface CodeIntelPerf {
  totalMs: number
  queueWaitMs: number
  cliCalls: number
  cli: CliPerfEntry[]
  parseMs: number
  truncated: boolean
}

export class PerfRecorder {
  private startTime: number
  private queueWaitMs = 0
  private parseMs = 0
  private truncated = false
  private cliEntries: CliPerfEntry[] = []

  constructor(private readonly now: () => number = () => Date.now()) {
    this.startTime = this.now()
  }

  recordQueueWait(ms: number): void {
    this.queueWaitMs += Math.max(0, Math.round(ms))
  }

  recordParse(ms: number): void {
    this.parseMs += Math.max(0, Math.round(ms))
  }

  recordCli(entry: CliPerfEntry): void {
    if (!CODEINTEL_CLI_COMMANDS.includes(entry.command as any)) {
      throw new Error(`Invalid CodeIntel CLI command: ${entry.command}. Must be one of ${CODEINTEL_CLI_COMMANDS.join(', ')}`)
    }
    this.cliEntries.push({
      tool: entry.tool,
      command: entry.command,
      ms: Math.max(0, Math.round(entry.ms)),
      stdoutBytes: Math.max(0, Math.round(entry.stdoutBytes)),
      ...(entry.rssPeakKb !== undefined && { rssPeakKb: Math.max(0, Math.round(entry.rssPeakKb)) })
    })
  }

  markTruncated(): void {
    this.truncated = true
  }

  build(): CodeIntelPerf {
    const elapsed = Math.max(0, Math.round(this.now() - this.startTime))
    return {
      totalMs: elapsed,
      queueWaitMs: this.queueWaitMs,
      cliCalls: this.cliEntries.length,
      cli: this.cliEntries.map(e => ({ ...e })),
      parseMs: this.parseMs,
      truncated: this.truncated
    }
  }
}

export function perfForCacheHit(totalMs = 0): CodeIntelPerf {
  return {
    totalMs: Math.max(0, Math.round(totalMs)),
    queueWaitMs: 0,
    cliCalls: 0,
    cli: [],
    parseMs: 0,
    truncated: false
  }
}

export function stripVolatileResultFields(result: any): any {
  if (result === null || typeof result !== 'object') return result
  const clone = JSON.parse(JSON.stringify(result))
  delete clone.perf
  delete clone.startedAt
  return clone
}
