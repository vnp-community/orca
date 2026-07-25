# TASK-011: Tạo `src/main/server-bootstrap.ts`

**Source:** SOL-BE-004  
**Phase:** 2 | **Effort:** M (1.5–2 giờ)  
**Depends on:** TASK-008, TASK-010

---

## Objective

Tạo `src/main/server-bootstrap.ts` — selective initialization sequence cho Node.js server mode. File này gọi đúng subset của startup sequence (không tạo window, không tray icon) và trả về RPC server handle.

---

## Context cần đọc trước

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# Đọc init sequence hiện tại
cat src/main/index.ts | head -100

# Xem persistence setup
grep -n "Store.init\|initDataPath\|initOrcaProfilePaths" src/main/*.ts | head -20

# Xem runtime init
grep -n "OrcaRuntimeService\|OrcaRuntimeRpcServer" src/main/*.ts | head -20

# Xem daemon init
grep -n "initDaemonPtyProvider\|daemon" src/main/*.ts | head -20
```

---

## File to create

### `src/main/server-bootstrap.ts`

```typescript
/**
 * Server Bootstrap — Selective initialization for Node.js server mode.
 *
 * Initializes only the backend services needed for headless operation:
 * - Persistence (SQLite)
 * - PTY Daemon
 * - Orca Runtime Service (all business logic)
 * - IPC Handlers (so web clients can call backend)
 * - WebSocket RPC Server (for remote/mobile clients)
 *
 * Does NOT initialize:
 * - BrowserWindow / GUI
 * - System tray
 * - Electron-specific hooks
 * - OS notifications
 *
 * @module main/server-bootstrap
 */

import type { IPlatformServices } from '../platform/types'

export interface ServerBootstrapResult {
  /** The WebSocket RPC server — call .close() to stop */
  rpcServer: { close(): Promise<void> }
  /** Port the HTTP/WS server is listening on */
  port: number
}

export interface ServerBootstrapOptions {
  platform: IPlatformServices
  /** HTTP port for WS + static files. Default: 6768 */
  port?: number
  /** Whether to serve web static files. Default: true */
  serveWebFiles?: boolean
}

/**
 * Initialize all Orca backend services for server mode.
 *
 * Call this AFTER setPlatform() has been called.
 * The function is idempotent per process (subsequent calls are no-ops).
 */
export async function initializeOrcaServices(
  options: ServerBootstrapOptions
): Promise<ServerBootstrapResult> {
  const { platform, port: requestedPort = 6768, serveWebFiles = true } = options

  // 1. Initialize data paths
  const userDataPath = platform.app.getPath('userData')
  console.log('[ServerBootstrap] userData:', userDataPath)

  // 2. Initialize core persistence (SQLite)
  const { Store } = await import('./persistence')
  await Store.init({ userDataPath })
  console.log('[ServerBootstrap] SQLite initialized')

  // 3. Initialize Orca profile paths
  try {
    const { initOrcaProfilePaths } = await import('./init-orca-profile-paths')
    await initOrcaProfilePaths(platform.app)
  } catch (err) {
    // Non-fatal — profile paths are best-effort
    console.warn('[ServerBootstrap] initOrcaProfilePaths failed:', err)
  }

  // 4. Initialize PTY daemon provider
  try {
    const { initDaemonPtyProvider } = await import('./daemon/daemon-init')
    await initDaemonPtyProvider({ userDataPath })
    console.log('[ServerBootstrap] PTY daemon initialized')
  } catch (err) {
    console.warn('[ServerBootstrap] PTY daemon init failed (terminal features may not work):', err)
  }

  // 5. Register IPC Handlers (all core handlers)
  const { registerCoreHandlers } = await import('./ipc/register-core-handlers')
  registerCoreHandlers({ ipc: platform.ipc, windowManager: platform.windowManager })
  console.log('[ServerBootstrap] IPC handlers registered')

  // 6. Initialize Orca Runtime Service
  const { OrcaRuntimeService } = await import('./runtime/orca-runtime')
  const runtime = await OrcaRuntimeService.init({
    platform,
    userDataPath,
    port: requestedPort,
    serveStaticFiles: serveWebFiles
  })
  console.log(`[ServerBootstrap] Runtime initialized on port ${requestedPort}`)

  // 7. Start WebSocket RPC Server
  await runtime.rpcServer.start()
  const actualPort = runtime.rpcServer.port ?? requestedPort
  console.log(`[ServerBootstrap] RPC server listening on :${actualPort}`)

  return {
    rpcServer: runtime.rpcServer,
    port: actualPort
  }
}
```

---

## Test file

### `src/main/__tests__/server-bootstrap.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { existsSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'

// ── Mock heavy dependencies ──────────────────────────────────────────────────
vi.mock('../persistence', () => ({
  Store: { init: vi.fn().mockResolvedValue(undefined) }
}))

vi.mock('../init-orca-profile-paths', () => ({
  initOrcaProfilePaths: vi.fn().mockResolvedValue(undefined)
}))

vi.mock('../daemon/daemon-init', () => ({
  initDaemonPtyProvider: vi.fn().mockResolvedValue(undefined)
}))

vi.mock('../ipc/register-core-handlers', () => ({
  registerCoreHandlers: vi.fn()
}))

const mockRpcServer = {
  start: vi.fn().mockResolvedValue(undefined),
  close: vi.fn().mockResolvedValue(undefined),
  port: 16768
}

vi.mock('../runtime/orca-runtime', () => ({
  OrcaRuntimeService: {
    init: vi.fn().mockResolvedValue({ rpcServer: mockRpcServer })
  }
}))
// ─────────────────────────────────────────────────────────────────────────────

const testPath = join(tmpdir(), `orca-bootstrap-test-${Date.now()}`)

function createMockPlatform() {
  return {
    mode: 'node' as const,
    app: {
      getPath: (name: string) => name === 'userData' ? testPath : `/tmp/${name}`,
      getVersion: () => '0.0.0',
      isPackaged: true,
      whenReady: () => Promise.resolve(),
      quit: vi.fn(),
      exit: vi.fn(),
      relaunch: vi.fn(),
      setName: vi.fn(),
      disableHardwareAcceleration: vi.fn(),
      getAppPath: () => testPath,
      on: vi.fn(), off: vi.fn(), once: vi.fn(), emit: vi.fn()
    },
    ipc: {
      handle: vi.fn(), removeHandler: vi.fn(),
      on: vi.fn(), off: vi.fn(), emit: vi.fn(),
      sendToWindow: vi.fn(), sendToAll: vi.fn()
    },
    windowManager: {
      createWindow: vi.fn().mockReturnValue({ id: 1, on: vi.fn(), destroy: vi.fn() }),
      getAllWindows: vi.fn().mockReturnValue([]),
      getFocusedWindow: vi.fn().mockReturnValue(null),
      getMainWindow: vi.fn().mockReturnValue(null),
      setMainWindow: vi.fn()
    },
    storage: {
      isEncryptionAvailable: vi.fn().mockReturnValue(false),
      encryptString: vi.fn((s: string) => Buffer.from(s)),
      decryptString: vi.fn((b: Buffer) => b.toString())
    },
    system: {}
  }
}

describe('initializeOrcaServices()', () => {
  afterEach(() => {
    vi.clearAllMocks()
    if (existsSync(testPath)) rmSync(testPath, { recursive: true })
  })

  it('calls Store.init with correct userDataPath', async () => {
    const { initializeOrcaServices } = await import('../server-bootstrap')
    const { Store } = await import('../persistence')
    const platform = createMockPlatform()

    await initializeOrcaServices({ platform })

    expect(Store.init).toHaveBeenCalledWith(
      expect.objectContaining({ userDataPath: testPath })
    )
  })

  it('calls initDaemonPtyProvider', async () => {
    const { initializeOrcaServices } = await import('../server-bootstrap')
    const { initDaemonPtyProvider } = await import('../daemon/daemon-init')

    await initializeOrcaServices({ platform: createMockPlatform() })

    expect(initDaemonPtyProvider).toHaveBeenCalledOnce()
  })

  it('calls registerCoreHandlers with ipc from platform', async () => {
    const { initializeOrcaServices } = await import('../server-bootstrap')
    const { registerCoreHandlers } = await import('../ipc/register-core-handlers')
    const platform = createMockPlatform()

    await initializeOrcaServices({ platform })

    expect(registerCoreHandlers).toHaveBeenCalledWith(
      expect.objectContaining({ ipc: platform.ipc })
    )
  })

  it('calls rpcServer.start()', async () => {
    const { initializeOrcaServices } = await import('../server-bootstrap')
    await initializeOrcaServices({ platform: createMockPlatform() })
    expect(mockRpcServer.start).toHaveBeenCalledOnce()
  })

  it('does NOT call windowManager.createWindow()', async () => {
    const { initializeOrcaServices } = await import('../server-bootstrap')
    const platform = createMockPlatform()

    await initializeOrcaServices({ platform })

    expect(platform.windowManager.createWindow).not.toHaveBeenCalled()
  })

  it('returns rpcServer and port', async () => {
    const { initializeOrcaServices } = await import('../server-bootstrap')
    const result = await initializeOrcaServices({ platform: createMockPlatform() })

    expect(result.rpcServer).toBeDefined()
    expect(result.port).toBe(16768)
  })

  it('throws if Store.init fails', async () => {
    const { initializeOrcaServices } = await import('../server-bootstrap')
    const { Store } = await import('../persistence')
    ;(Store.init as any).mockRejectedValueOnce(new Error('SQLite failed'))

    await expect(
      initializeOrcaServices({ platform: createMockPlatform() })
    ).rejects.toThrow('SQLite failed')
  })

  it('continues if initOrcaProfilePaths fails (non-fatal)', async () => {
    const { initializeOrcaServices } = await import('../server-bootstrap')
    const { initOrcaProfilePaths } = await import('../init-orca-profile-paths')
    ;(initOrcaProfilePaths as any).mockRejectedValueOnce(new Error('profile fail'))

    // Should NOT throw — profile paths are non-fatal
    await expect(
      initializeOrcaServices({ platform: createMockPlatform() })
    ).resolves.toBeDefined()
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript compile
npx tsc --noEmit 2>&1 | grep "server-bootstrap" | head -10

# Run tests
npx vitest run src/main/__tests__/server-bootstrap.test.ts
```

**Lưu ý:** Nếu `import('./persistence')` hoặc các module khác không tồn tại, cần điều chỉnh import path cho đúng với codebase thực tế. Đọc `src/main/index.ts` để xác định đúng module names trước khi implement.

Expected: **8+ tests pass**.

---

## Done criteria

- [x] `src/main/server-bootstrap.ts` tạo thành công
- [x] `initializeOrcaServices()` gọi đúng sequence: Store → daemon → handlers → rpcServer
- [x] Không có `createWindow()` call trong server bootstrap
- [x] `initOrcaProfilePaths` fail không crash server
- [x] 8+ tests pass
