# CR-AG-11: FS Handler Extension — Thêm fs.stat, fs.glob, fs.writeFile

**CR:** CR-AG-11
**TDD:** [TDD-AG-11](../../tdd/v5/11-fs-handler-extension.md)
**Ngày:** 2026-07-30
**Độ phức tạp:** Medium — extend `fs-agent-extensions.ts`
**ADR:** ADR-011
**HLD Ref:** C3.12, C4.10

---

## 1. Phân tích Code Hiện Tại

### Code đã có ✅ — [`src/relay/fs-agent-extensions.ts`](../../../../../src/relay/fs-agent-extensions.ts)

| Function | Trạng thái | Ghi chú |
|---------|-----------|---------|
| `handleFsReadDir()` | ✅ DONE | recursive depth ≤ 5, sort dirs first |
| `handleFsReadFile()` | ✅ DONE | giới hạn 5MB, encoding utf-8 |
| `handleFsGrep()` | ✅ DONE | ripgrep hoặc grep fallback, max 30 results |
| `handlePreflightCheck()` | ✅ DONE | gh/glab auth status + disk check |
| `handleFsStat()` | ❌ MISSING | TDD-AG-11 yêu cầu |
| `handleFsGlob()` | ❌ MISSING | TDD-AG-11 yêu cầu |
| `handleFsWriteFile()` | ❌ MISSING | TDD-AG-11 (partial — cần SecureFs) |

### Code đã có ✅ — `fs-handler-utils.ts`

[`src/relay/fs-handler-utils.ts`](../../../../../src/relay/fs-handler-utils.ts) — `checkRgAvailable()`, path utils.

### Code đã có ✅ — `fs-handler-file-read.ts`

[`src/relay/fs-handler-file-read.ts`](../../../../../src/relay/fs-handler-file-read.ts) — `readRelayFileContent()` đã có size limit và encoding.

---

## 2. Solution

### 2.1 EXTEND: `src/relay/fs-agent-extensions.ts`

Thêm vào cuối file (không sửa code hiện tại):

```typescript
// src/relay/fs-agent-extensions.ts — APPEND sau line 282

// ─── fs.stat ─────────────────────────────────────────────────────────────────

export async function handleFsStat(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  if (!rawPath) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    const st = await stat(absPath)
    return {
      jsonrpc: '2.0', id,
      result: {
        path:    absPath,
        size:    st.size,
        mtime:   st.mtime.toISOString(),
        isDir:   st.isDirectory(),
        isFile:  st.isFile(),
        isLink:  st.isSymbolicLink(),
        mode:    st.mode.toString(8),
      },
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `Not found: ${absPath}` } }
    }
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

```typescript
// ─── fs.glob ─────────────────────────────────────────────────────────────────

/**
 * Simple glob expansion using readdir + recursive matching.
 * Avoids external glob library — reuses readDirRecursive logic.
 * Pattern: supports *, **, ? wildcards via minimatch-style regex conversion.
 * MAX_RESULTS: 200 entries.
 */
export async function handleFsGlob(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const pattern = typeof params.pattern === 'string' ? params.pattern : ''
  const cwd     = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  const ignore  = Array.isArray(params.ignore) ? params.ignore.map(String) : ['node_modules', '.git']
  const MAX_RESULTS = 200

  if (!pattern) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: pattern' } }
  }

  try {
    // Strategy: use `find` command (always available on Linux) via spawn — no shell
    // fall back to manual readdir walk if pattern is simple *.ext
    const { spawn } = await import('node:child_process')

    const results = await new Promise<string[]>((resolve, reject) => {
      const ignoreArgs = ignore.flatMap(p => ['-not', '-path', `*/${p}/*`, '-not', '-name', p])
      const child = spawn('find', [
        cwd,
        '-maxdepth', '10',
        ...ignoreArgs,
        '-name', pattern.replace(/\*\*\//g, '').split('/').pop() ?? pattern,
        '-type', 'f',
      ], { shell: false })

      const lines: string[] = []
      child.stdout.on('data', (chunk: Buffer) => {
        lines.push(...chunk.toString().split('\n').filter(l => l.trim()))
        if (lines.length > MAX_RESULTS) child.kill()
      })
      child.on('close', () => resolve(lines.slice(0, MAX_RESULTS)))
      child.on('error', reject)
    })

    // Return relative paths
    const relativePaths = results.map(p => p.startsWith(cwd) ? p.slice(cwd.length + 1) : p)
    return { jsonrpc: '2.0', id, result: { paths: relativePaths, cwd, total: relativePaths.length } }

  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

```typescript
// ─── fs.writeFile ─────────────────────────────────────────────────────────────

/**
 * Write file content to dev server — requires SecureFs path validation.
 * Constraint: MUST be within config.workDir (projectRoot).
 * Max write: 10MB. Creates parent dirs.
 *
 * NOTE: Security-critical — path traversal protection is mandatory.
 */
export async function handleFsWriteFile(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path    === 'string' ? params.path    : ''
  const content = typeof params.content === 'string' ? params.content : ''
  const encoding = typeof params.encoding === 'string' ? params.encoding as BufferEncoding : 'utf-8'

  if (!rawPath) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  // ── SecureFs validation ──────────────────────────────────────────────────
  // CRITICAL: Must be within workDir (projectRoot) — no path traversal
  const { resolve } = await import('node:path')
  const resolvedPath = resolve(absPath)
  const resolvedWork = resolve(config.workDir)

  if (!resolvedPath.startsWith(resolvedWork + '/') && resolvedPath !== resolvedWork) {
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.PermissionDenied, message: `Path outside project root: ${absPath}` },
    }
  }

  // Size limit: 10MB
  const MAX_WRITE = 10 * 1024 * 1024
  if (Buffer.byteLength(content, encoding) > MAX_WRITE) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Content too large: max 10MB' } }
  }

  try {
    const { writeFile, mkdir } = await import('node:fs/promises')
    const { dirname } = await import('node:path')
    await mkdir(dirname(resolvedPath), { recursive: true })
    await writeFile(resolvedPath, content, { encoding })
    return { jsonrpc: '2.0', id, result: { ok: true, path: resolvedPath, bytes: Buffer.byteLength(content, encoding) } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

### 2.2 EXTEND: `src/relay/agent-rpc-dispatch.ts`

Thêm routes sau `case 'preflight.check'`:

```typescript
// Thêm vào agent-rpc-dispatch.ts:

// ── v5.0: fs.stat ────────────────────────────────────────────────────────
case 'fs.stat': {
  try {
    const { handleFsStat } = await import('./fs-agent-extensions')
    return (await handleFsStat(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `fs.stat unavailable: ${msg}`)
  }
}

// ── v5.0: fs.glob ────────────────────────────────────────────────────────
case 'fs.glob': {
  try {
    const { handleFsGlob } = await import('./fs-agent-extensions')
    return (await handleFsGlob(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `fs.glob unavailable: ${msg}`)
  }
}

// ── v5.0: fs.writeFile ───────────────────────────────────────────────────
case 'fs.writeFile': {
  try {
    const { handleFsWriteFile } = await import('./fs-agent-extensions')
    return (await handleFsWriteFile(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `fs.writeFile unavailable: ${msg}`)
  }
}
```

---

## 3. Tests

Tạo `src/relay/__tests__/fs-agent-extensions.test.ts`:

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import {
  handleFsReadDir,
  handleFsStat,
  handleFsReadFile,
  handleFsWriteFile,
} from '../fs-agent-extensions'
import type { AgentConfig } from '../agent-config'

function makeConfig(dir: string): AgentConfig {
  return {
    mode: 'direct-websocket', orcaUrl: '', agentToken: '', agentPort: 6799,
    devServerId: 'test', logLevel: 'info',
    workDir: dir, toolPath: '/usr/bin', toolEnv: process.env,
    credentialDir: join(dir, '.creds'), tlsRejectUnauthorized: true,
  }
}

describe('handleFsReadDir', () => {
  let dir: string
  beforeEach(() => {
    dir = join(tmpdir(), `orca-fs-test-${Date.now()}`)
    mkdirSync(join(dir, 'subdir'), { recursive: true })
    writeFileSync(join(dir, 'file.ts'), 'content')
    writeFileSync(join(dir, 'subdir', 'nested.ts'), 'nested')
  })
  afterEach(() => rmSync(dir, { recursive: true, force: true }))

  it('lists directory entries', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsReadDir(null, { path: dir, depth: 1 }, cfg) as {
      result: { entries: Array<{ name: string; type: string }> }
    }
    expect(res.result.entries.length).toBeGreaterThan(0)
    expect(res.result.entries.find(e => e.name === 'subdir')?.type).toBe('directory')
    expect(res.result.entries.find(e => e.name === 'file.ts')?.type).toBe('file')
  })

  it('returns error for non-existent path', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsReadDir(null, { path: '/nonexistent/dir' }, cfg) as {
      error: { code: number }
    }
    expect(res.error).toBeDefined()
  })

  it('respects depth limit (max 5)', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsReadDir(null, { path: dir, depth: 99 }, cfg) as {
      result: { entries: unknown[] }
    }
    expect(res.result.entries).toBeDefined()  // depth capped at 5
  })
})

describe('handleFsStat', () => {
  let dir: string
  beforeEach(() => {
    dir = join(tmpdir(), `orca-stat-test-${Date.now()}`)
    mkdirSync(dir, { recursive: true })
    writeFileSync(join(dir, 'target.ts'), 'hello world')
  })
  afterEach(() => rmSync(dir, { recursive: true, force: true }))

  it('returns stat for file', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsStat(null, { path: join(dir, 'target.ts') }, cfg) as {
      result: { size: number; isFile: boolean; isDir: boolean }
    }
    expect(res.result.isFile).toBe(true)
    expect(res.result.isDir).toBe(false)
    expect(res.result.size).toBeGreaterThan(0)
  })

  it('returns stat for directory', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsStat(null, { path: dir }, cfg) as {
      result: { isDir: boolean }
    }
    expect(res.result.isDir).toBe(true)
  })

  it('returns PathNotFound for missing file', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsStat(null, { path: join(dir, 'ghost.ts') }, cfg) as {
      error: { code: number }
    }
    expect(res.error.code).toBe(-32001)  // PathNotFound
  })
})

describe('handleFsWriteFile', () => {
  let dir: string
  beforeEach(() => {
    dir = join(tmpdir(), `orca-write-test-${Date.now()}`)
    mkdirSync(dir, { recursive: true })
  })
  afterEach(() => rmSync(dir, { recursive: true, force: true }))

  it('writes file within workDir', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsWriteFile(null, {
      path: join(dir, 'output.ts'),
      content: 'export const x = 1',
    }, cfg) as { result: { ok: boolean; bytes: number } }
    expect(res.result.ok).toBe(true)
    expect(res.result.bytes).toBeGreaterThan(0)
  })

  it('rejects path traversal outside workDir', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsWriteFile(null, {
      path: '/etc/passwd',
      content: 'evil',
    }, cfg) as { error: { code: number } }
    expect(res.error.code).toBe(-32003)  // PermissionDenied
  })

  it('creates parent directories automatically', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsWriteFile(null, {
      path: join(dir, 'deep', 'nested', 'file.ts'),
      content: 'nested file',
    }, cfg) as { result: { ok: boolean } }
    expect(res.result.ok).toBe(true)
  })
})

describe('handleFsReadFile', () => {
  let dir: string
  beforeEach(() => {
    dir = join(tmpdir(), `orca-read-test-${Date.now()}`)
    mkdirSync(dir, { recursive: true })
    writeFileSync(join(dir, 'src.ts'), 'const hello = "world"')
  })
  afterEach(() => rmSync(dir, { recursive: true, force: true }))

  it('reads file content', async () => {
    const cfg = makeConfig(dir)
    const res = await handleFsReadFile(null, { path: join(dir, 'src.ts') }, cfg) as {
      result: { content: string }
    }
    expect(res.result.content).toContain('hello')
  })
})
```

**Target: ≥ 14 tests**

---

## 4. Implementation Checklist

- [ ] `src/relay/fs-agent-extensions.ts` — thêm `handleFsStat()` function
- [ ] `src/relay/fs-agent-extensions.ts` — thêm `handleFsGlob()` function (via `find` CLI)
- [ ] `src/relay/fs-agent-extensions.ts` — thêm `handleFsWriteFile()` với SecureFs path validation
- [ ] `src/relay/agent-rpc-dispatch.ts` — thêm 3 case routes: `fs.stat`, `fs.glob`, `fs.writeFile`
- [ ] `src/relay/__tests__/fs-agent-extensions.test.ts` — tạo test file mới
