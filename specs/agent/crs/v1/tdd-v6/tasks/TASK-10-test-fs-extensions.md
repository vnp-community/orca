# TASK-10: Write Tests — fs-agent-extensions

> ✅ **STATUS: DONE** — Completed 2026-07-30T18:13
> 📝 **Result:** 42/42 tests pass — Extended existing file: added `handleFsStat` (7), `handleFsGlob` (6), `handleFsWriteFile` (9) test suites.
**Phase:** 5
**File:** `src/relay/__tests__/fs-agent-extensions.test.ts` (NEW FILE)
**Operation:** CREATE
**Depends on:** TASK-04 phải hoàn thành

---

## Test File

Tạo `src/relay/__tests__/fs-agent-extensions.test.ts`:

```typescript
// src/relay/__tests__/fs-agent-extensions.test.ts

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync, writeFileSync, mkdirSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

import {
  handleFsReadDir,
  handleFsStat,
  handleFsGlob,
  handleFsWriteFile,
} from '../fs-agent-extensions'
import type { AgentConfig } from '../agent-config'

// ─── Fixtures ─────────────────────────────────────────────────────────────────

let tmpWorkDir: string
let config: AgentConfig

beforeEach(() => {
  tmpWorkDir = mkdtempSync(join(tmpdir(), 'orca-fs-test-'))
  config = {
    devServerId:   'test',
    agentToken:    '',
    workDir:       tmpWorkDir,
    credentialDir: tmpWorkDir,
    toolPath:      '/usr/bin:/bin',
    toolEnv:       {},
  } as unknown as AgentConfig

  // Create test files
  writeFileSync(join(tmpWorkDir, 'hello.ts'), 'export const x = 1')
  writeFileSync(join(tmpWorkDir, 'README.md'), '# Test')
  mkdirSync(join(tmpWorkDir, 'src'))
  writeFileSync(join(tmpWorkDir, 'src', 'index.ts'), 'export default {}')
})

afterEach(() => {
  rmSync(tmpWorkDir, { recursive: true, force: true })
})

// ─── handleFsReadDir ──────────────────────────────────────────────────────────

describe('handleFsReadDir', () => {
  it('returns entries for valid directory', async () => {
    const res = await handleFsReadDir(1, { path: tmpWorkDir, depth: 1 }, config) as {
      result?: { entries: unknown[]; path: string }
    }
    expect(res.result?.entries).toBeDefined()
    expect(Array.isArray(res.result?.entries)).toBe(true)
    expect(res.result?.entries.length).toBeGreaterThan(0)
  })

  it('returns InvalidParams for missing path', async () => {
    const res = await handleFsReadDir(1, {}, config) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns error for non-existent directory', async () => {
    const res = await handleFsReadDir(1, { path: '/nonexistent-abc-xyz' }, config) as { error?: unknown }
    expect(res.error).toBeDefined()
  })

  it('returns error when path is a file not directory', async () => {
    const res = await handleFsReadDir(1, {
      path: join(tmpWorkDir, 'hello.ts')
    }, config) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

// ─── handleFsStat ─────────────────────────────────────────────────────────────

describe('handleFsStat', () => {
  it('returns stat for existing file', async () => {
    const res = await handleFsStat(1, {
      path: join(tmpWorkDir, 'hello.ts')
    }, config) as {
      result?: { path: string; size: number; isFile: boolean; isDir: boolean; mtime: string; mode: string }
    }
    expect(res.result?.isFile).toBe(true)
    expect(res.result?.isDir).toBe(false)
    expect(res.result?.size).toBeGreaterThan(0)
    expect(res.result?.mtime).toMatch(/^\d{4}-\d{2}-\d{2}/)
    expect(res.result?.mode).toBeTruthy()
  })

  it('returns stat for directory', async () => {
    const res = await handleFsStat(1, {
      path: join(tmpWorkDir, 'src')
    }, config) as { result?: { isDir: boolean; isFile: boolean } }
    expect(res.result?.isDir).toBe(true)
    expect(res.result?.isFile).toBe(false)
  })

  it('returns error for non-existent path', async () => {
    const res = await handleFsStat(1, {
      path: join(tmpWorkDir, 'does-not-exist.txt')
    }, config) as { error?: unknown }
    expect(res.error).toBeDefined()
  })

  it('returns InvalidParams for missing path', async () => {
    const res = await handleFsStat(1, {}, config) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('resolves relative path against workDir', async () => {
    const res = await handleFsStat(1, { path: 'hello.ts' }, config) as {
      result?: { isFile: boolean }
    }
    expect(res.result?.isFile).toBe(true)
  })
})

// ─── handleFsGlob ─────────────────────────────────────────────────────────────

describe('handleFsGlob', () => {
  it('finds .ts files', async () => {
    const res = await handleFsGlob(1, {
      pattern: '*.ts',
      cwd:     tmpWorkDir,
    }, config) as { result?: { paths: string[]; total: number } }
    expect(res.result?.paths.length).toBeGreaterThan(0)
    expect(res.result?.paths.every(p => p.endsWith('.ts'))).toBe(true)
  })

  it('finds files in subdirectories', async () => {
    const res = await handleFsGlob(1, {
      pattern: '*.ts',
      cwd:     tmpWorkDir,
    }, config) as { result?: { paths: string[] } }
    // Should find hello.ts and src/index.ts
    expect(res.result?.paths.length).toBeGreaterThanOrEqual(2)
  })

  it('returns InvalidParams for missing pattern', async () => {
    const res = await handleFsGlob(1, { cwd: tmpWorkDir }, config) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns empty array for non-matching pattern', async () => {
    const res = await handleFsGlob(1, {
      pattern: '*.xyz-nonexistent',
      cwd:     tmpWorkDir,
    }, config) as { result?: { paths: string[]; total: number } }
    expect(res.result?.paths).toEqual([])
    expect(res.result?.total).toBe(0)
  })

  it('returns total count in result', async () => {
    const res = await handleFsGlob(1, {
      pattern: '*.ts',
      cwd:     tmpWorkDir,
    }, config) as { result?: { total: number; paths: string[] } }
    expect(res.result?.total).toBe(res.result?.paths.length)
  })
})

// ─── handleFsWriteFile ────────────────────────────────────────────────────────

describe('handleFsWriteFile', () => {
  it('writes file to workDir', async () => {
    const res = await handleFsWriteFile(1, {
      path:    join(tmpWorkDir, 'new-file.txt'),
      content: 'hello world',
    }, config) as { result?: { ok: boolean; path: string; bytes: number } }
    expect(res.result?.ok).toBe(true)
    expect(res.result?.bytes).toBe(11)  // 'hello world'.length
  })

  it('creates parent directories automatically', async () => {
    const res = await handleFsWriteFile(1, {
      path:    join(tmpWorkDir, 'deep', 'nested', 'file.ts'),
      content: 'const x = 1',
    }, config) as { result?: { ok: boolean } }
    expect(res.result?.ok).toBe(true)
  })

  it('rejects path outside workDir (path traversal)', async () => {
    const res = await handleFsWriteFile(1, {
      path:    '/etc/malicious-file',
      content: 'hack',
    }, config) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('rejects path traversal via ../', async () => {
    const res = await handleFsWriteFile(1, {
      path:    join(tmpWorkDir, '..', '..', 'etc', 'passwd'),
      content: 'root::0:0',
    }, config) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for missing path', async () => {
    const res = await handleFsWriteFile(1, { content: 'data' }, config) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('overwrites existing file', async () => {
    const filePath = join(tmpWorkDir, 'hello.ts')
    const res = await handleFsWriteFile(1, {
      path: filePath, content: 'updated content',
    }, config) as { result?: { ok: boolean } }
    expect(res.result?.ok).toBe(true)
  })
})
```

---

## Verify

```bash
pnpm test src/relay/__tests__/fs-agent-extensions.test.ts
# Expected: ≥ 20 tests pass
```

---

## Done criteria

- [ ] `handleFsReadDir` — 4 tests
- [ ] `handleFsStat` — 5 tests (file, dir, ENOENT, missing params, relative path)
- [ ] `handleFsGlob` — 5 tests (find, subdirs, missing pattern, no match, total count)
- [ ] `handleFsWriteFile` — 6 tests (write, mkdir, path traversal x2, missing params, overwrite)
- [ ] Tất cả ≥ 20 tests pass
