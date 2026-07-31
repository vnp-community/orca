# TASK-012: Wire ProfileService to Bootstrap (Step 7)

**Phase:** 2 — Profile Hierarchy  
**Solution ref:** [SOL-V5-000](../solutions/SOL-V5-000-server-bootstrap-changes.md) §2  
**Prerequisite:** TASK-005, TASK-007, TASK-008, TASK-011 (all tests pass)  
**Status:** ✅ DONE — 2026-07-28

---

## Mô tả

Thêm Step 7 (ProfileService + ProfileResolver) vào `initializeOrcaServices()` trong `src/main/server-bootstrap.ts`. Đồng thời thêm Step 2a-pool (RelayConnectionPool) vì nó cần được khởi tạo sớm.

---

## Thay đổi trong `src/main/server-bootstrap.ts`

### A. Thêm Step 2a-pool (sau DevServerManager ~line 118)

```typescript
// 2a-pool. Initialize RelayConnectionPool (v5.0 — prerequisite for Project + AI services)
const { RelayConnectionPool } = await import('./dev-server/relay-connection-pool')
const { DevServerRelayBridge } = await import('./dev-server/dev-server-relay-bridge')
const relayConnectionPool = new RelayConnectionPool(async (server) => {
  const bridge = new DevServerRelayBridge(server, sshManager, agentWsServer)
  await bridge.connect()
  return bridge
})
console.log('[ServerBootstrap] ✅ RelayConnectionPool initialized (v5.0)')
```

### B. Thêm Step 7 (sau FleetHealthMonitor `return` block — trước `return { ... }`)

```typescript
// 7. ProfileService + ProfileResolver [v5.0 TDD-14]
const { ProfileService } = await import('./profile/ProfileService')
const { ProfileResolver } = await import('./profile/ProfileResolver')
const profileService = new ProfileService(pool)
const profileResolver = new ProfileResolver(profileService)
console.log('[ServerBootstrap] ✅ ProfileService + ProfileResolver initialized (v5.0)')
```

### C. Update `return { ... }` block

Tìm và thay thế các placeholder `undefined as unknown as any` cho `profileService`, `profileResolver`, `relayConnectionPool` bằng các instance thực:

```typescript
return {
  // ... existing fields ...
  relayConnectionPool,    // replace placeholder
  profileService,         // replace placeholder
  profileResolver,        // replace placeholder
  // ... other v5.0 placeholders remain as undefined for now
  ...
}
```

### D. Update `shutdown()` — thêm RelayConnectionPool cleanup

```typescript
async shutdown() {
  // ... existing shutdown steps ...

  // [NEW v5.0]
  try {
    await relayConnectionPool.disconnectAll()
    console.log('[ServerBootstrap] ✅ RelayConnectionPool disconnected')
  } catch (err) {
    console.warn('[ServerBootstrap] RelayConnectionPool disconnect error:', err)
  }
}
```

---

## Verification

```bash
# TypeScript check
pnpm tsc --noEmit

# Ensure existing tests still pass
pnpm test --run src/main/

# Start server (smoke test)
node -e "require('./out/main/server-bootstrap')" 2>&1 | head -5
```

## Acceptance Criteria

- [x] `RelayConnectionPool` khởi tạo ở step 2a-pool
- [x] `ProfileService` + `ProfileResolver` ở step 7
- [x] `return` block có cả 3 fields thực (không còn `undefined`)
- [x] `shutdown()` có `relayConnectionPool.disconnectAll()`
- [x] Existing tests vẫn pass (`pnpm test --run src/main/auth/`)
- [x] Không TypeScript errors
