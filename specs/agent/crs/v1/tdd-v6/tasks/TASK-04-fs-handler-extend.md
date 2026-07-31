# TASK-04: Extend FS Handler — fs.stat + fs.glob + fs.writeFile

> ✅ **STATUS: DONE** — Completed 2026-07-30T17:44

**Phase:** 2
**File:** `src/relay/fs-agent-extensions.ts`
**Operation:** EXTEND (append to existing file)
**CR:** [CR-AG-11](../solutions/CR-AG-11-fs-handler.md)
**TDD:** TDD-AG-11
**Depends on:** Không có dependency (standalone)
**Blocked by:** Không

---

## Mục tiêu

Thêm vào `fs-agent-extensions.ts` (282 lines hiện tại):
1. **`handleFsStat()`** — stat file/dir (size, mtime, isDir, isFile, mode)
2. **`handleFsGlob()`** — glob pattern matching (via `find` CLI, no shell)
3. **`handleFsWriteFile()`** — write file với SecureFs path validation

---

## Context đọc trước

```
src/relay/fs-agent-extensions.ts  (282 lines)
  - Line 6: imports từ 'node:fs/promises'  ← cần thêm stat
  - Line 7: import { join, isAbsolute }   ← cần thêm resolve
  - Line 8: import { spawn }             ← đã có (dùng cho fs.glob)
  - Line 282: EOF — APPEND vào đây
```

**Import block hiện tại (lines 6-12):**
```typescript
import { readdir, stat } from 'node:fs/promises'
import { join, isAbsolute } from 'node:path'
import { spawn } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { readRelayFileContent } from './fs-handler-file-read'
import { checkRgAvailable } from './fs-handler-utils'
```

**AgentErrorCode values** (từ `src/shared/agent-wire-protocol.ts`):
- `-32602` = `InvalidParams`
- `-32001` = `PathNotFound` (hoặc `ServerError` — check file)
- `-32003` = `PermissionDenied` (hoặc confirm từ protocol file)

---

## Thay đổi cần thực hiện

### Edit 1 — Update import (line 7)

```diff
-import { join, isAbsolute } from 'node:path'
+import { join, isAbsolute, resolve as resolvePath, dirname } from 'node:path'
```

### Edit 2 — Thêm `writeFile` vào fs/promises import (line 6)

```diff
-import { readdir, stat } from 'node:fs/promises'
+import { readdir, stat, writeFile, mkdir } from 'node:fs/promises'
```

### Edit 3 — APPEND toàn bộ đoạn sau vào **cuối file** (sau line 282)

```typescript
// ─── fs.stat ──────────────────────────────────────────────────────────────────

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
        path:   absPath,
        size:   st.size,
        mtime:  st.mtime.toISOString(),
        isDir:  st.isDirectory(),
        isFile: st.isFile(),
        isLink: st.isSymbolicLink(),
        mode:   st.mode.toString(8),
      },
    }
  } catch (err: unknown) {
    const nodeErr = err as NodeJS.ErrnoException
    if (nodeErr.code === 'ENOENT') {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `Not found: ${absPath}` } }
    }
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── fs.glob ──────────────────────────────────────────────────────────────────

/**
 * Glob files on dev server using `find` CLI (always available on Linux/macOS).
 * shell: false — no shell injection possible.
 * MAX_RESULTS: 200 entries to prevent memory explosion.
 */
export async function handleFsGlob(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const pattern = typeof params.pattern === 'string' ? params.pattern : ''
  const cwd     = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  const ignore  = Array.isArray(params.ignore)
    ? (params.ignore as unknown[]).map(String)
    : ['node_modules', '.git', 'dist', 'out']
  const MAX_RESULTS = 200

  if (!pattern) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: pattern' } }
  }

  // Extract filename pattern (last segment of pattern)
  const filePattern = pattern.split('/').pop() ?? pattern

  const ignoreArgs = ignore.flatMap(p => [
    '-not', '-path', `*/${p}/*`,
    '-not', '-name', p,
  ])

  const results = await new Promise<string[]>((resolve, reject) => {
    const child = spawn('find', [
      cwd,
      '-maxdepth', '10',
      ...ignoreArgs,
      '-name', filePattern,
      '-type', 'f',
    ], { shell: false })

    const lines: string[] = []

    child.stdout?.on('data', (chunk: Buffer) => {
      const newLines = chunk.toString().split('\n').filter(l => l.trim())
      lines.push(...newLines)
      if (lines.length > MAX_RESULTS) child.kill('SIGTERM')
    })

    child.on('close', () => resolve(lines.slice(0, MAX_RESULTS)))
    child.on('error', reject)
  })

  const relativePaths = results.map(p =>
    p.startsWith(cwd + '/') ? p.slice(cwd.length + 1) : p
  )

  return {
    jsonrpc: '2.0', id,
    result: { paths: relativePaths, cwd, total: relativePaths.length },
  }
}

// ─── fs.writeFile ─────────────────────────────────────────────────────────────

/**
 * Write file on dev server.
 * SECURITY CRITICAL: path must be within config.workDir (projectRoot).
 * Max write: 10MB. Creates parent directories automatically.
 */
export async function handleFsWriteFile(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath  = typeof params.path     === 'string' ? params.path    : ''
  const content  = typeof params.content  === 'string' ? params.content : ''
  const encoding = typeof params.encoding === 'string' ? params.encoding as BufferEncoding : 'utf-8'

  if (!rawPath) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }

  const absPath     = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)
  const resolvedPath = resolvePath(absPath)
  const resolvedWork = resolvePath(config.workDir)

  // SecureFs: must be within workDir
  if (!resolvedPath.startsWith(resolvedWork + '/') && resolvedPath !== resolvedWork) {
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: `Path outside project root: ${rawPath}` },
    }
  }

  // Size limit: 10MB
  const MAX_WRITE = 10 * 1024 * 1024
  if (Buffer.byteLength(content, encoding) > MAX_WRITE) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Content too large: max 10MB' } }
  }

  try {
    await mkdir(dirname(resolvedPath), { recursive: true })
    await writeFile(resolvedPath, content, { encoding })
    return {
      jsonrpc: '2.0', id,
      result: { ok: true, path: resolvedPath, bytes: Buffer.byteLength(content, encoding) },
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

---

## Verify

```bash
# TypeScript compile
npx tsc --noEmit -p config/tsconfig.node.json

# Check exports
grep -n "^export async function" src/relay/fs-agent-extensions.ts
# Expected:
# handleFsReadDir   (existing)
# handleFsReadFile  (existing)
# handleFsGrep      (existing)
# handlePreflightCheck (existing)
# handleFsStat      ← NEW
# handleFsGlob      ← NEW
# handleFsWriteFile ← NEW
```

---

## Done criteria

- [ ] `writeFile`, `mkdir` thêm vào import fs/promises
- [ ] `resolve as resolvePath`, `dirname` thêm vào import path
- [ ] `handleFsStat()` — trả về size, mtime, isDir, isFile, isLink, mode
- [ ] `handleFsGlob()` — dùng `find` CLI (shell: false), max 200 kết quả
- [ ] `handleFsWriteFile()` — SecureFs validation (path phải trong workDir), max 10MB
- [ ] TypeScript compile không lỗi
