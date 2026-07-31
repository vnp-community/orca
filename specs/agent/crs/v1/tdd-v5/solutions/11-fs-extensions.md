# SOL-11: fs-agent-extensions.ts — FS RPC Handlers (v5.0)

**TDD Ref:** TDD-AG-11  
**File:** `src/relay/fs-agent-extensions.ts` [NEW] (wraps existing fs-handler-*.ts)  
**Mức độ:** 🟡 Trung bình  
**Thời gian ước tính:** 2h

---

## Chiến lược: Wrap existing fs-handler-*.ts

Codebase đã có:
- `src/relay/fs-handler-file-read.ts` → `readRelayFileContent()` (kiểm tra size limit, binary detect)
- `src/relay/fs-handler-list-files.ts` → `listRelayFiles()`
- `src/relay/fs-handler-utils.ts` → `checkRgAvailable()`, constants
- `src/relay/fs-handler-rg-availability.ts`

**KHÔNG viết lại** — tạo `fs-agent-extensions.ts` chỉ là RPC handler wrappers.

---

## Full Implementation

```typescript
// src/relay/fs-agent-extensions.ts

import { readdir, stat } from 'node:fs/promises'
import { join, isAbsolute } from 'node:path'
import { spawn } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { readRelayFileContent } from './fs-handler-file-read'   // REUSE existing
import { checkRgAvailable } from './fs-handler-utils'           // REUSE existing

// ─── fs.readDir ───────────────────────────────────────────────────────────────

interface FileTreeNode {
  path: string
  name: string
  type: 'file' | 'directory'
  size?: number
  children?: FileTreeNode[]
}

export async function handleFsReadDir(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  const depth   = typeof params.depth === 'number' ? Math.min(params.depth, 5) : 1  // max depth=5

  if (!rawPath) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing path' } }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    const st = await stat(absPath)
    if (!st.isDirectory()) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: `Not a directory: ${absPath}` } }
    }

    const entries = await readDirRecursive(absPath, depth, 1)
    return { jsonrpc: '2.0', id, result: { entries, path: absPath } }

  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

async function readDirRecursive(
  dir: string,
  maxDepth: number,
  currentDepth: number
): Promise<FileTreeNode[]> {
  const entries = await readdir(dir, { withFileTypes: true })
  const nodes = await Promise.all(
    entries.map(async (entry): Promise<FileTreeNode> => {
      const fullPath = join(dir, entry.name)
      const node: FileTreeNode = {
        path: fullPath,
        name: entry.name,
        type: entry.isDirectory() ? 'directory' : 'file',
        size: entry.isFile() ? (await stat(fullPath)).size : undefined,
      }
      if (entry.isDirectory() && currentDepth < maxDepth) {
        node.children = await readDirRecursive(fullPath, maxDepth, currentDepth + 1)
      }
      return node
    })
  )
  // Sort: directories first, then files; alphabetically within each group
  return nodes.sort((a, b) => {
    if (a.type !== b.type) return a.type === 'directory' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
}

// ─── fs.readFile ─────────────────────────────────────────────────────────────

export async function handleFsReadFile(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  if (!rawPath) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing path' } }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    // Reuse existing readRelayFileContent() — handles size limits, binary detection
    const result = await readRelayFileContent(absPath)
    return {
      jsonrpc: '2.0', id,
      result: {
        content:  result.content,
        encoding: result.isBinary ? 'base64' : 'utf-8',
        isBinary: result.isBinary,
        path:     absPath,
      },
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    if (msg.includes('File too large')) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'FILE_TOO_LARGE' } }
    }
    if (msg.includes('ENOENT') || msg.includes('not found')) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `File not found: ${absPath}` } }
    }
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── fs.grep ─────────────────────────────────────────────────────────────────

interface GrepMatch {
  file: string
  line: number
  text: string
}

export async function handleFsGrep(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const root       = typeof params.root === 'string' && params.root ? params.root : config.workDir
  const pattern    = typeof params.pattern === 'string' ? params.pattern : ''
  const maxResults = typeof params.maxResults === 'number' ? Math.min(params.maxResults, 200) : 50

  if (!pattern) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing pattern' } }
  }

  const absRoot = isAbsolute(root) ? root : join(config.workDir, root)
  const rgAvailable = await checkRgAvailable()

  try {
    const matches = rgAvailable
      ? await grepWithRg(pattern, absRoot, maxResults, config)
      : await grepFallback(pattern, absRoot, maxResults, config)

    return {
      jsonrpc: '2.0', id,
      result: { matches, total: matches.length, truncated: matches.length >= maxResults },
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

function grepWithRg(
  pattern: string,
  root: string,
  maxResults: number,
  config: AgentConfig
): Promise<GrepMatch[]> {
  return new Promise((resolve, reject) => {
    const child = spawn('rg', ['--json', '--ignore-case', '--max-count', String(maxResults), pattern, root], {
      env: config.toolEnv, stdio: ['pipe', 'pipe', 'pipe'], shell: false
    })
    const lines: string[] = []
    child.stdout?.on('data', (c: Buffer) => lines.push(c.toString()))
    child.on('close', (code) => {
      if (code !== 0 && code !== 1) { reject(new Error(`rg exited with code ${code}`)); return }
      const matches: GrepMatch[] = []
      for (const line of lines.join('').split('\n')) {
        if (!line.trim()) continue
        try {
          const obj = JSON.parse(line) as { type: string; data: { path: { text: string }; line_number: number; lines: { text: string } } }
          if (obj.type === 'match') {
            matches.push({ file: obj.data.path.text, line: obj.data.line_number, text: obj.data.lines.text.trimEnd() })
            if (matches.length >= maxResults) break
          }
        } catch { /* skip non-JSON */ }
      }
      resolve(matches)
    })
    child.stdin?.end()
  })
}

function grepFallback(
  pattern: string,
  root: string,
  maxResults: number,
  config: AgentConfig
): Promise<GrepMatch[]> {
  return new Promise((resolve, reject) => {
    const child = spawn('grep', ['-r', '-n', '-i', '--include=*.ts', '--include=*.js', '--include=*.go', pattern, root], {
      env: config.toolEnv, stdio: ['pipe', 'pipe', 'pipe'], shell: false
    })
    let output = ''
    child.stdout?.on('data', (c: Buffer) => { output += c.toString() })
    child.on('close', (code) => {
      if (code !== 0 && code !== 1) { reject(new Error(`grep exited with code ${code}`)); return }
      const matches: GrepMatch[] = []
      for (const line of output.split('\n')) {
        const m = line.match(/^(.+?):(\d+):(.*)$/)
        if (m) {
          matches.push({ file: m[1], line: parseInt(m[2], 10), text: m[3] })
          if (matches.length >= maxResults) break
        }
      }
      resolve(matches)
    })
    child.stdin?.end()
  })
}

// ─── preflight.check ─────────────────────────────────────────────────────────

export async function handlePreflightCheck(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const services = Array.isArray(params.services) ? params.services.map(String) : []
  const results: Record<string, boolean> = {}

  await Promise.all(services.map(async (service) => {
    try {
      switch (service) {
        case 'github-cli':
          results[service] = await checkBinaryAvailable('gh', config)
          break
        case 'ripgrep':
          results[service] = await checkRgAvailable()
          break
        case 'docker':
          results[service] = await checkBinaryAvailable('docker', config)
          break
        case 'claude':
          results[service] = await checkBinaryAvailable('claude', config)
          break
        default:
          results[service] = false
      }
    } catch {
      results[service] = false
    }
  }))

  return { jsonrpc: '2.0', id, result: results }
}

async function checkBinaryAvailable(binary: string, config: AgentConfig): Promise<boolean> {
  return new Promise((resolve) => {
    const child = spawn(binary, ['--version'], {
      env: config.toolEnv, stdio: ['pipe', 'pipe', 'pipe'], shell: false
    })
    const timer = setTimeout(() => { child.kill(); resolve(false) }, 5_000)
    child.on('close', (code) => { clearTimeout(timer); resolve(code === 0) })
    child.on('error', () => { clearTimeout(timer); resolve(false) })
    child.stdin?.end()
  })
}
```

---

## Definition of Done

- [x] `src/relay/fs-agent-extensions.ts` created
- [x] `readRelayFileContent` import resolves from existing `fs-handler-file-read.ts`
- [x] `checkRgAvailable` import resolves from existing `fs-handler-utils.ts`
- [x] `tsc` passes
- [x] `fs.readDir` tests: depth-aware, sorted, dirs-first (≥ 8 tests)
- [x] `fs.readFile` tests: delegates to `readRelayFileContent`, maps errors correctly (≥ 5 tests)
- [x] `fs.grep` tests: rg path + fallback path (mock spawn) (≥ 8 tests)
- [x] `preflight.check` tests: known + unknown services (≥ 5 tests)
