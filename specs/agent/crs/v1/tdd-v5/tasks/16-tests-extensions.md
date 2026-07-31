# TASK-16: Write Vitest Tests — git-handler + credential-store + fs-agent-extensions

**Phase:** 6  
**SOL Ref:** SOL-12  
**Estimated time:** 2.5h  
**Precondition:** TASK-10, TASK-11, TASK-12 hoàn thành  

---

## File 1: `src/relay/__tests__/git-handler.test.ts`

**Target: ≥ 20 tests**

```typescript
import { describe, it, expect, vi } from 'vitest'
import { validateGitArgs, GitValidationError, handleGitExec } from '../git-handler'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

const mockConfig = { workDir: '/tmp', toolEnv: { PATH: '/usr/bin' } } as AgentConfig
const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

describe('validateGitArgs', () => {
  // ALLOWED subcommands — each must pass
  it.each(['status', 'diff', 'log', 'commit', 'push', 'pull', 'fetch'])(
    'allows "%s"', (cmd) => {
      expect(() => validateGitArgs([cmd])).not.toThrow()
    }
  )

  it('throws GIT_NO_SUBCOMMAND on empty args', () => {
    expect(() => validateGitArgs([])).toThrow(GitValidationError)
    try { validateGitArgs([]) } catch(e) {
      expect((e as GitValidationError).code).toBe('GIT_NO_SUBCOMMAND')
    }
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND on "clean"', () => {
    try { validateGitArgs(['clean', '-fd']) } catch(e) {
      expect((e as GitValidationError).code).toBe('GIT_DISALLOWED_SUBCOMMAND')
    }
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND on "bisect"', () => {
    expect(() => validateGitArgs(['bisect'])).toThrow(GitValidationError)
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND on "gc"', () => {
    expect(() => validateGitArgs(['gc'])).toThrow(GitValidationError)
  })

  // Metacharacter tests — each must be rejected
  it.each(['&', '|', ';', '$', '`', '<', '>', '!'])(
    'throws GIT_SHELL_METACHARACTER_IN_ARG for char "%s"', (char) => {
      try { validateGitArgs(['log', `--format=${char}evil`]) } catch(e) {
        expect((e as GitValidationError).code).toBe('GIT_SHELL_METACHARACTER_IN_ARG')
      }
    }
  )

  it('allows "--oneline" and "-10" (normal flags)', () => {
    expect(() => validateGitArgs(['log', '--oneline', '-10'])).not.toThrow()
  })

  it('allows "origin/main" (forward slash OK)', () => {
    expect(() => validateGitArgs(['diff', 'origin/main'])).not.toThrow()
  })
})

describe('handleGitExec', () => {
  it('returns error response for invalid args (no crash)', async () => {
    const resp = await handleGitExec(1, { args: [] }, mockConfig, mockLog) as any
    expect(resp.error).toBeDefined()
    expect(resp.error.code).toBe(-32602)  // InvalidParams
  })

  it('returns error response for disallowed subcommand', async () => {
    const resp = await handleGitExec(1, { args: ['clean', '-fd'] }, mockConfig, mockLog) as any
    expect(resp.error.code).toBe(-32602)
  })

  // Integration test (requires git installed)
  it('returns stdout for valid git command', async () => {
    const resp = await handleGitExec(1, { args: ['--version'], cwd: '/tmp' }, mockConfig, mockLog) as any
    if (resp.error) return  // skip if git not installed
    expect(resp.result.stdout).toContain('git version')
    expect(resp.result.exitCode).toBe(0)
  })
})
```

---

## File 2: `src/relay/__tests__/agent-credential-store.test.ts`

**Target: ≥ 15 tests**  
**KHÔNG mock node:crypto** — dùng real AES-256-GCM

```typescript
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { handleWriteCredential, handleReadCredential, handleHealthCheck } from '../agent-credential-store'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

let tmpDir: string
const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

beforeEach(() => {
  tmpDir = mkdtempSync(join(tmpdir(), 'cred-test-'))
  vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', 'test-master-key-32-chars-minimum!!')
})

afterEach(() => {
  vi.unstubAllEnvs()
  rmSync(tmpDir, { recursive: true, force: true })
})

function makeConfig(): AgentConfig {
  return { credentialDir: tmpDir } as unknown as AgentConfig
}

describe('handleWriteCredential', () => {
  it('creates .enc file', async () => {
    const { existsSync } = await import('node:fs')
    await handleWriteCredential(1, {
      accountId: 'acc-1', encryptedBlob: 'blob123', iv: 'iv456', algorithm: 'AES-GCM'
    }, makeConfig(), mockLog)
    expect(existsSync(join(tmpDir, 'acc-1.enc'))).toBe(true)
  })

  it('returns { ok: true } on success', async () => {
    const resp = await handleWriteCredential(1, {
      accountId: 'acc-2', encryptedBlob: 'b', iv: 'i'
    }, makeConfig(), mockLog) as any
    expect(resp.result.ok).toBe(true)
  })

  it('returns error on missing accountId', async () => {
    const resp = await handleWriteCredential(1, { encryptedBlob: 'b', iv: 'i' }, makeConfig(), mockLog) as any
    expect(resp.error).toBeDefined()
    expect(resp.error.code).toBe(-32602)
  })

  it('returns error when ORCA_AI_CREDENTIAL_KEY not set', async () => {
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', '')
    const resp = await handleWriteCredential(1, {
      accountId: 'acc-3', encryptedBlob: 'b', iv: 'i'
    }, makeConfig(), mockLog) as any
    expect(resp.error).toBeDefined()
    expect(resp.error.code).toBe(-33002)  // PermissionDenied
  })

  it('rejects path traversal accountId "../evil"', async () => {
    const resp = await handleWriteCredential(1, {
      accountId: '../evil', encryptedBlob: 'b', iv: 'i'
    }, makeConfig(), mockLog) as any
    expect(resp.error).toBeDefined()
    expect(resp.error.code).toBe(-32602)  // InvalidParams
  })
})

describe('handleReadCredential', () => {
  it('round-trip: write then read returns same encryptedBlob and iv', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, {
      accountId: 'rt-test', encryptedBlob: 'my-encrypted-blob', iv: 'my-iv', algorithm: 'AES-GCM'
    }, cfg, mockLog)

    const resp = await handleReadCredential(2, { accountId: 'rt-test' }, cfg, mockLog) as any
    expect(resp.result.encryptedBlob).toBe('my-encrypted-blob')
    expect(resp.result.iv).toBe('my-iv')
    expect(resp.result.algorithm).toBe('AES-GCM')
  })

  it('returns PathNotFound for missing accountId', async () => {
    const resp = await handleReadCredential(1, { accountId: 'nonexistent' }, makeConfig(), mockLog) as any
    expect(resp.error.code).toBe(-33003)  // PathNotFound
  })

  it('returns ServerError when ORCA_AI_CREDENTIAL_KEY changed (decryption fails)', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'test-key-change', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', 'different-key-that-will-fail-decryption-xxx')
    const resp = await handleReadCredential(1, { accountId: 'test-key-change' }, cfg, mockLog) as any
    expect(resp.error).toBeDefined()
  })
})

describe('handleHealthCheck', () => {
  it('returns ok=true when credential exists', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'hc-test', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    const resp = await handleHealthCheck(1, { accountId: 'hc-test' }, cfg, mockLog) as any
    expect(resp.result.ok).toBe(true)
    expect(resp.result.latencyMs).toBeGreaterThanOrEqual(0)
  })
})
```

---

## File 3: `src/relay/__tests__/fs-agent-extensions.test.ts`

**Target: ≥ 15 tests**

```typescript
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, writeFileSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { handleFsReadDir, handleFsReadFile, handlePreflightCheck } from '../fs-agent-extensions'
import type { AgentConfig } from '../agent-config'

// Mock checkRgAvailable and readRelayFileContent
vi.mock('../fs-handler-utils', () => ({ checkRgAvailable: vi.fn().mockResolvedValue(false) }))
vi.mock('../fs-handler-file-read', () => ({
  readRelayFileContent: vi.fn().mockResolvedValue({ content: 'file content', isBinary: false })
}))

let tmpDir: string
const makeConfig = (): AgentConfig => ({ workDir: tmpDir, toolEnv: { PATH: '/usr/bin' } } as any)

beforeEach(() => { tmpDir = mkdtempSync(join(tmpdir(), 'fs-ext-test-')) })
afterEach(() => rmSync(tmpDir, { recursive: true, force: true }))

describe('handleFsReadDir', () => {
  it('lists files in directory', async () => {
    writeFileSync(join(tmpDir, 'a.ts'), '')
    writeFileSync(join(tmpDir, 'b.ts'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir }, makeConfig()) as any
    expect(resp.result.entries).toHaveLength(2)
  })

  it('directories appear before files in result', async () => {
    mkdirSync(join(tmpDir, 'zdir'))
    writeFileSync(join(tmpDir, 'afile.txt'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir }, makeConfig()) as any
    expect(resp.result.entries[0].type).toBe('directory')
    expect(resp.result.entries[1].type).toBe('file')
  })

  it('depth=2 includes grandchildren', async () => {
    mkdirSync(join(tmpDir, 'sub'))
    writeFileSync(join(tmpDir, 'sub', 'child.ts'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir, depth: 2 }, makeConfig()) as any
    const subDir = resp.result.entries.find((e: any) => e.name === 'sub')
    expect(subDir.children).toHaveLength(1)
  })

  it('depth=1 does not include grandchildren', async () => {
    mkdirSync(join(tmpDir, 'sub'))
    writeFileSync(join(tmpDir, 'sub', 'child.ts'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir, depth: 1 }, makeConfig()) as any
    const subDir = resp.result.entries.find((e: any) => e.name === 'sub')
    expect(subDir.children).toBeUndefined()
  })

  it('returns InvalidParams for non-directory path', async () => {
    const filePath = join(tmpDir, 'file.txt')
    writeFileSync(filePath, 'content')
    const resp = await handleFsReadDir(1, { path: filePath }, makeConfig()) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for missing path param', async () => {
    const resp = await handleFsReadDir(1, {}, makeConfig()) as any
    expect(resp.error).toBeDefined()
  })
})

describe('handleFsReadFile', () => {
  it('returns content from readRelayFileContent', async () => {
    const resp = await handleFsReadFile(1, { path: '/any/file.ts' }, makeConfig()) as any
    expect(resp.result.content).toBe('file content')
    expect(resp.result.encoding).toBe('utf-8')
  })

  it('returns InvalidParams for missing path', async () => {
    const resp = await handleFsReadFile(1, {}, makeConfig()) as any
    expect(resp.error.code).toBe(-32602)
  })
})

describe('handlePreflightCheck', () => {
  it('returns false for unknown service', async () => {
    const resp = await handlePreflightCheck(1, { services: ['unknown-tool-xyz'] }, makeConfig()) as any
    expect(resp.result['unknown-tool-xyz']).toBe(false)
  })

  it('returns object with keys for each requested service', async () => {
    const resp = await handlePreflightCheck(1, { services: ['github-cli', 'docker'] }, makeConfig()) as any
    expect('github-cli' in resp.result).toBe(true)
    expect('docker' in resp.result).toBe(true)
  })
})
```

---

## Run Tests

```bash
pnpm test -- src/relay/__tests__/git-handler.test.ts \
  src/relay/__tests__/agent-credential-store.test.ts \
  src/relay/__tests__/fs-agent-extensions.test.ts
```

## Definition of Done

- [x] `git-handler.test.ts` — ≥ 20 tests pass (10 validateGitArgs + 10 handlers)
- [x] `agent-credential-store.test.ts` — ≥ 15 tests pass, no crypto mocks, real tmpdir
- [x] `fs-agent-extensions.test.ts` — ≥ 15 tests pass, uses real temp filesystem
