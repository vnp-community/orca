# TASK-002-A — Thêm Grouping State vào SshSlice

**Task ID:** TASK-002-A  
**CR:** CR-002 — Server Grouping by Project/Team  
**Solution Ref:** SOL-CR-002, Section 2.2  
**Dependencies:** TASK-001-A  
**Estimated:** 1 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Mở rộng `SshSlice` để lưu danh sách `sshTargets` từ backend sync và state `collapsedSshGroups` cho UI collapsible groups.

---

## Bước thực thi

### Bước 1: Mở rộng SshSlice interface

Trong `src/renderer/src/store/slices/ssh.ts`, thêm vào type `SshSlice`:

```typescript
// Full targets list synced from RuntimeSyncWindowGraph
sshTargets: SshTarget[]
setSshTargets: (targets: SshTarget[]) => void

// Collapsed group state (keyed by project name or '__unassigned__')
collapsedSshGroups: Record<string, boolean>
toggleSshGroupCollapsed: (groupKey: string) => void
```

### Bước 2: Implement trong createSshSlice

```typescript
sshTargets: [],
collapsedSshGroups: {},

setSshTargets: (targets) =>
  set(s => { s.sshTargets = targets }),

toggleSshGroupCollapsed: (groupKey) =>
  set(s => {
    s.collapsedSshGroups[groupKey] = !s.collapsedSshGroups[groupKey]
  }),
```

### Bước 3: Import SshTarget type

Đảm bảo `SshTarget` được import từ đúng location:

```bash
grep -rn "type SshTarget\|interface SshTarget" src/shared/ src/renderer/src/
```

### Bước 4: Cập nhật sync-runtime-graph để populate sshTargets

Tìm file `src/renderer/src/runtime/sync-runtime-graph.ts`, tìm chỗ apply `sshTargets` từ graph:

```bash
grep -n "sshTargets\|setSshTargets" src/renderer/src/runtime/sync-runtime-graph.ts
```

Nếu chưa có, thêm vào `applyRuntimeGraph` hoặc `performRuntimeGraphSync`:

```typescript
// Trong apply graph function:
if (graph.sshTargets) {
  useAppStore.getState().setSshTargets(graph.sshTargets)
}
```

### Bước 5: Verify

```bash
npx tsc --noEmit 2>&1 | grep "sshTargets\|collapsedSsh" | head -10
```

---

## Acceptance Criteria

- [x] `sshTargets: SshTarget[]` trong SshSlice (default `[]`)
- [x] `setSshTargets()` action hoạt động
- [x] `collapsedSshGroups: Record<string, boolean>` trong SshSlice
- [x] `toggleSshGroupCollapsed()` toggle đúng (true/false)
- [x] `sync-runtime-graph.ts` gọi `setSshTargets()` khi graph update
- [x] TypeScript compile clean

---

## Notes cho AI

- `SshTarget` type ở `src/shared/ssh-types.ts` — có thể cần fields mới `project?`, `team?`, `environment?`
- Nếu `SshTarget` chưa có các fields này, thêm vào shared type (optional fields, backward compatible)
- `collapsedSshGroups` không cần persist (reset khi app restart là OK)

---

## Implementation Notes

> **Completed:** 2026-07-23 | `store/slices/ssh.ts`: sshTargets: SshTarget[], setSshTargets action, collapsedSshGroups: Record<string,boolean>, toggleSshGroupCollapsed. `ssh-types.ts`: project?, team?, environment? fields added to SshTarget + SshTargetGroup type + groupSshTargetsByProject util. TypeScript: ✅ 0 errors.
