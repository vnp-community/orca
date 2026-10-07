// src/relay/agent-capability-report.ts
// Builds the agent.capabilities report: probes installed tools, claude auth,
// env var presence, and host resource info.
//
// Security rules (CR-033 §2.6):
//   - Only tools in CAPABILITY_TOOL_ALLOWLIST are probed — never caller-supplied commands.
//   - env report contains only PRESENT/ABSENT booleans, never values.
//   - claude auth: only the loggedIn boolean, no email/org fields.

import * as os from 'node:os'
import { execFile as _execFile } from 'node:child_process'
import { promisify } from 'node:util'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { detectClaudeFlags } from './agent-readonly-tool-policy'
import { AGENT_BUILD_VERSION } from './agent-build-version'
import { AGENT_PROTOCOL_VERSION } from './agent-protocol-features'

const execFileAsync = promisify(_execFile)

// ── Constants ──────────────────────────────────────────────────────────────────

export const CAPABILITY_TOOL_ALLOWLIST: ReadonlyArray<{ id: string; args: readonly string[] }> = [
  { id: 'go', args: ['version'] },
  { id: 'node', args: ['--version'] },
  { id: 'pnpm', args: ['--version'] },
  { id: 'npm', args: ['--version'] },
  { id: 'git', args: ['--version'] },
  { id: 'openspec', args: ['--version'] },
  { id: 'claude', args: ['--version'] },
  { id: 'codegraph', args: ['--version'] },
  { id: 'gitnexus', args: ['--version'] },
  { id: 'rg', args: ['--version'] },
  { id: 'make', args: ['--version'] },
  { id: 'semgrep', args: ['--version'] }
]

export const DEFAULT_ENV_NAMES = [
  'ANTHROPIC_API_KEY',
  'OPENAI_API_KEY',
  'GOOGLE_API_KEY'
] as const

export const ENV_NAME_PATTERN = /^[A-Z][A-Z0-9_]{0,63}$/

const OVERALL_BUDGET_MS = 8_000
const TOOL_TIMEOUT_MS = 3_000
const CACHE_TTL_MS = 60_000

// ── Types ──────────────────────────────────────────────────────────────────────

export type CapabilityReportParams = {
  tools?: string[]
  envNames?: string[]
  refresh?: boolean
}

export type ToolEntry = {
  id: string
  installed: boolean | null
  version?: string
}

export type ClaudeEntry = {
  installed: boolean | null
  version?: string
  auth: 'logged_in' | 'logged_out' | 'unknown'
  flags?: { tools: boolean; permissionMode: boolean; disallowedTools: boolean }
}

export type CapabilityReport = {
  schemaVersion: number
  probedAt: string
  partial: boolean
  agent: { buildVersion: string; protocolVersion: number }
  tools: ToolEntry[]
  claude: ClaudeEntry
  host: {
    platform: string
    arch: string
    nodeVersion: string
    cpuCount: number
    memTotalMb: number
    memFreeMb: number
    diskFreeMb?: number
    loadAvg1: number
  }
  env: Array<{ name: string; present: boolean }>
  unknownTools?: string[]
  rejectedEnvNames?: string[]
}

export type CapabilityReportDeps = {
  run?: RunVersionCommand
  now?: () => number
  platform?: NodeJS.Platform
  statfs?: typeof import('node:fs/promises').statfs
  detectFlags?: typeof detectClaudeFlags
  env?: NodeJS.ProcessEnv
}

type RunVersionCommand = (
  id: string,
  args: readonly string[],
  env: NodeJS.ProcessEnv
) => Promise<{ stdout: string } | null>

// ── Validation ─────────────────────────────────────────────────────────────────

export function validateCapabilityParams(raw: Record<string, unknown>): {
  ok: true
  value: Required<CapabilityReportParams> & { unknownTools: string[]; rejectedEnvNames: string[] }
} | { ok: false; code: 'INVALID_CAPABILITY_PARAMS' | 'TOO_MANY_ENV_NAMES'; message: string } {
  if (raw['tools'] !== undefined && !Array.isArray(raw['tools'])) {
    return { ok: false, code: 'INVALID_CAPABILITY_PARAMS', message: 'tools must be an array of strings' }
  }
  if (raw['tools'] && (raw['tools'] as unknown[]).some(t => typeof t !== 'string')) {
    return { ok: false, code: 'INVALID_CAPABILITY_PARAMS', message: 'tools must be an array of strings' }
  }
  if (raw['envNames'] !== undefined && !Array.isArray(raw['envNames'])) {
    return { ok: false, code: 'INVALID_CAPABILITY_PARAMS', message: 'envNames must be an array of strings' }
  }
  if (raw['envNames'] && (raw['envNames'] as unknown[]).some(n => typeof n !== 'string')) {
    return { ok: false, code: 'INVALID_CAPABILITY_PARAMS', message: 'envNames must be an array of strings' }
  }
  if (raw['refresh'] !== undefined && typeof raw['refresh'] !== 'boolean') {
    return { ok: false, code: 'INVALID_CAPABILITY_PARAMS', message: 'refresh must be a boolean' }
  }

  const envNames: string[] = (raw['envNames'] as string[] | undefined) ?? [...DEFAULT_ENV_NAMES]
  if (envNames.length > 64) {
    return { ok: false, code: 'TOO_MANY_ENV_NAMES', message: `envNames must have at most 64 items, got ${envNames.length}` }
  }

  const requestedTools: string[] = (raw['tools'] as string[] | undefined) ?? CAPABILITY_TOOL_ALLOWLIST.map(t => t.id)
  const allowedIds = new Set(CAPABILITY_TOOL_ALLOWLIST.map(t => t.id))
  const validTools = requestedTools.filter(t => allowedIds.has(t))
  const unknownTools = requestedTools.filter(t => !allowedIds.has(t))

  const validEnvNames = envNames.filter(n => ENV_NAME_PATTERN.test(n))
  const rejectedEnvNames = envNames.filter(n => !ENV_NAME_PATTERN.test(n))

  return {
    ok: true,
    value: {
      tools: validTools,
      envNames: validEnvNames,
      refresh: (raw['refresh'] as boolean | undefined) ?? false,
      unknownTools,
      rejectedEnvNames
    }
  }
}

// ── Cache ──────────────────────────────────────────────────────────────────────

type CacheEntry = { at: number; report: CapabilityReport }
const reportCache = new Map<string, CacheEntry>()
const inFlight = new Map<string, Promise<CapabilityReport>>()

// ── Main builder ───────────────────────────────────────────────────────────────

export async function buildCapabilityReport(
  params: Required<CapabilityReportParams> & { unknownTools?: string[]; rejectedEnvNames?: string[] },
  config: AgentConfig,
  deps: CapabilityReportDeps = {}
): Promise<CapabilityReport> {
  const now = deps.now ?? Date.now
  const cacheKey = JSON.stringify([
    [...(params.tools ?? [])].sort(),
    [...(params.envNames ?? [])].sort()
  ])

  if (!params.refresh) {
    const cached = reportCache.get(cacheKey)
    if (cached && now() - cached.at < CACHE_TTL_MS) {
      return cached.report
    }
  }

  const existing = inFlight.get(cacheKey)
  if (existing && !params.refresh) {
    return existing
  }

  const promise = (async (): Promise<CapabilityReport> => {
    try {
      const report = await _buildReport(params, config, deps, now)
      reportCache.set(cacheKey, { at: now(), report })
      return report
    } finally {
      inFlight.delete(cacheKey)
    }
  })()

  inFlight.set(cacheKey, promise)
  return promise
}

async function _buildReport(
  params: Required<CapabilityReportParams> & { unknownTools?: string[]; rejectedEnvNames?: string[] },
  config: AgentConfig,
  deps: CapabilityReportDeps,
  now: () => number
): Promise<CapabilityReport> {
  const platform = deps.platform ?? process.platform
  const processEnv = deps.env ?? process.env
  const toolEnv = { ...processEnv, PATH: config.toolPath }
  const detectFlags = deps.detectFlags ?? detectClaudeFlags

  const runCmd = deps.run ?? async function defaultRun(
    id: string,
    args: readonly string[],
    env: NodeJS.ProcessEnv
  ): Promise<{ stdout: string } | null> {
    try {
      const { stdout } = await execFileAsync(id, [...args], {
        env,
        timeout: TOOL_TIMEOUT_MS,
        windowsHide: true,
        maxBuffer: 1024 * 1024
      })
      return { stdout }
    } catch (err: unknown) {
      const e = err as { code?: string }
      if (e.code === 'ENOENT') {
        // On Windows, try .cmd extension
        if (platform === 'win32') {
          try {
            const { stdout: stdout2 } = await execFileAsync(`${id}.cmd`, [...args], {
              env,
              timeout: TOOL_TIMEOUT_MS,
              windowsHide: true,
              maxBuffer: 1024 * 1024
            })
            return { stdout: stdout2 }
          } catch {
            // fall through
          }
        }
        return null  // not installed
      }
      return { stdout: '' }  // installed but timed out or errored → null means not installed
    }
  }

  const VERSION_RE = /\d+(?:\.\d+){1,3}[\w.+-]*/

  let partial = false
  const probedAt = new Date().toISOString()

  // ── Tool probes (parallel, with overall budget) ────────────────────────────

  const allowlistMap = new Map(CAPABILITY_TOOL_ALLOWLIST.map(t => [t.id, t.args]))
  const toolIds = (params.tools ?? CAPABILITY_TOOL_ALLOWLIST.map(t => t.id))
    .filter(id => allowlistMap.has(id))

  const toolResults = await Promise.race([
    Promise.all(
      toolIds.map(async (id): Promise<ToolEntry> => {
        const args = allowlistMap.get(id)!
        const result = await runCmd(id, args, toolEnv)
        if (result === null) {
          return { id, installed: false }
        }
        // Extract version from first line
        const firstLine = result.stdout.split('\n')[0] ?? ''
        const match = VERSION_RE.exec(firstLine)
        const version = match ? match[0]!.slice(0, 64) : undefined
        return version !== undefined
          ? { id, installed: true, version }
          : { id, installed: true }
      })
    ),
    new Promise<null>((resolve) => setTimeout(() => resolve(null), OVERALL_BUDGET_MS))
  ])

  let tools: ToolEntry[]
  if (toolResults === null) {
    // Budget exceeded — mark all as null (unknown)
    partial = true
    tools = toolIds.map(id => ({ id, installed: null }))
  } else {
    tools = toolResults
    if (tools.some(t => t.installed === null)) {
      partial = true
    }
  }

  // ── Claude details ─────────────────────────────────────────────────────────

  const claudeTool = tools.find(t => t.id === 'claude')
  let claudeEntry: ClaudeEntry = {
    installed: claudeTool?.installed ?? null,
    auth: 'unknown'
  }
  if (claudeTool?.version) {
    claudeEntry.version = claudeTool.version
  }

  if (claudeTool?.installed) {
    // Detect flags (uses cache from agent-readonly-tool-policy)
    const flags = await detectFlags(toolEnv)
    if (flags) {
      claudeEntry = { ...claudeEntry, flags }
    }

    // Auth status — only read loggedIn boolean, never email/org
    try {
      const authResult = await runCmd('claude', ['auth', 'status', '--json'], toolEnv)
      if (authResult) {
        try {
          const parsed = JSON.parse(authResult.stdout.trim()) as Record<string, unknown>
          if (typeof parsed['loggedIn'] === 'boolean') {
            claudeEntry = {
              ...claudeEntry,
              auth: parsed['loggedIn'] ? 'logged_in' : 'logged_out'
            }
          }
        } catch {
          claudeEntry = { ...claudeEntry, auth: 'unknown' }
        }
      }
    } catch {
      claudeEntry = { ...claudeEntry, auth: 'unknown' }
    }
  }

  // ── Host info ──────────────────────────────────────────────────────────────

  const cpus = os.cpus()
  const memInfo = process.memoryUsage ? undefined : undefined  // placeholder
  const totalMemBytes = os.totalmem()
  const freeMemBytes = os.freemem()

  let diskFreeMb: number | undefined
  if (deps.statfs) {
    try {
      const statfs = await deps.statfs(config.workDir)
      diskFreeMb = Math.floor((statfs.bavail * statfs.bsize) / (1024 * 1024))
    } catch {
      // omit diskFreeMb on error
    }
  } else {
    try {
      const { statfs } = await import('node:fs/promises')
      const s = await statfs(config.workDir)
      diskFreeMb = Math.floor((s.bavail * s.bsize) / (1024 * 1024))
    } catch {
      // omit diskFreeMb on error
    }
  }

  const hostInfo = {
    platform: platform as string,
    arch: os.arch(),
    nodeVersion: process.version,
    cpuCount: cpus.length,
    memTotalMb: Math.floor(totalMemBytes / (1024 * 1024)),
    memFreeMb: Math.floor(freeMemBytes / (1024 * 1024)),
    ...(diskFreeMb !== undefined ? { diskFreeMb } : {}),
    loadAvg1: os.loadavg()[0] ?? 0
  }

  // ── Env presence (never values) ────────────────────────────────────────────

  const envNames = params.envNames ?? [...DEFAULT_ENV_NAMES]
  const envReport = envNames.map(name => ({
    name,
    // Read from processEnv (= process.env) — what ai.complete actually sees
    present: typeof processEnv[name] === 'string' && processEnv[name] !== ''
  }))

  return {
    schemaVersion: 1,
    probedAt,
    partial,
    agent: {
      buildVersion: AGENT_BUILD_VERSION,
      protocolVersion: AGENT_PROTOCOL_VERSION
    },
    tools: tools.filter(t => t.id !== 'claude'),
    claude: claudeEntry,
    host: hostInfo,
    env: envReport,
    ...(params.unknownTools && params.unknownTools.length > 0 ? { unknownTools: params.unknownTools } : {}),
    ...(params.rejectedEnvNames && params.rejectedEnvNames.length > 0 ? { rejectedEnvNames: params.rejectedEnvNames } : {})
  }
}

// ── RPC handler ────────────────────────────────────────────────────────────────

export async function handleAgentCapabilities(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const validated = validateCapabilityParams(params)
  if (!validated.ok) {
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: `agent.capabilities: ${validated.message} (${validated.code})`,
        data: { reason: validated.code }
      }
    }
  }

  const startedAt = Date.now()
  const report = await buildCapabilityReport(validated.value, config)
  const probeMs = Date.now() - startedAt

  const installedCount = report.tools.filter(t => t.installed === true).length
  log.info(
    `agent.capabilities: ${installedCount}/${report.tools.length} tools installed ` +
    `partial=${report.partial} probeMs=${probeMs}`
  )

  return { jsonrpc: '2.0', id, result: report }
}
