# TDD-AG-05: Tool Registry & Discovery (v2.1 — TypeScript)

**Document:** TDD-AG-05
**Version:** 2.1
**Date:** 2026-07-28
**Source files:**
- `src/relay/agent-tool-registry.ts` ← [NEW]
**HLD Ref:** C3.8

---

## 1. TypeScript Tool Definition

```typescript
// src/relay/agent-tool-registry.ts

import { accessSync, constants } from 'node:fs'
import { join } from 'node:path'
import type { AgentConfig } from './agent-config'

export interface ToolInputSchema {
  type: 'object'
  properties: Record<string, {
    type: string
    description: string
    default?: unknown
    items?: { type: string }
  }>
  required?: string[]
}

export interface ToolDefinition<TParams = Record<string, unknown>> {
  readonly name: string
  readonly binary: string | null  // null = built-in (no binary check)
  readonly description: string
  readonly inputSchema: ToolInputSchema
  readonly handler: (params: TParams, config: AgentConfig) => Promise<ToolResult>
}

export interface ToolResult {
  stdout: string
  stderr: string
  exitCode: number
  meta?: Record<string, unknown>
  error?: string
}
```

---

## 2. discoverTools() — Typed

```typescript
export async function discoverTools(config: AgentConfig): Promise<ToolDefinition[]> {
  const discovered: ToolDefinition[] = []
  const pathDirs = config.toolPath.split(':')

  for (const tool of ALL_TOOL_DEFINITIONS) {
    if (tool.binary === null) {
      // Built-in tool — always available
      discovered.push(tool)
      continue
    }

    const found = pathDirs.some(dir => {
      const fullPath = join(dir, tool.binary!)
      try { accessSync(fullPath, constants.X_OK); return true }
      catch { return false }
    })

    if (found) {
      discovered.push(tool)
    }
    // Missing tools silently skipped (log handled by caller)
  }

  return discovered
}
```

---

## 3. Tool Definitions (Typed, v2.1)

```typescript
// src/relay/agent-tool-registry.ts

import { runCommandCapture } from './agent-exec-handler'  // REUSE existing!
import { readFile, readdir, stat } from 'node:fs/promises'
import { join, isAbsolute } from 'node:path'

const ALL_TOOL_DEFINITIONS: ToolDefinition[] = [

  // ── claude_code ────────────────────────────────────────────────────
  {
    name: 'claude_code',
    binary: 'claude',
    description: 'Run Claude Code AI assistant (--print mode).',
    inputSchema: {
      type: 'object',
      properties: {
        prompt: { type: 'string', description: 'Task or question for Claude Code' },
        cwd:    { type: 'string', description: 'Working directory (absolute). Defaults to AGENT_WORK_DIR.' },
        model:  { type: 'string', description: 'Claude model (e.g. claude-opus-4-5). Optional.', default: '' },
      },
      required: ['prompt'],
    },
    handler: async (params: { prompt: string; cwd?: string; model?: string }, config) => {
      const cwd  = params.cwd || config.workDir
      const args = ['--print', params.prompt]
      if (params.model) args.unshift('--model', params.model)
      return runCommandCapture('claude', args, { cwd, timeout: 300_000, env: config.toolEnv })
    },
  },

  // ── gh ────────────────────────────────────────────────────────────
  {
    name: 'gh',
    binary: 'gh',
    description: 'GitHub CLI — PRs, issues, repos, gists.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'gh arguments' },
        cwd:  { type: 'string', description: 'Working directory' },
      },
      required: ['args'],
    },
    handler: async (params: { args: string[]; cwd?: string }, config) =>
      runCommandCapture('gh', params.args, { cwd: params.cwd || config.workDir, timeout: 60_000, env: config.toolEnv }),
  },

  // ── git ───────────────────────────────────────────────────────────
  {
    name: 'git',
    binary: 'git',
    description: 'Run git commands.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'git arguments' },
        cwd:  { type: 'string', description: 'Working directory (git repository)' },
      },
      required: ['args'],
    },
    handler: async (params: { args: string[]; cwd?: string }, config) =>
      runCommandCapture('git', params.args, { cwd: params.cwd || config.workDir, timeout: 60_000, env: config.toolEnv }),
  },

  // ── gitnexus ──────────────────────────────────────────────────────
  {
    name: 'gitnexus',
    binary: 'gitnexus',
    description: 'GitNexus code intelligence CLI.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'gitnexus arguments' },
        cwd:  { type: 'string', description: 'Working directory (with .codegraph/)' },
      },
      required: ['args'],
    },
    handler: async (params: { args: string[]; cwd?: string }, config) =>
      runCommandCapture('gitnexus', params.args, { cwd: params.cwd || config.workDir, timeout: 60_000, env: config.toolEnv }),
  },

  // ── docker ───────────────────────────────────────────────────────
  {
    name: 'docker',
    binary: 'docker',
    description: 'Run docker commands.',
    inputSchema: {
      type: 'object',
      properties: {
        args: { type: 'array', items: { type: 'string' }, description: 'docker arguments' },
        cwd:  { type: 'string', description: 'Working directory' },
      },
      required: ['args'],
    },
    handler: async (params: { args: string[]; cwd?: string }, config) =>
      runCommandCapture('docker', params.args, { cwd: params.cwd || config.workDir, timeout: 120_000, env: config.toolEnv }),
  },

  // ── shell ─────────────────────────────────────────────────────────
  {
    name: 'shell',
    binary: 'bash',
    description: 'Run a shell command on the dev server.',
    inputSchema: {
      type: 'object',
      properties: {
        command: { type: 'string', description: 'Shell command (bash -c)' },
        cwd:     { type: 'string', description: 'Working directory' },
        timeout: { type: 'number', description: 'Timeout in ms (default: 60000, max: 600000)', default: 60000 },
      },
      required: ['command'],
    },
    handler: async (params: { command: string; cwd?: string; timeout?: number }, config) => {
      const timeout = Math.min(params.timeout ?? 60_000, 600_000)
      return runCommandCapture('bash', ['-c', params.command], {
        cwd: params.cwd || config.workDir, timeout, env: config.toolEnv
      })
    },
  },

  // ── read_file ─────────────────────────────────────────────────────
  {
    name: 'read_file',
    binary: null,  // built-in
    description: 'Read a file from the dev server filesystem.',
    inputSchema: {
      type: 'object',
      properties: {
        path:       { type: 'string', description: 'File path (absolute or relative to AGENT_WORK_DIR)' },
        start_line: { type: 'number', description: '1-indexed first line', default: 1 },
        end_line:   { type: 'number', description: 'Last line (inclusive)' },
      },
      required: ['path'],
    },
    handler: async (params: { path: string; start_line?: number; end_line?: number }, config) => {
      const filePath = isAbsolute(params.path) ? params.path : join(config.workDir, params.path)
      try {
        const content = await readFile(filePath, 'utf8')
        const lines   = content.split('\n')
        const start   = Math.max(0, (params.start_line ?? 1) - 1)
        const end     = params.end_line ?? lines.length
        return {
          stdout: lines.slice(start, end).join('\n'),
          stderr: '',
          exitCode: 0,
          meta: { path: filePath, totalLines: lines.length, startLine: start + 1, endLine: end },
        }
      } catch (err: any) {
        return { stdout: '', stderr: err.message, exitCode: 1, error: err.message }
      }
    },
  },

  // ── list_dir ──────────────────────────────────────────────────────
  {
    name: 'list_dir',
    binary: null,
    description: 'List directory contents on the dev server.',
    inputSchema: {
      type: 'object',
      properties: {
        path: { type: 'string', description: 'Directory path' },
      },
      required: ['path'],
    },
    handler: async (params: { path: string }, config) => {
      const dirPath = isAbsolute(params.path) ? params.path : join(config.workDir, params.path)
      try {
        const entries = await readdir(dirPath, { withFileTypes: true })
        const list = await Promise.all(entries.map(async (e) => ({
          name: e.name,
          type: e.isDirectory() ? 'dir' : 'file',
          size: e.isFile() ? (await stat(join(dirPath, e.name))).size : null,
        })))
        return { stdout: JSON.stringify(list, null, 2), stderr: '', exitCode: 0 }
      } catch (err: any) {
        return { stdout: '', stderr: err.message, exitCode: 1, error: err.message }
      }
    },
  },
]

export { ALL_TOOL_DEFINITIONS }
```

---

## 5. Key Changes vs v1

| v1 (CommonJS) | v2.1 (TypeScript) |
|--------------|-------------------|
| `TOOL_DEFINITIONS: any[]` | `ToolDefinition<TParams>[]` (generic typed) |
| `handler(params)` | `handler(params, config: AgentConfig)` — config injected |
| `TOOL_ENV` global | `config.toolEnv` passed as dependency |
| `WORK_DIR` global | `config.workDir` passed via config |
| `discoverTools()` mutates global `discoveredTools` | Returns `ToolDefinition[]` (pure function) |
| `runCommandCapture` inline | Import from `./agent-exec-handler` (REUSE existing) |
| `readFileSync` (blocking) | `readFile()` (async/await) |

---

## 6. Test Coverage

```typescript
// src/relay/__tests__/agent-tool-registry.test.ts
import { describe, it, expect, vi } from 'vitest'
import { discoverTools } from '../agent-tool-registry'

describe('discoverTools', () => {
  it('includes built-in tools regardless of PATH', async () => { ... })
  it('excludes tool whose binary not found in toolPath', async () => { ... })
  it('includes tool when binary found in toolPath', async () => { ... })
  it('passes config.toolEnv to handler', async () => { ... })
  it('handler: read_file reads correct line range', async () => { ... })
  it('handler: read_file returns error on missing file', async () => { ... })
  it('handler: list_dir returns entries sorted', async () => { ... })
  it('handler: shell caps timeout at 600000ms', async () => { ... })
})
```

**Target:** ≥ 20 tests
