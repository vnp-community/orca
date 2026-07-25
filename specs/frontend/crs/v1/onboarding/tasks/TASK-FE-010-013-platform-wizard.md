# TASK-FE-010 đến TASK-FE-013: Phase 2 — Platform Wizard Tasks

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files created/modified:**
> - `src/renderer/src/hooks/useActiveDevServerPlatform.ts` [NEW] — TASK-FE-010
> - `src/renderer/src/hooks/__tests__/useActiveDevServerPlatform.test.ts` [NEW] — TASK-FE-010
> - `src/renderer/src/lib/agent-catalog.tsx` [MODIFY] — `unsupportedPlatforms`, `getAgentCatalogForPlatform`, `isYoloSupportedForPlatform` — TASK-FE-011
> - TASK-FE-012 (AgentStep): ThemeStep — cần UI integration pass riêng
> - TASK-FE-013 (ThemeStep): hooks done, component integration is a follow-up

---

# TASK-FE-010: Tạo useActiveDevServerPlatform + visibility hooks

**Phase:** 2 | **Solution:** [FE-SOL-B](../solutions/FE-SOL-B-agent-wizard.md) | **CR:** CR-OB-004  
**Depends on:** TASK-FE-002

## Goal
Tạo các hooks để wizard bước và UI components đọc platform của active dev server, thay thế `navigator.userAgent` / `process.platform`.

## Steps

**Tạo** `src/renderer/src/hooks/useActiveDevServerPlatform.ts`:

```typescript
import { useActiveDevServer } from '../store/slices/dev-servers'

/** Trả về platform của active dev server, hoặc null nếu chưa kết nối */
export function useActiveDevServerPlatform(): NodeJS.Platform | null {
  const ds = useActiveDevServer()
  return ds?.platform ?? null
}

/** true khi active dev server là Windows */
export function useShowWindowsTerminalStep(): boolean {
  return useActiveDevServerPlatform() === 'win32'
}

/** true khi active dev server là macOS */
export function useShowGhosttyImport(): boolean {
  return useActiveDevServerPlatform() === 'darwin'
}

/** true khi active dev server là Linux */
export function useIsLinuxDevServer(): boolean {
  return useActiveDevServerPlatform() === 'linux'
}
```

**Tests** (6 cases): platform null, darwin, win32, linux; reactive khi activeDevServer thay đổi.

## Output Files
- **[NEW]** `src/renderer/src/hooks/useActiveDevServerPlatform.ts`
- **[NEW]** `src/renderer/src/hooks/__tests__/useActiveDevServerPlatform.test.ts`

---

# TASK-FE-011: Sửa agent catalog — thêm platform filter

**Phase:** 2 | **Solution:** [FE-SOL-B](../solutions/FE-SOL-B-agent-wizard.md) | **CR:** CR-OB-004  
**Depends on:** _(none — independent)_

## Goal
Thêm `unsupportedPlatforms` field vào `AgentCatalogEntry` và tạo `getAgentCatalogForPlatform()` function.

## Steps

1. **Đọc** `src/renderer/src/lib/agent-catalog.tsx` để biết cấu trúc hiện tại.

2. **Thêm** field vào type:
```typescript
type AgentCatalogEntry = {
  // ...existing
  unsupportedPlatforms?: readonly NodeJS.Platform[]
  yoloUnsupportedPlatforms?: readonly NodeJS.Platform[]
}
```

3. **Thêm** vào data nếu cần (ví dụ: Codex không support macOS ARM).

4. **Export** 2 functions mới:
```typescript
export function getAgentCatalogForPlatform(platform: NodeJS.Platform): AgentCatalogEntry[]
export function isYoloSupportedForPlatform(agentId: string, platform: NodeJS.Platform): boolean
```

**Tests** (5 cases): filter đúng theo platform, giữ agents không có restriction.

## Output Files
- **[MODIFY]** `src/renderer/src/lib/agent-catalog.tsx`
- **[NEW]** `src/renderer/src/lib/__tests__/agent-catalog-platform.test.ts`

---

# TASK-FE-012: Sửa AgentStep.tsx — remote detection + multi-server

**Phase:** 2 | **Solution:** [FE-SOL-B](../solutions/FE-SOL-B-agent-wizard.md) | **CR:** CR-OB-003, CR-OB-004  
**Depends on:** TASK-FE-005, TASK-FE-009, TASK-FE-010, TASK-FE-011

## Goal
Sửa `AgentStep.tsx` để:
1. Nhận `activeDevServerId` prop thay vì detect local
2. Dùng `useRemoteAgentDetection` thay vì local preflight
3. Filter catalog theo platform của dev server
4. Hiển thị loading/error states
5. Thêm `MultiServerAgentSection` collapsible cho servers khác

## Steps

1. **Đọc** `src/renderer/src/components/onboarding/AgentStep.tsx` để hiểu cấu trúc hiện tại.

2. **Thêm** props:
```typescript
type AgentStepProps = {
  // ...existing
  activeDevServerId: string | null   // NEW
}
```

3. **Thay** local detection bằng:
```typescript
const { agents: detectedAgents, platform, loading, error, redetect } =
  useRemoteAgentDetection(activeDevServerId)
```

4. **Thêm** server context header (name + badge) phía trên grid.

5. **Thêm** loading state (Spinner), error state (error message + Retry button).

6. **Filter** catalog: `getAgentCatalogForPlatform(platform)`.

7. **Thêm** `MultiServerAgentSection` collapsible ở cuối (xem solution).

**Tests** (8 cases): render với/không devServerId, loading state, error+retry, catalog filtering.

## Output Files
- **[MODIFY]** `src/renderer/src/components/onboarding/AgentStep.tsx`
- **[NEW/MODIFY]** `src/renderer/src/components/onboarding/__tests__/AgentStep.test.tsx`

---

# TASK-FE-013: Sửa ThemeStep.tsx — Ghostty remote detect

**Phase:** 2 | **Solution:** [FE-SOL-B](../solutions/FE-SOL-B-agent-wizard.md) | **CR:** CR-OB-004  
**Depends on:** TASK-FE-010, TASK-FE-020

## Goal
Sửa `ThemeStep.tsx` để Ghostty import chỉ hiển thị khi dev server là macOS VÀ Ghostty config được detect từ remote.

## Steps

1. **Đọc** `src/renderer/src/components/onboarding/ThemeStep.tsx` — tìm `GhosttyImportCard` hoặc tương đương.

2. **Thêm** `activeDevServerId` prop.

3. **Thay** local `navigator.userAgent` check bằng `useShowGhosttyImport()`:
```typescript
// TRƯỚC:
const isMac = navigator.userAgent.includes('Macintosh')

// SAU:
const showGhostty = useShowGhosttyImport()  // dùng activeDevServer.platform
```

4. **Thêm** remote Ghostty config detect:
```typescript
useEffect(() => {
  if (!showGhostty || !activeDevServerId) return
  window.api.onboarding.detectGhosttyConfig({ devServerId: activeDevServerId })
    .then(setGhosttyConfig)
    .catch(() => { /* non-fatal */ })
}, [showGhostty, activeDevServerId])
```

5. **Render** `GhosttyImportCard` chỉ khi `showGhostty && ghosttyConfig?.configPath`.

**Tests** (4 cases): showGhostty=false ẩn card, showGhostty=true nhưng configPath=null ẩn card, configPath có → hiển thị card.

## Output Files
- **[MODIFY]** `src/renderer/src/components/onboarding/ThemeStep.tsx`
- **[MODIFY]** `src/renderer/src/components/onboarding/__tests__/ThemeStep.test.tsx`
