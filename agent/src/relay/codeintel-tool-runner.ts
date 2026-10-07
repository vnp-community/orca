import fs from 'fs'
import path from 'path'
import os from 'os'
import { AgentConfig } from './agent-config'
import { CodeIntelError } from './codeintel-errors'
import { runToolCommand } from './agent-tool-registry'
import { buildCodeIntelChildEnv } from './codeintel-child-env'
import { CodeIntelConcurrencyGate } from './codeintel-concurrency-gate'
import { tailForStderr } from './codeintel-secret-redaction'

export type CodeIntelToolRunnerOpts = {
  signal?: AbortSignal
  deadline: number
  tool: 'gitnexus' | 'codegraph'
  stdinText?: string
}

export type CodeIntelToolResult = {
  stdout: string
  stderr: string
  exitCode: number
  durationMs: number
  stdoutBytes: number
}

let sweptStaleDirs = false

function sweepStaleCodeIntelTempDirs() {
  if (sweptStaleDirs) return
  sweptStaleDirs = true
  const uid = typeof process.getuid === 'function' ? process.getuid() : 'unknown'
  const prefix = `orca-codeintel-${uid}-`
  const tmpdir = os.tmpdir()
  try {
    const entries = fs.readdirSync(tmpdir)
    const now = Date.now()
    for (const entry of entries) {
      if (entry.startsWith(prefix)) {
        const fullPath = path.join(tmpdir, entry)
        try {
          const stat = fs.statSync(fullPath)
          if (now - stat.mtimeMs > 3600_000) {
            fs.rmSync(fullPath, { recursive: true, force: true })
          }
        } catch {}
      }
    }
  } catch {}
}

export function resolveCodeIntelBinary(binary: string, toolPath: string): string {
  const paths = toolPath.split(path.delimiter)
  for (const dir of paths) {
    if (!dir) continue
    const candidate = path.join(dir, binary)
    try {
      fs.accessSync(candidate, fs.constants.X_OK)
      return candidate
    } catch {}
  }
  return binary
}

export async function runCodeIntelTool(
  argv: string[],
  cwd: string,
  config: AgentConfig,
  gate: CodeIntelConcurrencyGate,
  opts: CodeIntelToolRunnerOpts,
  ctx?: { perf?: any[] }
): Promise<CodeIntelToolResult> {
  sweepStaleCodeIntelTempDirs()
  
  const startTime = Date.now()
  let elapsedBeforeQueue = 0

  const release = await gate.acquire(opts.tool, opts.signal)
  try {
    elapsedBeforeQueue = Date.now() - startTime
    const remainingTime = opts.deadline - Date.now()
    const toolTimeoutMs = 20000 // In reality comes from limits
    const timeout = Math.min(toolTimeoutMs, Math.max(0, remainingTime))
    
    if (timeout <= 0) {
      throw new CodeIntelError('CODEINTEL_TIMEOUT', 'Deadline exceeded before starting tool', { tool: opts.tool, elapsedMs: elapsedBeforeQueue })
    }

    const env = buildCodeIntelChildEnv(config)
    let tmpDir: string | undefined
    let stdoutFile: string | undefined
    const uid = typeof process.getuid === 'function' ? process.getuid() : 'unknown'
    const binary = resolveCodeIntelBinary(opts.tool, config.toolPath || '')

    try {
      let maxOutputBytes = 16 * 1024 * 1024
      if (opts.tool === 'gitnexus') {
        tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), `orca-codeintel-${uid}-`))
        fs.chmodSync(tmpDir, 0o700)
        stdoutFile = path.join(tmpDir, 'stdout.txt')
      }

      const res = await runToolCommand(binary, argv, {
        cwd,
        timeout,
        env,
        maxOutputBytes,
        stdoutFile,
        signal: opts.signal,
        stdinText: opts.stdinText
      })

      if (opts.signal?.aborted) {
        throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Aborted', { reason: 'aborted' })
      }

      let stdout = res.stdout
      let stdoutBytes = 0

      if (stdoutFile && fs.existsSync(stdoutFile)) {
        stdoutBytes = fs.statSync(stdoutFile).size
        if (stdoutBytes > maxOutputBytes) {
          throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Output too large', { reason: 'OUTPUT_TOO_LARGE', bytes: stdoutBytes, limit: maxOutputBytes })
        }
        stdout = fs.readFileSync(stdoutFile, 'utf8')
      } else {
        stdoutBytes = Buffer.byteLength(stdout)
      }

      if (res.meta?.truncated) {
        const r = res.meta?.timedOut ? 'TIMEOUT' : 'OUTPUT_TOO_LARGE'
        if (r === 'TIMEOUT') throw new CodeIntelError('CODEINTEL_TIMEOUT', 'Tool timeout', { tool: opts.tool, elapsedMs: Date.now() - startTime })
        throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Output truncated', { reason: 'OUTPUT_TOO_LARGE' })
      }

      if (res.meta?.timedOut) {
        throw new CodeIntelError('CODEINTEL_TIMEOUT', 'Tool timeout', { tool: opts.tool, elapsedMs: Date.now() - startTime })
      }

      const durationMs = Date.now() - startTime
      if (ctx?.perf) {
        ctx.perf.push({ tool: opts.tool, command: argv[0], ms: durationMs, stdoutBytes })
      }

      if (res.exitCode !== 0) {
        throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Tool exited with non-zero code', { tool: opts.tool, exitCode: res.exitCode, stderrTail: tailForStderr(res.stderr) })
      }

      return {
        stdout,
        stderr: res.stderr,
        exitCode: res.exitCode,
        durationMs,
        stdoutBytes
      }
    } finally {
      if (tmpDir && fs.existsSync(tmpDir)) {
        fs.rmSync(tmpDir, { recursive: true, force: true })
      }
    }
  } finally {
    release()
  }
}
