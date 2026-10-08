/**
 * agent-tool-use-command-summarizer.ts — FE-CV-TASK-089-02
 *
 * Reduces sampled agent tool uses to `commandsSummary` (CONTRACT ui-api 4.7).
 * Privacy: only a known program plus one allowlisted sub-command survives; args,
 * paths, URLs, env assignments, redirects and free-form words are dropped.
 * Never throws on arbitrary input.
 *
 * @module components/review-map/turns/agent-tool-use-command-summarizer
 */

export type CommandCategory = 'test' | 'lint' | 'typecheck' | 'build' | 'install' | 'git' | 'other'

export type NormalizedCommand = {
  name: string
  sub?: string
  category: CommandCategory
}

export type AgentTurnCommandsSummary = {
  v: 1
  totalToolUses: number
  commands: (NormalizedCommand & { count: number })[]
  toolCounts: Record<string, number>
  truncated: boolean
}

export const MAX_SUMMARY_COMMANDS = 20
export const MAX_TOOL_COUNT_KEYS = 32
const MAX_INPUT_CHARS = 512
const MAX_NAME_CHARS = 64

const PACKAGE_RUNNERS = new Set(['npm', 'pnpm', 'yarn', 'bun'])
// Why: a sub-command is only kept when it is a well-known verb; otherwise `echo hunter2`
// style arguments would leak as "sub-commands".
const SUBCOMMAND_ALLOWLIST: Record<string, ReadonlySet<string>> = {
  go: new Set(['test', 'build', 'vet', 'run', 'mod', 'generate', 'install']),
  cargo: new Set(['test', 'build', 'check', 'clippy', 'fmt', 'run', 'install', 'add']),
  git: new Set([
    'status', 'diff', 'add', 'commit', 'push', 'pull', 'fetch', 'checkout', 'switch', 'branch',
    'merge', 'rebase', 'log', 'stash', 'reset', 'restore', 'show', 'clone', 'tag'
  ]),
  make: new Set(['test', 'build', 'lint', 'check', 'all', 'install']),
  pip: new Set(['install']),
  dotnet: new Set(['test', 'build', 'restore', 'run']),
  gradle: new Set(['test', 'build', 'check']),
  mvn: new Set(['test', 'package', 'verify', 'install', 'compile'])
}
const PACKAGE_VERBS = new Set(['install', 'add', 'i', 'ci', 'test', 'run', 'exec', 'build', 'lint', 'dlx'])
const SCRIPT_NAMES = new Set(['test', 'lint', 'typecheck', 'build', 'check', 'dev', 'format', 'tsc'])

const TEST_PROGRAMS = new Set(['jest', 'vitest', 'pytest', 'mocha', 'jasmine', 'rspec', 'playwright'])
const LINT_PROGRAMS = new Set(['eslint', 'tslint', 'rubocop', 'flake8', 'pylint', 'oxlint', 'ruff', 'swiftlint', 'prettier', 'golangci-lint'])
const TYPECHECK_PROGRAMS = new Set(['tsc', 'mypy', 'pyright'])

const ENV_ASSIGNMENT = /^[A-Za-z_][A-Za-z0-9_]*=/
const SAFE_WORD = /^[a-z][a-z0-9-]*$/

function classify(name: string, sub?: string): CommandCategory {
  if (TEST_PROGRAMS.has(name)) {return 'test'}
  if (LINT_PROGRAMS.has(name)) {return 'lint'}
  if (TYPECHECK_PROGRAMS.has(name)) {return 'typecheck'}
  if (name === 'git') {return 'git'}
  if (sub === 'test') {return 'test'}
  if (sub === 'lint') {return 'lint'}
  if (sub === 'typecheck' || sub === 'check' || sub === 'vet') {return 'typecheck'}
  if (sub === 'build' || sub === 'compile' || sub === 'package') {return 'build'}
  if (sub === 'install' || sub === 'add' || sub === 'i' || sub === 'ci' || sub === 'restore') {return 'install'}
  return 'other'
}

function pickSubcommand(name: string, tokens: string[]): string | undefined {
  const rest = tokens.slice(1)
  const allowed = SUBCOMMAND_ALLOWLIST[name]
  if (PACKAGE_RUNNERS.has(name)) {
    const verb = rest.find((t) => PACKAGE_VERBS.has(t))
    if (!verb) {return undefined}
    if (verb === 'run' || verb === 'exec') {
      const script = rest.slice(rest.indexOf(verb) + 1).find((t) => SCRIPT_NAMES.has(t))
      return script ?? verb
    }
    return verb
  }
  if (allowed) {
    return rest.find((t) => allowed.has(t))
  }
  return undefined
}

/** Reduce one shell command line to program + allowlisted sub-command. */
export function normalizeShellCommand(raw: string): NormalizedCommand | null {
  try {
    const text = raw.slice(0, MAX_INPUT_CHARS).normalize('NFC').trim()
    if (!text) {return null}
    // Why: only the first simple command counts; `cd /tmp && rm -rf x` must not report `rm`.
    const first = text.split(/&&|\|\||[;|&<>]/)[0].trim()
    const tokens = first.split(/\s+/).filter(Boolean)
    // Skip leading env assignments (FOO=bar pnpm test) without recording them.
    while (tokens.length > 0 && ENV_ASSIGNMENT.test(tokens[0])) {tokens.shift()}
    const head = tokens[0]
    if (!head || /^https?:/i.test(head)) {return null}
    const name = head.replace(/^.*[/\\]/, '').toLowerCase()
    if (!name || !/^[a-z0-9][a-z0-9._+-]*$/.test(name) || name.length > MAX_NAME_CHARS) {return null}
    // Why: `cd` and shell builtins say nothing about verification.
    if (name === 'cd') {return null}
    const sub = pickSubcommand(name, tokens.map((t) => (SAFE_WORD.test(t) || t.startsWith('-') ? t : '')))
    return { name, ...(sub ? { sub } : {}), category: classify(name, sub) }
  } catch {
    return null
  }
}

function commandFromToolInput(toolInput: unknown): string | null {
  if (typeof toolInput === 'string') {return toolInput}
  if (typeof toolInput === 'object' && toolInput !== null) {
    const o = toolInput as Record<string, unknown>
    if (typeof o.command === 'string') {return o.command}
    if (typeof o.cmd === 'string') {return o.cmd}
  }
  return null
}

/** Only shell-like tools carry a command; other tools (Edit, Read) count as tools only. */
export function normalizeToolInput(toolName: string, toolInput: unknown): NormalizedCommand | null {
  try {
    if (!/^(bash|shell|terminal|exec|run_command|execute_command)$/i.test(toolName)) {return null}
    const raw = commandFromToolInput(toolInput)
    return raw ? normalizeShellCommand(raw) : null
  } catch {
    return null
  }
}

export type ToolUseObservation = {
  state: string
  updatedAt: number
  toolName?: string
  toolInput?: string
}

type PaneTally = {
  commands: Map<string, NormalizedCommand & { count: number }>
  toolCounts: Record<string, number>
  totalToolUses: number
  truncated: boolean
  lastKey: string
}

function emptyTally(): PaneTally {
  return { commands: new Map(), toolCounts: {}, totalToolUses: 0, truncated: false, lastKey: '' }
}

export type AgentTurnToolCollector = {
  observe(paneKey: string, entry: ToolUseObservation): void
  /** Returns the summary and clears the pane; null when nothing was observed. */
  take(paneKey: string): AgentTurnCommandsSummary | null
  reset(paneKey: string): void
}

export function createAgentTurnToolCollector(): AgentTurnToolCollector {
  const panes = new Map<string, PaneTally>()

  return {
    observe(paneKey, entry) {
      if (entry.state !== 'working' || !entry.toolName) {return}
      const tally = panes.get(paneKey) ?? emptyTally()
      panes.set(paneKey, tally)
      // Why: hook pings repeat the same tool at a new updatedAt; one use per distinct (tool, input).
      const key = `${entry.toolName}\u0000${entry.toolInput ?? ''}`
      if (key === tally.lastKey) {return}
      tally.lastKey = key

      const toolName = entry.toolName.slice(0, MAX_NAME_CHARS)
      if (toolName in tally.toolCounts || Object.keys(tally.toolCounts).length < MAX_TOOL_COUNT_KEYS) {
        tally.toolCounts[toolName] = (tally.toolCounts[toolName] ?? 0) + 1
      }
      tally.totalToolUses++

      const command = normalizeToolInput(entry.toolName, entry.toolInput)
      if (!command) {return}
      const id = `${command.name} ${command.sub ?? ''}`
      const existing = tally.commands.get(id)
      if (existing) {
        existing.count++
      } else if (tally.commands.size < MAX_SUMMARY_COMMANDS) {
        tally.commands.set(id, { ...command, count: 1 })
      } else {
        tally.truncated = true
      }
    },

    take(paneKey) {
      const tally = panes.get(paneKey)
      panes.delete(paneKey)
      if (!tally || tally.totalToolUses === 0) {return null}
      return {
        v: 1,
        totalToolUses: tally.totalToolUses,
        commands: [...tally.commands.values()],
        toolCounts: tally.toolCounts,
        truncated: tally.truncated
      }
    },

    reset(paneKey) {
      panes.delete(paneKey)
    }
  }
}
