// src/relay/agent-tool-registry.ts
// Tool registry for Orca Dev Agent — defines all available tools and discovers which
// are installed on the dev server.
//
// Design:
//   - All tool handlers receive (params, config: AgentConfig) — no module-level globals
//   - runToolCommand() always uses spawn with shell: false (no shell injection)
//   - discoverTools() is a pure function (no side effects, no caching)
//   - Built-in tools (binary=null) are always available regardless of PATH
//   - All tools return ToolResult { stdout, stderr, exitCode, meta? }

import { spawn } from 'node:child_process'
import { accessSync, constants } from 'node:fs'
import { readFile, readdir, stat } from 'node:fs/promises'
import { join, isAbsolute } from 'node:path'
import type { AgentConfig } from './agent-config'

// ─── Core types ───────────────────────────────────────────────────────────────

export interface ToolResult {
  stdout: string
  stderr: string
  exitCode: number
  meta?: Record<string, unknown>
}

export interface ToolInputSchema {
  type: 'object'
  properties: Record<string, {
    type: string
    description: string
    default?: unknown
    items?: { type: string }
  }>
  required?: readonly string[]
}

export interface ToolDefinition {
  readonly name: string
  /** null means built-in — always available, no binary check */
  readonly binary: string | null
  readonly description: string
  readonly inputSchema: ToolInputSchema
  handler(params: Record<string, unknown>, config: AgentConfig): Promise<ToolResult>
}

// ─── Core executor (shell: false — mandatory) ─────────────────────────────────

/**
 * Resolve a binary name to an absolute path by checking each directory
 * in the colon-separated toolPath. Returns binary as-is if not found
 * (spawn will fail with a clear ENOENT error, which is correct behavior).
 */
function resolveToolBinary(binary: string, toolPath: string): string {
  for (const dir of toolPath.split(':')) {
    if (!dir) continue
    const candidate = join(dir, binary)
    try {
      accessSync(candidate, constants.X_OK)
      return candidate
    } catch {
      // not found in this dir, try next
    }
  }
  return binary
}

/**
 * Spawn a process without a shell (shell: false) and capture stdout/stderr.
 * Kills with SIGTERM after timeout, returns exitCode 124 (same as bash timeout(1)).
 */
export function runToolCommand(
  binary: string,
  args: string[],
  opts: { cwd: string; timeout: number; env: NodeJS.ProcessEnv }
): Promise<ToolResult> {
  return new Promise((resolve) => {
    const resolved = resolveToolBinary(binary, opts.env.PATH ?? '')
    const child = spawn(resolved, args, {
      cwd:   opts.cwd,
      env:   opts.env,
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false,   // CRITICAL: no shell — prevents injection attacks
    })

    const stdout: string[] = []
    const stderr: string[] = []

    const timer = setTimeout(() => {
      child.kill('SIGTERM')
      resolve({
        stdout: stdout.join(''),
        stderr: `${stderr.join('')}\n[TIMEOUT: command exceeded ${opts.timeout}ms limit]`,
        exitCode: 124,
      })
    }, opts.timeout)

    child.stdout?.on('data', (chunk: Buffer) => stdout.push(chunk.toString()))
    child.stderr?.on('data', (chunk: Buffer) => stderr.push(chunk.toString()))

    child.on('close', (code) => {
      clearTimeout(timer)
      resolve({ stdout: stdout.join(''), stderr: stderr.join(''), exitCode: code ?? 0 })
    })

    child.on('error', (err) => {
      clearTimeout(timer)
      resolve({ stdout: '', stderr: err.message, exitCode: 1 })
    })

    child.stdin?.end()
  })
}

// ─── All tool definitions ─────────────────────────────────────────────────────

export const ALL_TOOL_DEFINITIONS: ToolDefinition[] = [

  // ── claude_code ──────────────────────────────────────────────────────────
  {
    name: 'claude_code',
    binary: 'claude',
    description: 'Run Claude Code AI assistant in --print mode (non-interactive). Streams output when done.',
    inputSchema: {
      type: 'object',
      properties: {
        prompt: { type: 'string', description: 'Task or question for Claude Code to execute' },
        cwd:    { type: 'string', description: 'Working directory (absolute). Defaults to AGENT_WORK_DIR.' },
        model:  { type: 'string', description: 'Claude model override (e.g. claude-opus-4-5). Empty = CLI default.', default: '' },
      },
      required: ['prompt'],
    },
    async handler(params, config) {
      const cwd  = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      const args = ['--print', String(params.prompt)]
      if (typeof params.model === 'string' && params.model) {
        args.unshift('--model', params.model)
      }
      return runToolCommand('claude', args, { cwd, timeout: 300_000, env: config.toolEnv })
    },
  },

  // ── gh ───────────────────────────────────────────────────────────────────
  {
    name: 'gh',
    binary: 'gh',
    description: 'GitHub CLI — manage PRs, issues, repos, gists, and more.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'gh subcommand and arguments (e.g. ["pr", "list"])' },
        cwd:  { type: 'string', description: 'Working directory (git repository)' },
      },
      required: ['args'],
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : []
      const cwd  = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      return runToolCommand('gh', args, { cwd, timeout: 60_000, env: config.toolEnv })
    },
  },

  // ── git ──────────────────────────────────────────────────────────────────
  {
    name: 'git',
    binary: 'git',
    description: 'Run git commands on the dev server. For UI-driven operations prefer the git.exec RPC method (has whitelist validation).',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'git arguments (e.g. ["log", "--oneline", "-10"])' },
        cwd:  { type: 'string', description: 'Working directory (git repository root)' },
      },
      required: ['args'],
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : []
      const cwd  = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      return runToolCommand('git', args, { cwd, timeout: 60_000, env: config.toolEnv })
    },
  },

  // ── gitnexus ─────────────────────────────────────────────────────────────
  {
    name: 'gitnexus',
    binary: 'gitnexus',
    description: 'GitNexus code intelligence CLI. Query the codebase graph, find symbols and their usages.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'gitnexus subcommand and arguments' },
        cwd:  { type: 'string', description: 'Working directory (project with .codegraph/ or gitnexus index)' },
      },
      required: ['args'],
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : []
      const cwd  = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      return runToolCommand('gitnexus', args, { cwd, timeout: 60_000, env: config.toolEnv })
    },
  },

  // ── codegraph ────────────────────────────────────────────────────────────
  {
    name: 'codegraph',
    binary: 'codegraph',
    description: 'CodeGraph local code analysis. Explore symbols, dependencies, and call graphs.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'codegraph subcommand and arguments' },
        cwd:  { type: 'string', description: 'Working directory (project with .codegraph/)' },
      },
      required: ['args'],
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : []
      const cwd  = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      return runToolCommand('codegraph', args, { cwd, timeout: 60_000, env: config.toolEnv })
    },
  },

  // ── docker ───────────────────────────────────────────────────────────────
  {
    name: 'docker',
    binary: 'docker',
    description: 'Run Docker CLI commands. Inspect containers, images, volumes, and logs.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'docker subcommand and arguments (e.g. ["ps", "-a"])' },
        cwd:  { type: 'string', description: 'Working directory' },
      },
      required: ['args'],
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : []
      const cwd  = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      return runToolCommand('docker', args, { cwd, timeout: 120_000, env: config.toolEnv })
    },
  },

  // ── shell ────────────────────────────────────────────────────────────────
  {
    name: 'shell',
    binary: 'bash',
    description: 'Run an arbitrary shell command on the dev server via bash -c. Output is captured and returned.',
    inputSchema: {
      type: 'object',
      properties: {
        command: { type: 'string', description: 'Shell command to execute (runs as: bash -c "<command>")' },
        cwd:     { type: 'string', description: 'Working directory' },
        timeout: { type: 'number', description: 'Timeout in milliseconds (default: 60000, max: 600000)', default: 60000 },
      },
      required: ['command'],
    },
    async handler(params, config) {
      const cwd     = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      const timeout = Math.min(typeof params.timeout === 'number' ? params.timeout : 60_000, 600_000)
      return runToolCommand('bash', ['-c', String(params.command)], { cwd, timeout, env: config.toolEnv })
    },
  },

  // ── read_file (built-in) ─────────────────────────────────────────────────
  {
    name: 'read_file',
    binary: null,   // built-in: always available, no binary check
    description: 'Read a file from the dev server filesystem. Supports line range selection.',
    inputSchema: {
      type: 'object',
      properties: {
        path:       { type: 'string', description: 'File path (absolute, or relative to AGENT_WORK_DIR)' },
        start_line: { type: 'number', description: 'First line to read, 1-indexed (default: 1)', default: 1 },
        end_line:   { type: 'number', description: 'Last line to read, inclusive (default: end of file)' },
      },
      required: ['path'],
    },
    async handler(params, config) {
      const filePath = typeof params.path === 'string' && isAbsolute(params.path)
        ? params.path
        : join(config.workDir, String(params.path ?? ''))
      try {
        const content = await readFile(filePath, 'utf8')
        const lines   = content.split('\n')
        const start   = Math.max(0, (typeof params.start_line === 'number' ? params.start_line : 1) - 1)
        const end     = typeof params.end_line === 'number' ? params.end_line : lines.length
        const slice   = lines.slice(start, end).join('\n')
        return {
          stdout: slice,
          stderr: '',
          exitCode: 0,
          meta: { path: filePath, totalLines: lines.length, startLine: start + 1, endLine: Math.min(end, lines.length) },
        }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return { stdout: '', stderr: msg, exitCode: 1 }
      }
    },
  },

  // ── list_dir (built-in) ──────────────────────────────────────────────────
  {
    name: 'list_dir',
    binary: null,   // built-in: always available, no binary check
    description: 'List directory contents on the dev server. Returns name, type, and size for each entry.',
    inputSchema: {
      type: 'object',
      properties: {
        path: { type: 'string', description: 'Directory path (absolute, or relative to AGENT_WORK_DIR)' },
      },
      required: ['path'],
    },
    async handler(params, config) {
      const dirPath = typeof params.path === 'string' && isAbsolute(params.path)
        ? params.path
        : join(config.workDir, String(params.path ?? ''))
      try {
        const entries = await readdir(dirPath, { withFileTypes: true })
        const list = await Promise.all(
          entries.map(async (e) => ({
            name: e.name,
            type: e.isDirectory() ? 'dir' : 'file',
            size: e.isFile() ? (await stat(join(dirPath, e.name))).size : null,
          }))
        )
        return { stdout: JSON.stringify(list, null, 2), stderr: '', exitCode: 0 }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return { stdout: '', stderr: msg, exitCode: 1 }
      }
    },
  },
]

// ─── Discovery ────────────────────────────────────────────────────────────────

/**
 * Discover which tools are available on this dev server by checking binary existence.
 * Pure function — no side effects, no caching.
 * Always includes built-in tools (binary=null).
 */
export async function discoverTools(config: AgentConfig): Promise<ToolDefinition[]> {
  const pathDirs = config.toolPath.split(':').filter(Boolean)
  const discovered: ToolDefinition[] = []

  for (const tool of ALL_TOOL_DEFINITIONS) {
    if (tool.binary === null) {
      // Built-in: always available
      discovered.push(tool)
      continue
    }

    // Check if binary exists and is executable in any toolPath directory
    const found = pathDirs.some((dir) => {
      try {
        accessSync(join(dir, tool.binary!), constants.X_OK)
        return true
      } catch {
        return false
      }
    })

    if (found) discovered.push(tool)
  }

  return discovered
}
