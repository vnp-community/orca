// src/relay/agent-readonly-tool-policy.ts
// Read-only tool policy for agent.execPrompt: probes claude's supported flags
// via `claude --help` and builds the readonly argument list.
// Cache is keyed by PATH to handle different claude binaries on the same host.

import { execFile as _execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

const execFileAsync = promisify(_execFile)

// ── Constants ──────────────────────────────────────────────────────────────────

// UNVERIFIED tool names — confirmed once real e2e test runs (task 04)
export const READONLY_DEFAULT_TOOLS = ['Read', 'Glob', 'Grep'] as const
export const CLAUDE_FLAGS_TTL_MS = 600_000   // 10 minutes
export const CLAUDE_HELP_TIMEOUT_MS = 5_000

// ── Types ──────────────────────────────────────────────────────────────────────

export type ClaudeFlagSupport = {
  tools: boolean
  permissionMode: boolean
  disallowedTools: boolean
}

export type ClaudeHelpProbe = (env: NodeJS.ProcessEnv) => Promise<string>

export type ReadonlyUnsupportedReason =
  | 'CLAUDE_HELP_UNAVAILABLE'
  | 'FLAG_TOOLS_MISSING'
  | 'FLAG_PERMISSION_MODE_MISSING'

// ── Default probe ──────────────────────────────────────────────────────────────

export async function defaultClaudeHelpProbe(env: NodeJS.ProcessEnv): Promise<string> {
  const { stdout, stderr } = await execFileAsync('claude', ['--help'], {
    env,
    timeout: CLAUDE_HELP_TIMEOUT_MS,
    maxBuffer: 1024 * 1024,
    windowsHide: true
  })
  // Some CLIs print help to stderr
  return `${stdout}\n${stderr}`
}

// ── Flag detection with caching ───────────────────────────────────────────────

type CacheEntry = { at: number; flags: ClaudeFlagSupport }
const flagCache = new Map<string, CacheEntry>()
// In-flight promises for single-flight per PATH key
const inFlight = new Map<string, Promise<ClaudeFlagSupport | null>>()

export async function detectClaudeFlags(
  env: NodeJS.ProcessEnv,
  probe: ClaudeHelpProbe = defaultClaudeHelpProbe,
  now: () => number = Date.now
): Promise<ClaudeFlagSupport | null> {
  const key = env['PATH'] ?? ''
  const cached = flagCache.get(key)
  if (cached && now() - cached.at < CLAUDE_FLAGS_TTL_MS) {
    return cached.flags
  }

  // Single-flight: concurrent detections for same key share one probe
  const existing = inFlight.get(key)
  if (existing) {
    return existing
  }

  const promise = (async (): Promise<ClaudeFlagSupport | null> => {
    try {
      const helpText = await probe(env)
      const flags: ClaudeFlagSupport = {
        tools: /(?:^|[\s,])--tools(?=[\s<=,]|$)/m.test(helpText),
        permissionMode: /(?:^|[\s,])--permission-mode(?=[\s<=,]|$)/m.test(helpText),
        disallowedTools: /(?:^|[\s,])--(?:disallowedTools|disallowed-tools)(?=[\s<=,]|$)/m.test(helpText)
      }
      // Only cache successes; failed probes are retried next call
      flagCache.set(key, { at: now(), flags })
      return flags
    } catch {
      return null
    } finally {
      inFlight.delete(key)
    }
  })()

  inFlight.set(key, promise)
  return promise
}

// ── Reason classification ──────────────────────────────────────────────────────

export function readonlyUnsupportedReason(
  flags: ClaudeFlagSupport | null
): ReadonlyUnsupportedReason | null {
  if (flags === null) { return 'CLAUDE_HELP_UNAVAILABLE' }
  if (!flags.tools) { return 'FLAG_TOOLS_MISSING' }
  if (!flags.permissionMode) { return 'FLAG_PERMISSION_MODE_MISSING' }
  return null
}

// ── Argument builder ───────────────────────────────────────────────────────────

export function buildReadonlyArgs(): string[] {
  // Tools joined with comma because --tools accepts a variadic list and we
  // don't want the flag parser to consume subsequent positional args.
  // UNVERIFIED: if claude only accepts space-separated, switch to three elements
  // at the end of argv (task 04 adversarial e2e will confirm).
  return ['--permission-mode', 'plan', '--tools', READONLY_DEFAULT_TOOLS.join(',')]
}

// ── Error response builder ─────────────────────────────────────────────────────

export function readonlyUnsupportedError(
  id: string | number | null,
  method: 'agent.execPrompt' | 'agent.execPromptStream',
  reason: ReadonlyUnsupportedReason
): object {
  return {
    jsonrpc: '2.0',
    id,
    error: {
      code: AgentErrorCode.InvalidParams,
      message: `${method}: READONLY_MODE_UNSUPPORTED (${reason})`,
      data: {
        // data.reason = class-level code; data.detail = specific reason
        reason: 'READONLY_MODE_UNSUPPORTED',
        detail: reason
      }
    }
  }
}

// ── Test helper ───────────────────────────────────────────────────────────────

/** Only for use in tests — clears the in-process flag cache. */
export function resetClaudeFlagCacheForTests(): void {
  flagCache.clear()
  inFlight.clear()
}
