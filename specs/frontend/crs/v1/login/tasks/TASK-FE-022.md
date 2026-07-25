# TASK-FE-022 — Extend `store/slices/ssh.ts` — User Accounts State

**Phase:** 4 — SSH UI
**Solution:** [SOL-FE-LG-004](../solutions/SOL-FE-LG-004-ssh-ui.md) §3, §5.5
**Depends on:** TASK-FE-003 (store pattern reference)
**Blocks:** TASK-FE-023, TASK-FE-024
**Effort:** S (~20 phút)
**Status:** ✅ Done

---

## Mô tả

Mở rộng SSH Zustand slice với state tracking linux user accounts và provisioning status per server.

---

## File cần sửa

### `src/renderer/src/store/slices/ssh.ts` [MODIFY]

**Thêm types:**

```typescript
type ProvisioningStatus =
  | { phase: 'idle' }
  | { phase: 'checking' }
  | { phase: 'provisioning'; step: string; progress: number }
  | { phase: 'done'; linuxUsername: string }
  | { phase: 'error'; message: string }

type SshUserAccount = {
  linuxUsername: string
  provisioned: boolean
  provisioningStatus: ProvisioningStatus
}
```

**Thêm vào slice state:**

```typescript
sshUserAccounts: new Map<string, SshUserAccount>(),
```

**Thêm actions:**

```typescript
setSshUserAccount: (serverId: string, account: SshUserAccount) =>
  set(state => ({
    sshUserAccounts: new Map(state.sshUserAccounts).set(serverId, account)
  })),

updateProvisioningStatus: (serverId: string, status: ProvisioningStatus) =>
  set(state => {
    const existing = state.sshUserAccounts.get(serverId)
    if (!existing) return state
    return {
      sshUserAccounts: new Map(state.sshUserAccounts).set(serverId, {
        ...existing,
        provisioningStatus: status
      })
    }
  }),
```

**Thêm vào AppState type** (nếu ssh slice có type export riêng):

```typescript
export type SshSlice = {
  // ... existing fields ...
  sshUserAccounts: Map<string, SshUserAccount>
  setSshUserAccount: (serverId: string, account: SshUserAccount) => void
  updateProvisioningStatus: (serverId: string, status: ProvisioningStatus) => void
}
```

---

## Constraints

- KHÔNG xóa hoặc thay đổi bất kỳ existing state/action nào trong ssh.ts
- Chỉ ADD mới — additive only
- Sử dụng `new Map(state.sshUserAccounts)` khi update (immutable pattern)

---

## Verify

```bash
npx tsc --noEmit
# Expected: 0 TypeScript errors
```
