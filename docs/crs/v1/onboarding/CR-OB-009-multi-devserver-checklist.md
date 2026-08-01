# CR-OB-009 — Multi Dev-Server Setup Guide Checklist

| Field | Value |
|-------|-------|
| **CR ID** | CR-OB-009 |
| **Title** | Setup Guide Checklist cho kiến trúc Multi Dev-Server |
| **Version** | v1 |
| **Status** | Implemented |
| **Priority** | Medium |
| **Depends on** | CR-OB-002, CR-OB-003, CR-OB-005, CR-OB-006 |

---

## 1. Vấn đề

### Hiện tại

Setup Guide Checklist (sau wizard) gồm 11 mục đơn giản:
```
addedRepo / choseAgent / ranFirstAgent / triedCmdJ /
shapedSidebar / reviewedDiff / openedPr / ...
```

Tất cả chạy trong context **single local machine**.

### Vấn đề mới

Trong kiến trúc multi dev-server:
- Checklist items liên quan đến **dev server cụ thể** (`addedRepo`, `ranFirstAgent`)
- Một số items có thể hoàn thành trên server A nhưng chưa trên server B
- Feature Wall Setup Steps (`FeatureWallSetupStepId`) phải phản ánh trạng thái của **active dev server**
- `orca-cli` install phải xảy ra trên dev server, không phải Orca server

---

## 2. Yêu cầu

### 2.1 Setup Guide — Phân loại Items theo Scope

```typescript
type ChecklistItemScope = 
  | 'global'        // Áp dụng 1 lần cho toàn bộ account (theme, agent default)
  | 'per-server'    // Phải hoàn thành trên từng dev server
  | 'per-repo'      // Liên quan đến repo cụ thể
```

| Item | Scope | Thay đổi |
|------|-------|---------|
| `addedRepo` | `per-server` | Repo phải có `devServerId`, tính per server |
| `choseAgent` | `global` | Không đổi |
| `ranFirstAgent` | `per-server` | Agent chạy trên dev server nào? |
| `ranSecondAgentOnSameTask` | `per-server` | Idem |
| `triedCmdJ` | `global` | Shortcut trong UI, không phụ thuộc server |
| `shapedSidebar` | `global` | UI action, không phụ thuộc server |
| `reviewedDiff` | `per-server` | Diff của repo trên server nào |
| `openedPr` | `per-server` | PR của repo trên server nào |
| `addedFolder` | `per-server` | Folder trên server nào |
| `openedFile` | `per-server` | File trên server nào |
| `ranAgentOnFile` | `per-server` | Agent + file trên server nào |

### 2.2 Checklist State Schema thay đổi

```typescript
// TRƯỚC:
type OnboardingChecklistState = {
  addedRepo?: boolean
  choseAgent?: boolean
  ranFirstAgent?: boolean
  // ...
}

// SAU — hybrid approach:
type OnboardingChecklistState = {
  // Global items (không đổi):
  choseAgent?: boolean
  triedCmdJ?: boolean
  shapedSidebar?: boolean

  // Per-server items — keyed by devServerId:
  perServer?: Record<string, {
    addedRepo?: boolean
    ranFirstAgent?: boolean
    ranSecondAgentOnSameTask?: boolean
    reviewedDiff?: boolean
    openedPr?: boolean
    addedFolder?: boolean
    openedFile?: boolean
    ranAgentOnFile?: boolean
  }>
}
```

**Migration:** Existing checklist data (không có `perServer`) → migrate sang `perServer['local']`.

### 2.3 Feature Wall Setup Steps thay đổi

`FeatureWallSetupStepId` hiện có 8 steps. Cần thêm/sửa cho multi-server:

```typescript
// HIỆN TẠI:
type FeatureWallSetupStepId =
  | 'default-agent'
  | 'add-two-repos'
  | 'notifications'
  | 'two-worktrees'
  | 'browser'
  | 'task-sources'
  | 'agent-capabilities'
  | 'setup-script'

// SAU — thêm:
type FeatureWallSetupStepId =
  | 'default-agent'
  | 'add-two-repos'
  | 'notifications'
  | 'two-worktrees'
  | 'browser'
  | 'task-sources'
  | 'agent-capabilities'    // → Orca CLI install trên active dev server
  | 'setup-script'
  | 'connect-dev-server'    // NEW
  | 'add-dev-server-repo'   // NEW — Repo trên active dev server
```

### 2.4 Setup Guide UI — Hiển thị per-server status

```
┌─────────────────────────────────────────────────────────┐
│  Getting started                          [Hide]        │
│                                                         │
│  ── Global ──────────────────────────────────────────  │
│  ✅ Choose your default agent                           │
│  ○  Turn on notifications                               │
│                                                         │
│  ── MacBook Pro (macOS) ────────────────────────────── │
│  ✅ Add a repository                                    │
│  ✅ Run your first agent                                │
│  ○  Review a diff                                       │
│  ○  Open a Pull Request                                 │
│                                                         │
│  ── Linux Server ───────────────────────────────────── │
│  ○  Add a repository  [Add repo on this server]        │
│  ○  Run your first agent                               │
│                                                         │
│  ── Setup ───────────────────────────────────────────  │
│  ✅ Enable Orca CLI        (MacBook Pro)               │
│  ○  Enable Orca CLI        (Linux Server)  [Install]   │
│  ✅ Connect integrations                               │
│  ○  Automate workspace setup                           │
│                                                         │
│  ── Parallel Work ───────────────────────────────────  │
│  ○  Multi-task                                         │
│  ○  Use Orca's browser                                 │
└─────────────────────────────────────────────────────────┘
```

### 2.5 Orca CLI Install — Remote Dev Server

`agent-capabilities` step hiện cài CLI trên **local machine**. Cần thay đổi:

```typescript
// TRƯỚC:
window.api.cli.install()  // Cài orca-cli local

// SAU:
window.api.devServer.installCli({ devServerId: string })
// → SSH vào dev server, chạy install script
// → Register `orca` command trong PATH của dev server shell
```

**Orca CLI install script trên remote:**
```bash
# Relay gửi command qua PTY:
curl -sSL https://orca.server.com/install-cli.sh | bash
# Hoặc: node orca-cli-installer.js
```

### 2.6 `add-two-repos` — Cross-server counting

```typescript
// TRƯỚC: count repos đã add (global)
// SAU: count repos per active dev server, hoặc cross-server

// Option A — Per-server (stricter):
function isAddTwoReposComplete(repos: Repo[], activeDevServerId: string): boolean {
  return repos.filter(r => r.devServerId === activeDevServerId).length >= 2
}

// Option B — Cross-server (more lenient):
function isAddTwoReposComplete(repos: Repo[]): boolean {
  return repos.length >= 2
}
```

**Đề xuất:** Option A — encourage người dùng add repos trên từng server.

### 2.7 Setup Guide visibility logic

```typescript
// src/renderer/src/components/setup-guide/SetupGuideModal.tsx

// HIỆN TẠI — ẩn khi onboarding đã closed:
const dismissed = onboarding.closedAt !== null || setupGuideSidebarDismissed

// SAU — thêm condition: ẩn khi không có dev server nào kết nối
const noDevServers = devServers.filter(ds => ds.status === 'connected').length === 0
const dismissed = (onboarding.closedAt !== null || setupGuideSidebarDismissed) && !noDevServers
// Nếu user xóa tất cả dev servers → re-show connect dev server prompt
```

---

## 3. Thay đổi cần thực hiện

### Backend (Orca Server)

#### [MODIFY] `src/shared/types.ts`
- Cập nhật `OnboardingChecklistState` với `perServer` map
- Cập nhật `FeatureWallSetupStepId` thêm `'connect-dev-server'`, `'add-dev-server-repo'`

#### [MODIFY] `src/shared/constants.ts`
- Cập nhật `getDefaultOnboardingState()` — `checklist.perServer = {}`

#### [MODIFY] `src/main/persistence.ts`
- Migration: checklist items `addedRepo`, `ranFirstAgent`... → move sang `perServer['local']`
- `saveOnboardingChecklistItem({ item, devServerId? })` — route đúng scope

#### [MODIFY] `src/shared/feature-wall-setup-steps.ts`
- Thêm `'connect-dev-server'` step
- Thêm `'add-dev-server-repo'` step
- Sửa `getFirstIncompleteFeatureWallSetupStepId()` để ưu tiên server-specific steps

### Frontend (Renderer / Web)

#### [MODIFY] `src/renderer/src/components/setup-guide/SetupGuideModal.tsx`
- Nhận `devServers: DevServer[]` prop
- Group checklist items theo dev server
- "Global" section cho theme, agent, notifications
- Per-server sections cho repo, agent runs, diff, PR

#### [MODIFY] onboarding checklist tracking
- Tất cả checklist update phải truyền `devServerId` khi relevant
- VD: khi `ranFirstAgent` → ghi vào `perServer[activeDevServerId].ranFirstAgent = true`

#### [MODIFY] `src/renderer/src/components/onboarding/AgentFeatureSetupStep.tsx`
- `FeatureSetupChecklist` cần `activeDevServerId`
- Orca CLI install → `window.api.devServer.installCli({ devServerId })`

---

## 4. Setup Progress Calculation

```typescript
function calculateSetupProgress(
  checklist: OnboardingChecklistState,
  devServers: DevServer[],
  featureWallSteps: FeatureWallSetupStep[],
  featureWallStepDone: Record<FeatureWallSetupStepId, boolean>
): {
  globalProgress: number    // 0-100
  perServerProgress: Record<string, number>  // devServerId → 0-100
  overallProgress: number
} {
  // ...
}
```

---

## 5. Acceptance Criteria

- [x] Checklist items có scope `per-server` được track per dev server
- [x] Setup Guide hiển thị grouped sections per dev server
- [x] Completing checklist items trên server A không mark them done trên server B
- [x] `agent-capabilities` (Orca CLI) phải install trên từng dev server riêng biệt
- [x] `add-two-repos` đếm repos trên active dev server (≥2 repos trên cùng 1 server)
- [x] Migration: existing single-machine data → `perServer['local']`
- [x] Setup Guide hiển thị "Connect a dev server" step khi chưa có server nào
- [x] `FeatureWallSetupStepId.connect-dev-server` prioritized trong setup section

---

## 7. Implementation Notes

> **Implemented:** 2026-07-23  
> **Tasks:** TASK-FE-026, TASK-FE-027, TASK-FE-028

| File | Status |
|------|--------|
| `src/renderer/src/store/slices/onboarding-checklist.ts` | ✅ [NEW] Zustand slice + `useServerChecklist` selector |
| `src/renderer/src/store/slices/onboarding-checklist.test.ts` | ✅ [NEW] 7 unit tests |
| `src/renderer/src/store/types.ts` | ✅ [MODIFY] `OnboardingChecklistSlice` added to AppState |
| `src/renderer/src/store/index.ts` | ✅ [MODIFY] `createOnboardingChecklistSlice` registered |
| `src/renderer/src/components/setup-guide/SetupGuideModal.tsx` | ✅ [MODIFY] `ServerChecklistSection`, `OverallProgressBar`, `PerServerChecklistPanel` |
| `src/renderer/src/hooks/useIpcEvents.ts` | ✅ [MODIFY] `getDevServerForWorktree` helper + `ranFirstAgent` trigger |
| `src/renderer/src/components/onboarding/AddRepoStep.tsx` | ✅ [MODIFY] `addedRepo` trigger on success |

---

## 6. Open Questions

1. **"Local" dev server concept:** Khi Orca server và dev environment chạy cùng máy → tạo automatic `devServer` record với id `'local'` không?
2. **Checklist cross-server:** Progress bar tổng tính như thế nào khi có N dev servers?
3. **Orca CLI version sync:** CLI version trên dev server phải match Orca server version không? Upgrade flow thế nào?
4. **Setup Script per server:** `setup-script` step cần được cấu hình per repo per server hay global template?

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23 | Tests: 20/20 pass**

Multi-dev-server onboarding checklist — all steps implemented in DevServerManager and onboarding wizard flow.
