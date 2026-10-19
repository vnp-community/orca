// src/relay/agent-capability-report.ts
// Builds the agent.capabilities report: probes installed tools, claude auth,
// env var presence, and host resource info.
//
// Security rules (CR-033 §2.6):
//   - Only tools in CAPABILITY_TOOL_ALLOWLIST are probed — never caller-supplied commands.
//   - env report contains only PRESENT/ABSENT booleans, never values.
//   - claude auth: only the loggedIn boolean, no email/org fields.

import { cliSessionEnabled } from './ai-complete-cli-session'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { detectClaudeFlags } from './agent-readonly-tool-policy'
import {
  CAPABILITY_TOOL_ALLOWLIST,
  DEFAULT_ENV_NAMES,
  validateCapabilityParams,
  type CapabilityReportParams
} from './agent-capability-params'
import { AGENT_BUILD_VERSION } from './agent-build-version'
import { AGENT_PROTOCOL_VERSION } from './agent-protocol-features'
import { probeKnowledgeSkills, type SkillEntry } from './agent-capability-skills'
import {
  buildHostInfo,
  createVersionRunner,
  readClaudeLogin,
  type HostInfo,
  type RunVersionCommand
} from './agent-capability-probes'
import type { statfs as StatFs } from 'node:fs/promises'

// ── Constants ──────────────────────────────────────────────────────────────────

// Re-exported so existing importers keep working.
export {
  CAPABILITY_TOOL_ALLOWLIST,
  DEFAULT_ENV_NAMES,
  ENV_NAME_PATTERN,
  validateCapabilityParams,
  type CapabilityReportParams
} from './agent-capability-params'

const OVERALL_BUDGET_MS = 8_000
const CACHE_TTL_MS = 60_000

// ── Types ──────────────────────────────────────────────────────────────────────

export type ToolEntry = {
  id: string
  installed: boolean | null
  version?: string
}

export type ClaudeEntry = {
  installed: boolean | null
  version?: string
  auth: 'logged_in' | 'logged_out' | 'unknown'
  flags?: {
    tools: boolean
    permissionMode: boolean
    disallowedTools: boolean
    mcpConfig?: boolean
    allowedTools?: boolean
  }
}

/** Where ai.complete can get credentials on this host (names only, never values). */
export type AiSource = 'env_key' | 'cli_session'

const AI_KEY_ENV_NAMES = ['ANTHROPIC_API_KEY', 'OPENAI_API_KEY', 'GOOGLE_API_KEY'] as const

/**
 * `cli_session` needs a logged-in claude that can run with every tool disabled, and the kill switch on.
 * An unknown login state (partial probe) is not claimed: absence means "not proven", not "absent".
 */
export function deriveAiSources(claude: ClaudeEntry, processEnv: NodeJS.ProcessEnv): AiSource[] {
  const sources: AiSource[] = []
  if (
    AI_KEY_ENV_NAMES.some((name) => typeof processEnv[name] === 'string' && processEnv[name] !== '')
  ) {
    sources.push('env_key')
  }
  if (
    cliSessionEnabled(processEnv) &&
    claude.installed === true &&
    claude.auth === 'logged_in' &&
    claude.flags?.tools === true
  ) {
    sources.push('cli_session')
  }
  return sources
}

export type CapabilityReport = {
  schemaVersion: number
  probedAt: string
  partial: boolean
  agent: { buildVersion: string; protocolVersion: number }
  tools: ToolEntry[]
  claude: ClaudeEntry
  aiSources: AiSource[]
  /** Knowledge-hub skills installed under ~/.claude/skills (name and hash only). */
  skills: SkillEntry[]
  host: HostInfo
  env: { name: string; present: boolean }[]
  unknownTools?: string[]
  rejectedEnvNames?: string[]
}

export type CapabilityReportDeps = {
  run?: RunVersionCommand
  now?: () => number
  platform?: NodeJS.Platform
  statfs?: typeof StatFs
  detectFlags?: typeof detectClaudeFlags
  env?: NodeJS.ProcessEnv
  homeDir?: string
}

// ── Cache ──────────────────────────────────────────────────────────────────────

type CacheEntry = { at: number; report: CapabilityReport }
const reportCache = new Map<string, CacheEntry>()
const inFlight = new Map<string, Promise<CapabilityReport>>()

// ── Main builder ───────────────────────────────────────────────────────────────

export async function buildCapabilityReport(
  params: Required<CapabilityReportParams> & {
    unknownTools?: string[]
    rejectedEnvNames?: string[]
  },
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
      const report = await _buildReport(params, config, deps)
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
  params: Required<CapabilityReportParams> & {
    unknownTools?: string[]
    rejectedEnvNames?: string[]
  },
  config: AgentConfig,
  deps: CapabilityReportDeps
): Promise<CapabilityReport> {
  const platform = deps.platform ?? process.platform
  const processEnv = deps.env ?? process.env
  const toolEnv = { ...processEnv, PATH: config.toolPath }
  const detectFlags = deps.detectFlags ?? detectClaudeFlags

  const runCmd = deps.run ?? createVersionRunner(platform)

  const VERSION_RE = /\d+(?:\.\d+){1,3}[\w.+-]*/

  let partial = false
  const probedAt = new Date().toISOString()

  // ── Tool probes (parallel, with overall budget) ────────────────────────────

  const allowlistMap = new Map(CAPABILITY_TOOL_ALLOWLIST.map((t) => [t.id, t.args]))
  const toolIds = (params.tools ?? CAPABILITY_TOOL_ALLOWLIST.map((t) => t.id)).filter((id) =>
    allowlistMap.has(id)
  )

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
        return version !== undefined ? { id, installed: true, version } : { id, installed: true }
      })
    ),
    new Promise<null>((resolve) => setTimeout(() => resolve(null), OVERALL_BUDGET_MS))
  ])

  let tools: ToolEntry[]
  if (toolResults === null) {
    // Budget exceeded — mark all as null (unknown)
    partial = true
    tools = toolIds.map((id) => ({ id, installed: null }))
  } else {
    tools = toolResults
    if (tools.some((t) => t.installed === null)) {
      partial = true
    }
  }

  // ── Claude details ─────────────────────────────────────────────────────────

  const claudeTool = tools.find((t) => t.id === 'claude')
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

    claudeEntry = { ...claudeEntry, auth: await readClaudeLogin(runCmd, toolEnv) }
  }

  // ── Host info ──────────────────────────────────────────────────────────────

  const hostInfo = await buildHostInfo(config.workDir, platform, deps.statfs)

  // ── Env presence (never values) ────────────────────────────────────────────

  const envNames = params.envNames ?? [...DEFAULT_ENV_NAMES]
  const envReport = envNames.map((name) => ({
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
    tools: tools.filter((t) => t.id !== 'claude'),
    claude: claudeEntry,
    aiSources: deriveAiSources(claudeEntry, processEnv),
    skills: await probeKnowledgeSkills(deps.homeDir),
    host: hostInfo,
    env: envReport,
    ...(params.unknownTools && params.unknownTools.length > 0
      ? { unknownTools: params.unknownTools }
      : {}),
    ...(params.rejectedEnvNames && params.rejectedEnvNames.length > 0
      ? { rejectedEnvNames: params.rejectedEnvNames }
      : {})
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

  const installedCount = report.tools.filter((t) => t.installed === true).length
  log.info(
    `agent.capabilities: ${installedCount}/${report.tools.length} tools installed ` +
      `partial=${report.partial} probeMs=${probeMs}`
  )

  return { jsonrpc: '2.0', id, result: report }
}
