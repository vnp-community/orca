/**
 * agent-tool-use-command-summarizer.ts — FE-CV-TASK-089-02
 *
 * Normalizes and collects tool command usage per agent pane.
 * Safety rules:
 * - Keep only program name + optional one sub-command
 * - Strip args, paths, env vars, URLs, redirects
 * - Never throw on arbitrary input
 * - Cap at 20 commands, 32 toolCounts keys
 *
 * @module components/review-map/turns/agent-tool-use-command-summarizer
 */

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type CommandCategory =
  | 'test'
  | 'lint'
  | 'typecheck'
  | 'build'
  | 'install'
  | 'git'
  | 'other'

export type NormalizedCommand = {
  program: string
  subcommand?: string
  category: CommandCategory
}

export type AgentTurnCommandsSummary = {
  commands: NormalizedCommand[]
  toolCounts: Record<string, number>
  truncated: boolean
}

export type ToolUseObservation = {
  state: 'working' | string
  updatedAt: string | number
  toolName: string
  toolInput: unknown
}

// ---------------------------------------------------------------------------
// Category classification
// ---------------------------------------------------------------------------

const CATEGORY_MAP: Array<[RegExp, CommandCategory]> = [
  [/^(jest|vitest|pytest|mocha|jasmine|rspec|cargo\s+test|go\s+test|dotnet\s+test)/, 'test'],
  [/^(eslint|tslint|rubocop|flake8|pylint|oxlint|ruff|swiftlint)/, 'lint'],
  [/^(tsc|mypy|pyright|flow)/, 'typecheck'],
  [/^(npm|pnpm|yarn|cargo|gradle|maven|make|cmake)\s+(build|compile|bundle|pack)/, 'build'],
  [/^(npm|pnpm|yarn|pip|gem|cargo)\s+(install|add|get|fetch|update|upgrade)/, 'install'],
  [/^git\b/, 'git'],
]

function classifyCategory(program: string, subcommand?: string): CommandCategory {
  const combined = subcommand ? `${program} ${subcommand}` : program
  for (const [pattern, cat] of CATEGORY_MAP) {
    if (pattern.test(combined)) return cat
  }
  return 'other'
}

// ---------------------------------------------------------------------------
// Input sanitizer
// ---------------------------------------------------------------------------

const ARGS_STRIP = /\s+(-{1,2}\S+|[<>|&;]|\S+=\S+|https?:\/\/\S+|\$\w+|'\S+'|"\S+")[\s$]*/g
const URL_PATTERN = /^https?:\/\//i
const ENV_PATTERN = /^\w+=\S+/
const PATH_PATTERN = /^[./~]|\\\\/

function safeStr(v: unknown): string {
  if (typeof v !== 'string') return ''
  return v.slice(0, 512) // cap input length for safety
}

/**
 * Normalize a shell command string.
 * Extracts program + optional first sub-command; strips everything else.
 */
export function normalizeShellCommand(raw: string): NormalizedCommand | null {
  try {
    // Strip common dangerous/sensitive patterns
    const s = raw.normalize('NFC').trim()
    if (!s) return null

    // Tokenize by whitespace (minimal — no shell grammar)
    const tokens = s.split(/\s+/)
    if (!tokens.length) return null

    const programRaw = tokens[0]

    // Reject URLs, env assignments, and redirects
    if (!programRaw || URL_PATTERN.test(programRaw) || ENV_PATTERN.test(programRaw)) return null
    if ([';', '|', '&', '<', '>'].includes(programRaw[0])) return null

    // Strip path prefix from program name
    const program = programRaw.replace(/^.*[/\\]/, '').toLowerCase()
    if (!program) return null

    // Candidate sub-command: next non-flag, non-path, non-env, non-URL token
    let subcommand: string | undefined
    for (let i = 1; i < tokens.length && i <= 3; i++) {
      const t = tokens[i]
      if (!t || t.startsWith('-') || PATH_PATTERN.test(t) || ENV_PATTERN.test(t) || URL_PATTERN.test(t)) continue
      // Accept simple alphanumeric subcommand only
      if (/^[a-z][a-z0-9-]*$/.test(t)) {
        subcommand = t
        break
      }
    }

    const category = classifyCategory(program, subcommand)
    return { program, subcommand, category }
  } catch {
    return null
  }
}

/**
 * Extract command string from a tool input.
 * Handles {command: string}, {cmd: string}, {args: string[]}, string.
 */
export function extractCommandFromToolInput(toolInput: unknown): string | null {
  if (typeof toolInput === 'string') return toolInput
  if (typeof toolInput !== 'object' || toolInput === null) return null
  const o = toolInput as Record<string, unknown>
  if (typeof o.command === 'string') return o.command
  if (typeof o.cmd === 'string') return o.cmd
  if (Array.isArray(o.args)) return o.args.map(String).join(' ')
  return null
}

/**
 * Normalize a tool input into a command.
 * Returns null if not extractable or the result is empty.
 */
export function normalizeToolInput(
  toolName: string,
  toolInput: unknown
): NormalizedCommand | null {
  try {
    const raw = extractCommandFromToolInput(toolInput)
    if (!raw) {
      // Use tool name as program when no command extractable
      return { program: safeStr(toolName).toLowerCase().slice(0, 64), category: 'other' }
    }
    return normalizeShellCommand(raw)
  } catch {
    return null
  }
}

// ---------------------------------------------------------------------------
// Collector
// ---------------------------------------------------------------------------

const MAX_COMMANDS = 20
const MAX_TOOL_COUNTS_KEYS = 32

export type AgentTurnToolCollector = {
  /** Observe a new tool use observation (call on each setAgentStatus) */
  observe(obs: ToolUseObservation): void
  /** Take the current summary and reset */
  take(): AgentTurnCommandsSummary
  /** Reset without taking */
  reset(): void
}

/**
 * Create a tool collector for a pane.
 * Counts unique tool uses across a turn. Deduplicates consecutive same-tool pings.
 */
export function createAgentTurnToolCollector(): AgentTurnToolCollector {
  let commands: NormalizedCommand[] = []
  let toolCounts: Record<string, number> = {}
  let truncated = false
  let lastToolKey = ''
  let lastUpdatedAt: string | number = ''

  return {
    observe(obs) {
      // Only count when working and updatedAt advanced
      if (obs.state !== 'working') return
      if (obs.updatedAt === lastUpdatedAt && obs.toolName === lastToolKey) return

      const key = obs.toolName
      const normalized = normalizeToolInput(obs.toolName, obs.toolInput)

      // Tool counts
      if (Object.keys(toolCounts).length < MAX_TOOL_COUNTS_KEYS) {
        toolCounts[key] = (toolCounts[key] ?? 0) + 1
      } else if (key in toolCounts) {
        toolCounts[key]++
      }

      // Commands list
      if (normalized && commands.length < MAX_COMMANDS) {
        commands.push(normalized)
      } else if (normalized && commands.length >= MAX_COMMANDS) {
        truncated = true
      }

      lastToolKey = obs.toolName
      lastUpdatedAt = obs.updatedAt
    },

    take() {
      const summary: AgentTurnCommandsSummary = {
        commands: [...commands],
        toolCounts: { ...toolCounts },
        truncated
      }
      this.reset()
      return summary
    },

    reset() {
      commands = []
      toolCounts = {}
      truncated = false
      lastToolKey = ''
      lastUpdatedAt = ''
    }
  }
}
