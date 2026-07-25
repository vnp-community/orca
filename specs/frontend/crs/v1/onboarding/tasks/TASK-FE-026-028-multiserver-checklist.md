# TASK-FE-026 đến TASK-FE-028: Phase 3 — Multi-Server Checklist

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files created/modified:**
> - `src/renderer/src/store/slices/onboarding-checklist.ts` [NEW] — Zustand slice + `useServerChecklist` selector — TASK-FE-026
> - `src/renderer/src/store/slices/onboarding-checklist.test.ts` [NEW] — 7 unit tests — TASK-FE-026
> - `src/renderer/src/store/types.ts` [MODIFY] — added `OnboardingChecklistSlice` to AppState — TASK-FE-026
> - `src/renderer/src/store/index.ts` [MODIFY] — registered `createOnboardingChecklistSlice` — TASK-FE-026
> - `src/renderer/src/components/setup-guide/SetupGuideModal.tsx` [MODIFY] — `ServerChecklistSection`, `OverallProgressBar`, `PerServerChecklistPanel` — TASK-FE-027
> - `src/renderer/src/hooks/useIpcEvents.ts` [MODIFY] — `getDevServerForWorktree` helper + `ranFirstAgent` trigger — TASK-FE-028
> - `src/renderer/src/components/onboarding/AddRepoStep.tsx` [MODIFY] — `addedRepo` trigger on success — TASK-FE-028

---

# TASK-FE-026: Sửa Onboarding slice — perServer checklist + actions

**Phase:** 3 | **Solution:** [FE-SOL-D](../solutions/FE-SOL-D-platform-polish.md) | **CR:** CR-OB-009  
**Depends on:** TASK-FE-001, TASK-FE-020

## Goal
Sửa Zustand onboarding slice để:
1. Lưu `checklistState.perServer` (per-server items, keyed by devServerId)
2. Thêm actions `markGlobalChecklistItem` và `markServerChecklistItem`
3. Thêm selectors `useServerChecklist`

## Steps

1. **Đọc** slice onboarding hoặc phần checklist trong store (có thể trong `slices/ui.ts` hoặc `slices/preflight.ts`).

2. **Thêm** type vào slice:
```typescript
type OnboardingSlice = {
  checklistState: OnboardingChecklistState
  markGlobalChecklistItem: (
    item: keyof OnboardingChecklistState,
    value?: boolean
  ) => void
  markServerChecklistItem: (
    devServerId: string,
    item: keyof PerServerChecklistState,
    value?: boolean
  ) => void
}
```

3. **Implement** actions:
```typescript
markGlobalChecklistItem: (item, value = true) => {
  set((state) => ({
    checklistState: { ...state.checklistState, [item]: value },
  }))
  void window.api.onboarding.markChecklistItem({ item: item as string, value })
},

markServerChecklistItem: (devServerId, item, value = true) => {
  set((state) => ({
    checklistState: {
      ...state.checklistState,
      perServer: {
        ...state.checklistState.perServer,
        [devServerId]: {
          ...(state.checklistState.perServer?.[devServerId] ?? {}),
          [item]: value,
        },
      },
    },
  }))
  void window.api.onboarding.markChecklistItem({ item: item as string, devServerId, value })
},
```

4. **Export** selectors:
```typescript
export function useServerChecklist(devServerId: string | null): PerServerChecklistState {
  return useAppStore(
    useShallow((s) => (devServerId ? s.checklistState.perServer?.[devServerId] ?? {} : {}))
  )
}
```

5. **Sync** `checklistState` từ backend khi app khởi động (từ `settings.onboarding.checklist`).

**Tests** (7 cases): markGlobal, markServer, perServer isolation, selector reactive.

## Output Files
- **[MODIFY]** `src/renderer/src/store/slices/` (file liên quan đến onboarding/checklist)
- **[MODIFY]** `src/renderer/src/store/index.ts` (nếu cần thêm vào AppState)

---

# TASK-FE-027: Sửa SetupGuideModal.tsx — grouped per-server UI

**Phase:** 3 | **Solution:** [FE-SOL-D](../solutions/FE-SOL-D-platform-polish.md) | **CR:** CR-OB-009  
**Depends on:** TASK-FE-002, TASK-FE-005, TASK-FE-026

## Goal
Sửa `SetupGuideModal.tsx` để hiển thị checklist theo nhóm:
- **Global section**: choseAgent, notifications, two-worktrees, browser
- **Per-server sections**: 1 section per connected dev server
- **Setup section**: connect-dev-server, Orca CLI per server, setup-script
- **Overall progress bar**

## Steps

1. **Đọc** `src/renderer/src/components/setup-guide/SetupGuideModal.tsx` để hiểu cấu trúc hiện tại.

2. **Import** `useDevServers`, `useConnectedDevServers` selectors.

3. **Import** `useServerChecklist` selector.

4. **Tạo** `ServerChecklistSection` sub-component (inline):
```tsx
function ServerChecklistSection({ devServer, checklist }: {
  devServer: DevServer
  checklist: PerServerChecklistState
})
```
Với checklist items: addedRepo, ranFirstAgent, reviewedDiff, openedPr.  
Mỗi item chưa done → hiển thị action button phù hợp.

5. **Tạo** `OverallProgressBar` sub-component:
```tsx
function OverallProgressBar({ checklist, devServers }: {
  checklist: OnboardingChecklistState
  devServers: DevServer[]
})
```
Tính: global items (3) + per-server items (4 × N servers) = total.

6. **Sửa** main return để render:
- `<SetupSection title="General">` — global items
- `{connectedServers.map(ds => <ServerChecklistSection key={ds.id} ... />)}`
- `<SetupSection title="Setup">` — CLI items per server
- `<OverallProgressBar />`

7. **Thêm** `<ChecklistItem id="connect-dev-server">` với `done={connectedServers.length > 0}`.

**Tests** (8 cases):
- Render Global section với correct items
- Render 1 ServerChecklistSection per connected server
- ServerChecklistSection hiển thị items cho đúng server
- OverallProgressBar tính đúng (N global + N×servers per-server)
- connect-dev-server done khi có ≥1 connected server
- No connected servers → show connect prompt

## Output Files
- **[MODIFY]** `src/renderer/src/components/setup-guide/SetupGuideModal.tsx`
- **[MODIFY]** `src/renderer/src/components/setup-guide/__tests__/SetupGuideModal.test.tsx`

---

# TASK-FE-028: Thêm checklist triggers vào IPC event handlers

**Phase:** 3 | **Solution:** [FE-SOL-D](../solutions/FE-SOL-D-platform-polish.md) | **CR:** CR-OB-009  
**Depends on:** TASK-FE-026

## Goal
Tự động mark checklist items khi user thực hiện các hành động liên quan, thông qua existing IPC event handlers.

## Steps

1. **Đọc** `src/renderer/src/hooks/useIpcEvents.ts` — tìm handlers sau đây:
   - `onAgentStatusUpdate` (khi agent bắt đầu chạy → `ranFirstAgent`)
   - Nếu không có: tìm event tương đương

2. **Thêm** helper function:
```typescript
function getDevServerForWorktree(worktreeId: string): string | null {
  const repos = useAppStore.getState().repos
  const worktrees = useAppStore.getState().worktrees
  const wt = worktrees.find((w) => w.id === worktreeId)
  if (!wt) return null
  const repo = repos.find((r) => r.id === wt.repoId)
  return repo?.devServerId ?? null
}
```

3. **Sửa** `onAgentStatusUpdate` handler — thêm checklist trigger:
```typescript
window.api.onAgentStatusUpdate((event) => {
  store.updateAgentStatus(event)
  // NEW: mark ranFirstAgent
  if (event.status === 'running') {
    const devServerId = getDevServerForWorktree(event.worktreeId)
    if (devServerId) {
      const cl = useAppStore.getState().checklistState.perServer?.[devServerId]
      if (!cl?.ranFirstAgent) {
        useAppStore.getState().markServerChecklistItem(devServerId, 'ranFirstAgent')
      }
    }
  }
})
```

4. **Thêm** trigger khi repo được add thành công (tìm relevant event hoặc action):
```typescript
// Trong AddRepoStep.onRepoAdded hoặc repo.add handler:
const activeDevServerId = useAppStore.getState().activeDevServerId
if (activeDevServerId) {
  useAppStore.getState().markServerChecklistItem(activeDevServerId, 'addedRepo')
}
```

5. **Thêm** trigger `ranSecondAgentOnSameTask` khi agent thứ 2 start trên cùng worktree.

6. **Thêm** trigger `reviewedDiff` khi PR diff được mở (tìm relevant event).

**Tests** (5 cases):
- ranFirstAgent mark khi agent status = 'running' lần đầu
- ranFirstAgent không mark lại lần 2 (idempotent)
- addedRepo mark khi repo.addRemote thành công
- markServerChecklistItem gọi api.onboarding.markChecklistItem
- getDevServerForWorktree resolve đúng devServerId từ repo

## Output Files
- **[MODIFY]** `src/renderer/src/hooks/useIpcEvents.ts`
- **[MODIFY]** `src/renderer/src/components/onboarding/AddRepoStep.tsx`
- **[NEW/MODIFY]** `src/renderer/src/hooks/__tests__/checklist-triggers.test.ts`
