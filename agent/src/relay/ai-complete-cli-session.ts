// src/relay/ai-complete-cli-session.ts
// ai.complete through the dev server's own Claude Code login (no API key).
//
// The CLI runs with every tool disabled and in an empty scratch directory, so the call can only return
// text: the prompt is untrusted request content and must never be able to read files or run commands.
// Do NOT add --bare: it makes the CLI ignore OAuth/keychain, which is exactly the login this path needs.

import { execFile as _execFile, spawn } from 'node:child_process'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { promisify } from 'node:util'
import { AICompleteProviderError, type AICompleteErrorReason } from './ai-complete-errors'
import type { AICompleteUsage } from './ai-complete-handler'

const execFileAsync = promisify(_execFile)

export const CLI_PROVIDER_NAME = 'claude_cli'
export const CLI_DEFAULT_TIMEOUT_MS = 120_000
export const CLI_MAX_OUTPUT_BYTES = 4 * 1024 * 1024
export const CLI_MAX_CONCURRENT = 2
export const CLI_MAX_QUEUED = 8

/** Environment variables the CLI needs to find its login and reach the network; nothing else is inherited. */
const PASSTHROUGH_ENV = [
  'HOME', 'USER', 'LOGNAME', 'PATH', 'LANG', 'LC_ALL', 'TMPDIR', 'SHELL',
  'XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_CACHE_HOME',
  'HTTP_PROXY', 'HTTPS_PROXY', 'NO_PROXY', 'http_proxy', 'https_proxy', 'no_proxy',
  'SSL_CERT_FILE', 'SSL_CERT_DIR', 'NODE_EXTRA_CA_CERTS',
  // Token-based CLI login on hosts without a keychain.
  'CLAUDE_CODE_OAUTH_TOKEN', 'CLAUDE_CONFIG_DIR'
] as const

const SYSTEM_PROMPT_TEXT = 'You are a text-completion backend. Answer exactly what is asked, with no preamble and no tools.'
const SYSTEM_PROMPT_JSON =
  'You are a text-completion backend. Respond with valid JSON only. No markdown code fences and no explanation outside the JSON.'

export type CliCompletionInput = {
  prompt: string
  format: 'json' | 'text'
  /** Passed to --model only when the caller named one; otherwise the CLI picks its own default. */
  model?: string
  timeoutMs?: number
}

export type CliCompletionResult = {
  content: string
  usage?: AICompleteUsage
  model?: string
}

export type CliSessionDeps = {
  env?: NodeJS.ProcessEnv
  /** Overridable for tests that exercise the real process-spawning code with a real executable. */
  command?: string
}

// ── Concurrency gate ────────────────────────────────────────────────────────

class CliGate {
  private running = 0
  private readonly waiting: (() => void)[] = []

  async acquire(): Promise<() => void> {
    if (this.running < CLI_MAX_CONCURRENT) {
      this.running += 1
      return () => this.release()
    }
    if (this.waiting.length >= CLI_MAX_QUEUED) {
      throw cliError('ai.complete: the Claude CLI is busy, try again shortly', 'CLI_BUSY', true)
    }
    await new Promise<void>(resolve => this.waiting.push(resolve))
    this.running += 1
    return () => this.release()
  }

  private release(): void {
    this.running -= 1
    this.waiting.shift()?.()
  }
}

const gate = new CliGate()

// ── Errors ──────────────────────────────────────────────────────────────────

function cliError(message: string, reason: AICompleteErrorReason, retryable: boolean): AICompleteProviderError {
  return new AICompleteProviderError(message, { provider: CLI_PROVIDER_NAME, httpStatus: null, retryable, reason })
}

/** Maps the CLI's own error text (and the HTTP status it reports) to a reason the backend can act on. */
export function classifyCliFailure(resultText: string, apiErrorStatus: number | null): AICompleteErrorReason {
  const text = resultText.toLowerCase()
  if (/not logged in|please run \/login|run `?claude login|no credentials/.test(text)) {
    return 'CLI_NOT_LOGGED_IN'
  }
  if (apiErrorStatus === 401 || apiErrorStatus === 403 || /token (has )?expired|session (has )?expired|invalid (api key|authentication|credentials)|authentication_error/.test(text)) {
    return 'CLI_SESSION_EXPIRED'
  }
  if (apiErrorStatus === 429 || /usage limit|rate limit|quota|limit reached|overloaded_error/.test(text)) {
    return 'CLI_QUOTA_EXHAUSTED'
  }
  return 'CLI_FAILED'
}

// ── Flag support ────────────────────────────────────────────────────────────

const helpCache = new Map<string, { at: number; text: string }>()
const HELP_TTL_MS = 600_000

async function claudeHelp(env: NodeJS.ProcessEnv, command: string): Promise<string> {
  const key = `${command}\0${env['PATH'] ?? ''}`
  const hit = helpCache.get(key)
  if (hit && Date.now() - hit.at < HELP_TTL_MS) {
    return hit.text
  }
  try {
    const { stdout, stderr } = await execFileAsync(command, ['--help'], { env, timeout: 5_000, maxBuffer: 1024 * 1024, windowsHide: true })
    const text = `${stdout}\n${stderr}`
    helpCache.set(key, { at: Date.now(), text })
    return text
  } catch {
    throw cliError('ai.complete: the claude CLI is not installed or does not start', 'CLI_UNAVAILABLE', false)
  }
}

const hasFlag = (help: string, flag: string): boolean => new RegExp(`(?:^|[\\s,])${flag}(?=[\\s<=,]|$)`, 'm').test(help)

/** Whether this CLI build can be used safely: it must be able to turn every tool off. */
export async function cliSessionSupported(env: NodeJS.ProcessEnv = process.env, command = 'claude'): Promise<boolean> {
  try {
    return hasFlag(await claudeHelp(buildCliEnv(env), command), '--tools')
  } catch {
    return false
  }
}

// ── Process ─────────────────────────────────────────────────────────────────

export function buildCliEnv(source: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = {}
  for (const name of PASSTHROUGH_ENV) {
    const value = source[name]
    if (value !== undefined) {
      env[name] = value
    }
  }
  return env
}

type CliJson = {
  result?: unknown
  is_error?: unknown
  api_error_status?: unknown
  usage?: { input_tokens?: unknown; output_tokens?: unknown }
  modelUsage?: Record<string, unknown>
}

/** ORCA_AI_CLI_SESSION=off|0|false|no turns the CLI source off (ops kill switch); on by default. */
export function cliSessionEnabled(env: NodeJS.ProcessEnv = process.env): boolean {
  const v = (env['ORCA_AI_CLI_SESSION'] ?? '').trim().toLowerCase()
  return !(v === 'off' || v === '0' || v === 'false' || v === 'no')
}

export function buildCliArgs(input: CliCompletionInput, help: string): string[] {
  const args = ['-p', '--output-format', 'json', '--tools', '']
  if (hasFlag(help, '--no-session-persistence')) {
    args.push('--no-session-persistence')
  }
  args.push('--system-prompt', input.format === 'json' ? SYSTEM_PROMPT_JSON : SYSTEM_PROMPT_TEXT)
  if (input.model) {
    args.push('--model', input.model)
  }
  return args
}

export async function completeWithCliSession(input: CliCompletionInput, deps: CliSessionDeps = {}): Promise<CliCompletionResult> {
  const command = deps.command ?? 'claude'
  const env = buildCliEnv(deps.env ?? process.env)
  const help = await claudeHelp(env, command)
  if (!hasFlag(help, '--tools')) {
    throw cliError('ai.complete: this claude CLI cannot disable tools, refusing to run untrusted prompts', 'CLI_UNAVAILABLE', false)
  }

  const release = await gate.acquire()
  const scratch = await mkdtemp(join(tmpdir(), 'orca-ai-'))
  try {
    return await runCli(command, buildCliArgs(input, help), input, scratch, env)
  } finally {
    release()
    await rm(scratch, { recursive: true, force: true }).catch(() => undefined)
  }
}

function killTree(child: ReturnType<typeof spawn>): void {
  try {
    // Own process group on POSIX so helpers the CLI started cannot outlive the call or hold its pipes open.
    if (process.platform !== 'win32' && child.pid) {
      process.kill(-child.pid, 'SIGKILL')
      return
    }
  } catch {
    // fall through to killing the direct child
  }
  child.kill('SIGKILL')
}

function runCli(command: string, args: string[], input: CliCompletionInput, cwd: string, env: NodeJS.ProcessEnv): Promise<CliCompletionResult> {
  const timeoutMs = input.timeoutMs ?? CLI_DEFAULT_TIMEOUT_MS
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { cwd, env, stdio: ['pipe', 'pipe', 'pipe'], windowsHide: true, detached: process.platform !== 'win32' })
    const out: Buffer[] = []
    let outBytes = 0
    let settled = false
    const finish = (fn: () => void): void => {
      if (!settled) {
        settled = true
        clearTimeout(timer)
        fn()
      }
    }

    const timer = setTimeout(() => {
      killTree(child)
      finish(() => reject(cliError(`ai.complete: the claude CLI did not answer within ${Math.round(timeoutMs / 1000)}s`, 'PROVIDER_TIMEOUT', true)))
    }, timeoutMs)

    child.stdout.on('data', (chunk: Buffer) => {
      outBytes += chunk.length
      if (outBytes > CLI_MAX_OUTPUT_BYTES) {
        killTree(child)
        finish(() => reject(cliError('ai.complete: the claude CLI produced more output than allowed', 'CLI_FAILED', false)))
        return
      }
      out.push(chunk)
    })
    // stderr is drained and discarded: it can echo prompt fragments, which must not reach logs.
    child.stderr.on('data', () => undefined)

    child.on('error', err => {
      const missing = (err as NodeJS.ErrnoException).code === 'ENOENT'
      finish(() => reject(cliError(missing ? 'ai.complete: the claude CLI is not installed' : 'ai.complete: could not start the claude CLI', 'CLI_UNAVAILABLE', false)))
    })

    child.on('close', code => {
      // A throw inside an event handler would be uncaught, so failures must go through reject.
      finish(() => {
        try {
          resolve(parseCliOutput(Buffer.concat(out).toString('utf8'), code))
        } catch (err) {
          reject(err)
        }
      })
    })

    child.stdin.on('error', () => undefined)
    child.stdin.end(input.prompt)
  })
}

export function parseCliOutput(stdout: string, exitCode: number | null): CliCompletionResult {
  let parsed: CliJson
  try {
    parsed = JSON.parse(stdout) as CliJson
  } catch {
    throw cliError(`ai.complete: the claude CLI returned unreadable output (exit ${exitCode ?? 'signal'})`, 'CLI_FAILED', false)
  }
  const text = typeof parsed.result === 'string' ? parsed.result : ''
  if (parsed.is_error === true || (exitCode !== 0 && exitCode !== null)) {
    const status = typeof parsed.api_error_status === 'number' ? parsed.api_error_status : null
    const reason = classifyCliFailure(text, status)
    // The CLI's own message ("Not logged in · Please run /login") is safe to forward: it never contains the prompt.
    throw cliError(`ai.complete: claude CLI failed: ${text || 'no message'}`, reason, reason === 'CLI_QUOTA_EXHAUSTED')
  }
  const inputTokens = typeof parsed.usage?.input_tokens === 'number' ? parsed.usage.input_tokens : undefined
  const outputTokens = typeof parsed.usage?.output_tokens === 'number' ? parsed.usage.output_tokens : undefined
  const usage: AICompleteUsage | undefined =
    inputTokens !== undefined || outputTokens !== undefined
      ? {
          ...(inputTokens !== undefined ? { promptTokens: inputTokens } : {}),
          ...(outputTokens !== undefined ? { completionTokens: outputTokens } : {}),
          ...(inputTokens !== undefined && outputTokens !== undefined ? { totalTokens: inputTokens + outputTokens } : {})
        }
      : undefined
  const model = parsed.modelUsage ? Object.keys(parsed.modelUsage)[0] : undefined
  return { content: text, ...(usage ? { usage } : {}), ...(model ? { model } : {}) }
}
