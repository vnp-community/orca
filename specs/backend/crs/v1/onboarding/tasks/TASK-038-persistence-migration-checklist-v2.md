# TASK-038: Sửa `src/main/persistence.ts` — Migration Checklist v1 → v2

**Phase:** 3 — Multi Dev-Server Checklist  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §C.2  
**Depends on:** TASK-037  
**Blocks:** (không)

---

## Mục tiêu

Thêm migration chuyển `OnboardingChecklistState` từ flat format (v1) sang `perServer` format (v2), giữ items cũ dưới key `'local'`.

---

## File cần sửa

**Path:** `src/main/persistence.ts`

---

## Thay đổi cần thực hiện

```typescript
import type { PerServerChecklistState } from '../../shared/types'

function migrateOnboardingChecklist(onboarding: OnboardingState): OnboardingState {
  const cl = onboarding.checklist
  if (!cl || cl.perServer !== undefined) return onboarding  // đã migrate hoặc chưa có

  // v1 → v2: move flat per-server items sang perServer['local']
  const PER_SERVER_KEYS: (keyof PerServerChecklistState)[] = [
    'addedRepo', 'ranFirstAgent', 'ranSecondAgentOnSameTask',
    'reviewedDiff', 'openedPr', 'addedFolder', 'openedFile', 'ranAgentOnFile'
  ]

  const perServerItems: PerServerChecklistState = {}
  for (const key of PER_SERVER_KEYS) {
    if ((cl as Record<string, unknown>)[key] === true) {
      perServerItems[key] = true
    }
  }

  return {
    ...onboarding,
    checklist: {
      choseAgent: cl.choseAgent,
      triedCmdJ: cl.triedCmdJ,
      shapedSidebar: cl.shapedSidebar,
      perServer: Object.keys(perServerItems).length > 0
        ? { local: perServerItems }
        : {}
    }
  }
}
```

Gọi migration trong normalize pipeline (sau `migrateDevServers`):

```typescript
// Trong normalizeLoadedState() hoặc tương đương:
if (state.onboarding) {
  state.onboarding = migrateOnboardingChecklist(state.onboarding)
}
```

---

## Acceptance Criteria

- [x] State v1 (có `addedRepo: true` ở flat) → migrate sang `perServer: { local: { addedRepo: true } }`
- [x] State đã có `perServer` → không migrate lại (idempotent)
- [x] Global items `choseAgent`, `triedCmdJ`, `shapedSidebar` giữ nguyên sau migrate
- [x] Per-server items flat bị `false` hoặc `undefined` → không xuất hiện trong `perServer.local`
- [x] Empty per-server items → `perServer: {}` (không tạo key 'local')
- [x] TypeScript compile thành công
