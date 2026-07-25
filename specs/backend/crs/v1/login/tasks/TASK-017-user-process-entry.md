# TASK-017: Tạo `src/main/session/user-process-entry.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 2 — User Sandbox
**Solution:** [SOL-LG-002](../solutions/SOL-LG-002-user-sandbox.md) §4.4
**Depends on:** TASK-016
**Blocks:** TASK-018 (server/index.ts)

---

## Mục tiêu

Tạo entry point cho forked user process — mỗi user process chạy OrcaRuntime riêng với userDataPath riêng.

---

## File cần tạo

**Path:** `src/main/session/user-process-entry.ts`

> **Lưu ý quan trọng**: File này là **entry point của child process**, KHÔNG được import từ supervisor process. Nó sẽ được compiled và path của built `.js` file truyền vào `fork()`.

---

## Nội dung

```typescript
// src/main/session/user-process-entry.ts
// ⚠️ Entry point for forked user process — NOT imported by supervisor

const userId   = process.env.ORCA_USER_ID
const dataPath = process.env.ORCA_USER_DATA_PATH
const sockPath = process.env.ORCA_SOCKET_PATH

if (!userId || !dataPath || !sockPath) {
  console.error('[UserProcess] ERROR: Missing required env vars:', {
    ORCA_USER_ID:        Boolean(userId),
    ORCA_USER_DATA_PATH: Boolean(dataPath),
    ORCA_SOCKET_PATH:    Boolean(sockPath)
  })
  process.exit(1)
}

console.log(`[UserProcess] Starting: userId=${userId}, sockPath=${sockPath}`)

async function main(): Promise<void> {
  // Dynamic imports để tránh circular dependencies với supervisor
  const { createNodeAdapter } = await import('../../platform/adapters/node')
  const { setPlatform }       = await import('../../platform/context')
  const { initializeOrcaServices } = await import('../server-bootstrap')

  // Khởi tạo platform adapter với per-user data path
  const adapter = createNodeAdapter({ userDataPath: dataPath! })
  setPlatform(adapter)

  // Boot OrcaRuntime với:
  //   - socketPath thay vì TCP port (server chỉ listen Unix socket)
  //   - userDataPath riêng cho mỗi user
  const { shutdown } = await initializeOrcaServices({
    platform:     adapter,
    socketPath:   sockPath!,
    userDataPath: dataPath!,
    userId:       userId!,
  })

  // Báo hiệu supervisor: sẵn sàng nhận connections
  process.send!({ type: 'ready', socketPath: sockPath })
  console.log(`[UserProcess] Ready: userId=${userId}`)

  // Graceful shutdown handlers
  const handleExit = async (signal: string) => {
    console.log(`[UserProcess] Shutting down (${signal}): userId=${userId}`)
    await shutdown()
    process.exit(0)
  }
  process.on('SIGTERM', () => void handleExit('SIGTERM'))
  process.on('SIGINT',  () => void handleExit('SIGINT'))
}

main().catch((err: unknown) => {
  console.error(`[UserProcess] Fatal error: userId=${userId}`, err)
  process.exit(1)
})
```

---

## Vite/Build config

File này cần được compile riêng hoặc include trong build output. Thêm vào build config nếu cần:

```typescript
// vite.server.config.ts — MODIFY (nếu cần build entry separately)
// Thêm user-process-entry.ts là thêm entry point riêng:
build: {
  rollupOptions: {
    input: {
      index: 'src/server/index.ts',
      'user-process-entry': 'src/main/session/user-process-entry.ts'  // NEW
    }
  }
}
```

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] Validate env vars, exit(1) nếu thiếu
- [x] Gọi `initializeOrcaServices()` với `socketPath` (Unix socket, không phải TCP port)
- [x] `process.send({ type: 'ready', socketPath })` sau khi services init xong
- [x] SIGTERM/SIGINT → graceful shutdown + process.exit(0)
- [x] Không export gì (entry-only file)
