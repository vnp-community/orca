# SOL-04: agent-tool-registry.ts — Tool Registry

**TDD Ref:** TDD-AG-05  
**File:** `src/relay/agent-tool-registry.ts` [NEW]  
**Mức độ:** 🟡 Trung bình  
**Thời gian ước tính:** 2h

---

## Vấn đề

Agent v1 dùng `TOOL_DEFINITIONS: any[]` với inline handlers, global `WORK_DIR`, global `TOOL_ENV`.  
Agent v2.1:
1. Typed `ToolDefinition<TParams>` generic interface
2. Handlers nhận `config: AgentConfig` (dependency injection)
3. `discoverTools(config)` → pure function, returns `ToolDefinition[]`
4. **REUSE** `AgentExecHandler` logic từ `agent-exec-handler.ts` (existing)

---

## Vấn đề với AgentExecHandler (existing)

`AgentExecHandler` trong codebase **là class** với `constructor(dispatcher: RelayDispatcher)` — thiết kế cho relay daemon, không phù hợp trực tiếp với agent.

**Giải pháp:** Tạo `runToolCommand()` wrapper function dùng `spawn` trực tiếp (giống v1):

```typescript
// src/relay/agent-tool-registry.ts — runToolCommand() wrapper

import { spawn } from 'node:child_process'
import { accessSync, constants } from 'node:fs'
import { join, isAbsolute } from 'node:path'
import { readFile, readdir, stat } from 'node:fs/promises'
import type { AgentConfig } from './agent-config'

export interface ToolResult {
  stdout: string
  stderr: string
  exitCode: number
  meta?: Record<string, unknown>
}

// ── Core executor (no shell) ──────────────────────────────────────────────────

function resolveToolBinary(binary: string, toolPath: string): string {
  const found = toolPath.split(':').reduce<string | null>((acc, dir) => {
    if (acc) return acc
    const candidate = join(dir, binary)
    try { accessSync(candidate, constants.X_OK); return candidate }
    catch { return null }
  }, null)
  return found ?? binary
}

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
      shell: false,   // CRITICAL: no shell injection
    })

    const stdout: string[] = []
    const stderr: string[] = []

    const timer = setTimeout(() => {
      child.kill('SIGTERM')
      resolve({
        stdout: stdout.join(''),
        stderr: stderr.join('') + '\n[TIMEOUT: command exceeded limit]',
        exitCode: 124,
      })
    }, opts.timeout)

    child.stdout?.on('data', (c: Buffer) => stdout.push(c.toString()))
    child.stderr?.on('data', (c: Buffer) => stderr.push(c.toString()))
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
```

---

## ToolDefinition Interface + discoverTools()

```typescript
// src/relay/agent-tool-registry.ts (continued)

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
  readonly binary: string | null
  readonly description: string
  readonly inputSchema: ToolInputSchema
  handler(params: Record<string, unknown>, config: AgentConfig): Promise<ToolResult>
}

// Tool discovery — pure function (no side effects)
export async function discoverTools(config: AgentConfig): Promise<ToolDefinition[]> {
  const pathDirs = config.toolPath.split(':')
  const discovered: ToolDefinition[] = []

  for (const tool of ALL_TOOL_DEFINITIONS) {
    if (tool.binary === null) {
      // Built-in: always available (read_file, list_dir)
      discovered.push(tool)
      continue
    }
    const found = pathDirs.some(dir => {
      try { accessSync(join(dir, tool.binary!), constants.X_OK); return true }
      catch { return false }
    })
    if (found) discovered.push(tool)
  }
  return discovered
}
```

---

## All Tool Definitions

```typescript
// src/relay/agent-tool-registry.ts (continued)

const ALL_TOOL_DEFINITIONS: ToolDefinition[] = [

  // ── claude_code ──────────────────────────────────────────────────────────
  {
    name: 'claude_code', binary: 'claude',
    description: 'Run Claude Code AI assistant. Uses --print mode for non-interactive output.',
    inputSchema: {
      type: 'object',
      properties: {
        prompt: { type: 'string', description: 'Task or question for Claude Code' },
        cwd:    { type: 'string', description: 'Working directory (absolute). Defaults to AGENT_WORK_DIR.' },
        model:  { type: 'string', description: 'Claude model override (e.g. claude-opus-4-5)', default: '' },
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

  // ── gh ──────────────────────────────────────────────────────────────────
  {
    name: 'gh', binary: 'gh',
    description: 'GitHub CLI — PRs, issues, repos, gists.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'gh subcommand and arguments' },
        cwd:  { type: 'string', description: 'Working directory' },
      },
      required: ['args'],
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : []
      const cwd  = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      return runToolCommand('gh', args, { cwd, timeout: 60_000, env: config.toolEnv })
    },
  },

  // ── git ─────────────────────────────────────────────────────────────────
  {
    name: 'git', binary: 'git',
    description: 'Run git commands. For UI-driven git operations, prefer the git.exec RPC method (has whitelist validation).',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'git arguments' },
        cwd:  { type: 'string', description: 'Working directory (git repository)' },
      },
      required: ['args'],
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : []
      const cwd  = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      return runToolCommand('git', args, { cwd, timeout: 60_000, env: config.toolEnv })
    },
  },

  // ── gitnexus ────────────────────────────────────────────────────────────
  {
    name: 'gitnexus', binary: 'gitnexus',
    description: 'GitNexus code intelligence CLI. Query codebase graph, find symbols.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'gitnexus arguments' },
        cwd:  { type: 'string', description: 'Working directory (with .codegraph/)' },
      },
      required: ['args'],
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : []
      const cwd  = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
      return runToolCommand('gitnexus', args, { cwd, timeout: 60_000, env: config.toolEnv })
    },
  },

  // ── codegraph ───────────────────────────────────────────────────────────
  {
    name: 'codegraph', binary: 'codegraph',
    description: 'CodeGraph local code analysis. Explore symbols, dependencies.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'codegraph arguments' },
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

  // ── docker ──────────────────────────────────────────────────────────────
  {
    name: 'docker', binary: 'docker',
    description: 'Run docker commands. Inspect containers, images, logs.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'docker arguments' },
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

  // ── shell ───────────────────────────────────────────────────────────────
  {
    name: 'shell', binary: 'bash',
    description: 'Run a shell command on the dev server. Output is captured and returned.',
    inputSchema: {
      type: 'object',
      properties: {
        command: { type: 'string', description: 'Shell command to execute (bash -c)' },
        cwd:     { type: 'string', description: 'Working directory' },
        timeout: { type: 'number', description: 'Timeout in ms (default: 60000, max: 600000)', default: 60000 },
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
    name: 'read_file', binary: null,
    description: 'Read a file from the dev server filesystem.',
    inputSchema: {
      type: 'object',
      properties: {
        path:       { type: 'string', description: 'File path (absolute or relative to AGENT_WORK_DIR)' },
        start_line: { type: 'number', description: 'First line to read (1-indexed)', default: 1 },
        end_line:   { type: 'number', description: 'Last line to read (inclusive)' },
      },
      required: ['path'],
    },
    async handler(params, config) {
      const filePath = isAbsolute(String(params.path))
        ? String(params.path)
        : join(config.workDir, String(params.path))
      try {
        const content = await readFile(filePath, 'utf8')
        const lines   = content.split('\n')
        const start   = Math.max(0, (typeof params.start_line === 'number' ? params.start_line : 1) - 1)
        const end     = typeof params.end_line === 'number' ? params.end_line : lines.length
        return {
          stdout: lines.slice(start, end).join('\n'),
          stderr: '',
          exitCode: 0,
          meta: { path: filePath, totalLines: lines.length, startLine: start + 1, endLine: end },
        }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return { stdout: '', stderr: msg, exitCode: 1 }
      }
    },
  },

  // ── list_dir (built-in) ──────────────────────────────────────────────────
  {
    name: 'list_dir', binary: null,
    description: 'List directory contents on the dev server.',
    inputSchema: {
      type: 'object',
      properties: {
        path: { type: 'string', description: 'Directory path (absolute or relative to AGENT_WORK_DIR)' },
      },
      required: ['path'],
    },
    async handler(params, config) {
      const dirPath = isAbsolute(String(params.path))
        ? String(params.path)
        : join(config.workDir, String(params.path))
      try {
        const entries = await readdir(dirPath, { withFileTypes: true })
        const list = await Promise.all(entries.map(async (e) => ({
          name: e.name,
          type: e.isDirectory() ? 'dir' : 'file',
          size: e.isFile() ? (await stat(join(dirPath, e.name))).size : null,
        })))
        return { stdout: JSON.stringify(list, null, 2), stderr: '', exitCode: 0 }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return { stdout: '', stderr: msg, exitCode: 1 }
      }
    },
  },
]

export { ALL_TOOL_DEFINITIONS }
```

---

## Definition of Done

- [x] `src/relay/agent-tool-registry.ts` created (single file)
- [x] `tsc` passes
- [x] `discoverTools()` returns correct subset based on installed binaries
- [x] `src/relay/__tests__/agent-tool-registry.test.ts` — ≥ 20 tests pass
- [x] `read_file` handler reads correct line range (test with real temp file)
- [x] `shell` handler caps timeout at 600_000ms (unit test)
