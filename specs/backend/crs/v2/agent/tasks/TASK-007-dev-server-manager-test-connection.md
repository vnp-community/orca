# TASK-007: Sửa `dev-server-manager.ts` — `testConnection()` cho relay-websocket

> **Status:** ✅ DONE (2026-07-26)
> **TypeScript:** 0 errors
> **Regression tests:** 19/19 pass

**Status:** ✅ DONE  
**Phase:** 3 — relay-websocket mode  
**Solution:** [SOL-AG-003](../solutions/SOL-AG-003-relay-websocket.md) §3.2  
**Depends on:** TASK-006  
**Blocks:** (không có — TASK-011 parallel)  

---

## Mục tiêu

Sửa `DevServerManager.testConnection()` để handle `relay-websocket` mode
mà không cần SSH connect trước. Hiện tại code gọi `connectRegisteredSshTarget`
cho mọi connection type — nhưng relay-websocket không có SSH target.

---

## File cần sửa

**Path:** `src/main/dev-server/dev-server-manager.ts`

---

## Phân tích code hiện tại

```typescript
// Tìm testConnection() trong DevServerManager — khoảng line 80-140
async testConnection(input: DevServerInput): Promise<ConnectionTestResult> {
  // Hiện tại: xử lý relay-ssh
  // Vấn đề: không có branch cho relay-websocket
}
```

---

## Thay đổi cần thực hiện

**Tìm method `testConnection()` trong `DevServerManager` và sửa:**

```typescript
  async testConnection(input: DevServerInput): Promise<ConnectionTestResult> {
    // relay-websocket và direct-websocket: không cần SSH setup
    // Tạo ephemeral bridge và test trực tiếp
    if (
      input.connectionType === 'relay-websocket' ||
      input.connectionType === 'direct-websocket'
    ) {
      const ephemeral: PersistedDevServer = {
        id: 'test-ephemeral',
        name: input.name ?? 'test',
        connectionType: input.connectionType,
        sshTargetId: undefined,
        wsUrl: input.wsUrl,
        workspaceDir: null,
        addedAt: Date.now(),
      }
      const bridge = new DevServerRelayBridge(ephemeral, this.sshManager)
      try {
        const info = await bridge.connect({ testOnly: true })
        return {
          ok: true,
          platform: info.platform,
          nodeVersion: info.nodeVersion,
        }
      } catch (err) {
        return {
          ok: false,
          error: err instanceof Error ? err.message : String(err),
        }
      }
      // bridge auto-disconnects in testOnly mode, no finally needed
    }

    // relay-ssh: existing logic (giữ nguyên phần này)
    // ... existing SSH connect logic ...
  }
```

**Quan trọng:** Giữ nguyên toàn bộ logic relay-ssh hiện có. Chỉ thêm branch đầu cho
`relay-websocket` và `direct-websocket` trước phần relay-ssh.

---

## Import cần thêm (nếu chưa có)

```typescript
import { DevServerRelayBridge } from './dev-server-relay-bridge'
import type { PersistedDevServer, DevServerInput, ConnectionTestResult } from '../../shared/dev-server-types'
```

---

## Acceptance Criteria

- [x] `testConnection({ connectionType: 'relay-websocket', wsUrl: '...' })` không gọi SSH connect
- [x] `testConnection({ connectionType: 'relay-websocket', wsUrl: 'ws://...' })` trả về `{ ok: true }` khi agent accessible
- [x] `testConnection({ connectionType: 'relay-ssh', ... })` vẫn hoạt động bình thường (không regression)
- [x] Khi relay-websocket connect fail → `{ ok: false, error: 'relay-websocket: ...' }`
- [x] TypeScript compile không lỗi
