# Conflict Analysis — Backend × Agent × Frontend Solutions
**Verified against codebase: 2026-07-30T17:48**  
**Resolution Strategy Updated: 2026-07-30T18:32** — "New File + Compile-time Flag"

---

## Chiến lược giải quyết conflict (UPDATED)

> **Nguyên tắc cốt lõi:** Không chỉnh sửa file hiện tại — tạo file mới, chọn lúc compile.
>
> | Cách tiếp cận | Mô tả |
> |--------------|-------|
> | **New File** | Tạo file `*-v6.ts` / `*-v6.tsx` mới — không đụng file hiện tại |
> | **Compile-time Flag** | `__ORCA_GIT_V6__`, `__ORCA_WORKSPACE_V6__` via `electron.vite.config.ts` `define` block |
> | **Selector File** | Barrel file chọn đúng implementation dựa vào compile-time constant |
>
> **Pattern đã có trong codebase:** `ORCA_BUILD_IDENTITY`, `ORCA_POSTHOG_WRITE_KEY`  
> → Khai báo trong `src/types/build-constants.d.ts`, inject trong `electron.vite.config.ts`

---

## Kết quả phân tích

| Loại | Số lượng |
|------|--------|
| ⚠️ HIGH Conflict | 3 |
| ⚡ MEDIUM Conflict | 3 |
| 🆕 New Finding (phát hiện khi verify) | 2 |
| ✅ False Alarm (đóng lại) | 3 |

---

## ⚠️ HIGH — C1: `src/relay/git-remote-handler.ts` bị duplicate

**Vấn đề:** Backend Task T15 đặc tả tạo `git-remote-handler.ts` hoặc extend `git-handler.ts` với `git.exec` + `git.execStream`.

**Thực tế verify:** `src/relay/git-remote-handler.ts` **ĐÃ TỒN TẠI** (93 lines, tạo 2026-07-29) với đầy đủ:
```typescript
export const gitRemoteHandlers = {
  'git.exec': async (params) => { ... },       // line 63 — ĐÃ CÓ
  'git.execStream': async (params) => { ... }, // line 87 — ĐÃ CÓ
}
export function validateGitArgs(args)           // security validation — ĐÃ CÓ
```

**Conflict:** Nếu T15 thêm code mới → **DUPLICATE logic**. Agent CR-AG-10 cũng add git.pr.create vào `agent-git-handler.ts` (line 244 confirmed).

**Resolution — New File + Compile-time Flag:**
```
KHÔNG chỉnh git-remote-handler.ts (93 lines) — GIỮ NGUYÊN

TẠO MỚI: src/relay/git-remote-handler-v6.ts       ← +9 high-level methods
TẠO MỚI: src/relay/git-remote-handler-index.ts    ← compile-time selector
```
```typescript
// git-remote-handler-index.ts
declare const __ORCA_GIT_V6__: boolean
export * from __ORCA_GIT_V6__
  ? './git-remote-handler-v6'   // v6: git.status, git.diff, git.add, ...
  : './git-remote-handler'      // v5: chỉ git.exec, git.execStream
```
**Phân công:**
```
git-remote-handler.ts         → Backend owns (UNCHANGED — v5 baseline)
git-remote-handler-v6.ts      → Backend owns (NEW — v6 extensions)
git-remote-handler-index.ts   → Backend owns (NEW — compile selector)
agent-git-handler.ts          → Agent owns  (UNCHANGED — git.pr.create line 244)
```

---

## ⚠️ HIGH — C2: `src/renderer/src/context/WorkspaceContext.tsx` bị overwrite

**Vấn đề:** Backend Task T17 spec tạo mới `WorkspaceContext.tsx`.

**Thực tế verify:**
```
src/renderer/src/context/WorkspaceContext.tsx — 180 LINES (EXISTING)
  Line 25: fileTree: FileNode | null    ← TYPE SAI (phải là array)
  Line 55: useState<FileNode | null>    ← cần sửa
  Missing: currentWorktree, availableWorktrees, pendingTasks
```

**Conflict:** T17 apply → **OVERWRITE** 180 lines code hiện tại → mất implementation hiện có.

**Resolution — New File + Compile-time Flag:**
```
KHÔNG chỉnh WorkspaceContext.tsx (185 lines) — GIỮ NGUYÊN

TẠO MỚI: src/renderer/src/context/WorkspaceContextV6.tsx       ← full v6 spec
TẠO MỚI: src/renderer/src/context/WorkspaceContextBridge.ts   ← compile-time selector
```
```typescript
// WorkspaceContextBridge.ts
declare const __ORCA_WORKSPACE_V6__: boolean
export * from __ORCA_WORKSPACE_V6__
  ? './WorkspaceContextV6'   // v6: switchProject, event bus, pendingTasks, FileTreeNode[]
  : './WorkspaceContext'     // v5: existing 185-line implementation

// App.tsx — import từ Bridge thay vì trực tiếp:
import { WorkspaceProvider, useWorkspace } from './context/WorkspaceContextBridge'
```
> ✅ v5 và v6 cùng tồn tại — rollback = chỉ đổi env var khi build.
> `❌ HỦY Task T17 backend` — thay bằng task tạo `WorkspaceContextV6.tsx` [NEW].

---

## ⚠️ HIGH — C3: `src/relay/agent-rpc-dispatch.ts` — 4 CRs concurrent modify

**Vấn đề:** 4 Agent CRs cùng modify `agent-rpc-dispatch.ts`:

| CR | Cases thêm |
|----|-----------|
| CR-AG-09 | `ai.provider.deleteCredential` |
| CR-AG-10 | `git.pr.create`, `git.worktree.list`, `git.worktree.add`, `git.worktree.remove` |
| CR-AG-11 | `fs.stat`, `fs.glob`, `fs.writeFile` |
| CR-AG-12 | `agent.spawn`, `agent.kill` |

**Tổng: 9 case blocks mới** — nếu apply theo thứ tự sai hoặc riêng lẻ → merge conflict hoặc partial patch.

**Trạng thái thực tế:**
> ✅ **TASK-07 đã DONE (2026-07-30T17:52)** — tất cả 13 cases đã được thêm vào `agent-rpc-dispatch.ts`.  
> Confirmed: `grep -n "git.pr.create" src/relay/agent-rpc-dispatch.ts` → line 237.

**Resolution — C3 CLOSED:**
> TASK-07 đã áp dụng consolidated patch đúng thứ tự CR-AG-09 → CR-AG-10 → CR-AG-11 → CR-AG-12.  
> **Không cần thêm action nào.**
>
> **Nếu trong tương lai cần thêm cases mới (không sửa dispatch.ts):**
> ```
> TẠO MỚI: src/relay/agent-rpc-dispatch-extensions.ts
> // Wrap pattern — không chỉnh file gốc:
> export function mountExtensions(dispatch: DispatchFn): DispatchFn {
>   return async (rpc, ctx) => {
>     const result = await extensionHandlers(rpc, ctx)
>     if (result) return result
>     return dispatch(rpc, ctx)  // fallback to original
>   }
> }
> ```

---

## ⚡ MEDIUM — C4: `ai-provider-handler.ts` vs `agent-credential-store.ts` — Duplicated Logic

**Verified:**
```
src/relay/agent-credential-store.ts  — 11,584 bytes (LỚN HƠN — owns logic, Dev Server tier)
src/relay/ai-provider-handler.ts     —  3,366 bytes (NHỎ HƠN — thin layer, Orca Server tier)
  → import from node:fs/promises trực tiếp (KHÔNG delegate sang agent-credential-store)
  → INDEPENDENT implementation, KHÔNG phải wrapper
```

**Vấn đề:** Cả 2 file có `healthCheck` logic riêng. Agent CR-AG-09 upgrade healthCheck trong `agent-credential-store.ts` → **`ai-provider-handler.ts` vẫn dùng logic cũ**.

**Context kiến trúc:**
- `agent-credential-store.ts` → Agent tier (chạy trên Dev Server)
- `ai-provider-handler.ts` → Server/Relay tier (chạy trên Orca Server)

Đây là **intentional separation** theo kiến trúc 2-tier. Không cần merge.

**Resolution — New File (shared contract):**
```
KHÔNG merge 2 files (2-tier là intentional).

TẠO MỚI: src/shared/ai-credential-contract.ts  ← interface dùng chung
  export interface CredentialReadResult { encryptedBlob: string; iv: string; updatedAt: string }
  export interface HealthCheckResult { ok: boolean; latencyMs: number; error?: string }

Cả 2 files import types từ contract mới:
  agent-credential-store.ts → implements HealthCheckResult
  ai-provider-handler.ts    → implements HealthCheckResult (same shape)
```
> ✅ API contract đồng bộ qua shared types — không cần merge implementation.

---

## ⚡ MEDIUM — C5: `git.pr.create` — RPC Contract trùng method name, khác schema

**Verified:**
```
src/relay/agent-git-handler.ts (line 244-289):
  git.pr.create → params: { cwd, title, body?, base?, head? }
  → calls gh CLI trực tiếp

src/relay/git-remote-handler.ts:
  KHÔNG có git.pr.create ← confirmed (grep tìm không thấy)

Backend Task T15:
  git.pr.create → params: { projectId, title, body?, base?, head? }
  → routes qua ProjectServerRouter → relay bridge → agent
```

**Flow đúng:**
```
Frontend: useGit().createPR({ projectId, title })
    ↓
Backend: git.pr.create({ projectId })
    ↓ lookup relay
Agent:  git.pr.create({ cwd: '/project/path', title })
    ↓
gh CLI: gh pr create --title ...
```

**Không conflict** — 2-tier routing pattern là đúng.

**Resolution — New File:**
```
KHÔNG thêm git.pr.create vào git-remote-handler.ts (file cũ — GIỮ NGUYÊN)

TẠO MỚI: src/relay/git-remote-handler-v6.ts
  → có 'git.pr.create' làm routing-only method (proxy xuống agent)
  → import qua git-remote-handler-index.ts (compile selector)
  → KHÔNG re-implement gh CLI — agent-git-handler.ts (line 244) đã có

TẠO MỚI: src/main/runtime/rpc/methods/git-remote-rpc.ts
  → Backend tier: nhận projectId → resolve relay → gọi relay.call('git.pr.create')
```

---

## ⚡ MEDIUM — C6: Duplicate Type Definitions (renderer vs shared)

**Verified — 3 file pairs tồn tại song song:**
```
src/renderer/src/types/ai-provider-types.ts  ← Frontend copy (TDD-FE-13)
src/shared/ai-provider-types.ts              ← Backend copy (TDD-16)

src/renderer/src/types/profile-types.ts      ← Frontend copy
src/renderer/src/types/task-types.ts         ← Frontend copy
```
**Vấn đề:** Frontend có HEADER `// Shared types for AI Provider (TDD-FE-13)` → tự định nghĩa lại thay vì import từ `src/shared/`.

**Rủi ro:** Types diverge over time → RPC type mismatch.

**Resolution — New File (không sửa file cũ):**
```
KHÔNG chỉnh renderer/types/ai-provider-types.ts — GIỮ NGUYÊN

TẠO MỚI: src/renderer/src/types/ai-provider-types-shared.ts
  // Re-export tất cả từ src/shared/ + renderer-specific additions
  export type { AIProviderType, AIProviderAccount, AIProviderStatus, AIProviderScope }
    from '../../../shared/ai-provider-types'

Components mới → import từ ai-provider-types-shared.ts
Components cũ  → vẫn dùng ai-provider-types.ts (không đụng)
```

---

## 🆕 NEW FINDING A: `ProfileAwareAgentSpawner` — 2 tier, không conflict

**Verified:**
```
src/main/project/ProfileAwareAgentSpawner.ts  ← Orca Server tier (4.6KB, EXISTS)
src/relay/agent-spawner.ts                     ← KHÔNG TỒN TẠI (Agent CR-AG-12 sẽ tạo mới)
```

Agent CR-AG-12 tạo `agent-spawner.ts` cho Dev Server tier — **khác hoàn toàn** về vai trò.

**Resolution:**
> Naming giống nhau gây confusion. Recommend: CR-AG-12 export class `SubAgentSpawner` thay vì `ProfileAwareAgentSpawner`.

---

## 🆕 NEW FINDING B: `git-remote-handler.ts` thiếu phần lớn git commands

**Verified:** `git-remote-handler.ts` (93 lines) chỉ có `git.exec` và `git.execStream`.  
Backend Task T15/T16 và Frontend SOL-FE-V6-006 cần: `git.status`, `git.diff`, `git.add`, `git.commit`, `git.push`, `git.pull`, `git.branch.list`, `git.checkout`, `git.generateCommitMessage`, `git.pr.create` (routing).

**Resolution — New File (gom vào C1 resolution):**
```
KHÔNG chỉnh git-remote-handler.ts (93 lines) — GIỮ NGUYÊN

TẠO MỚI: src/relay/git-remote-handler-v6.ts (~250 lines)
  → Sử dụng gitRemoteHandlers['git.exec'] (export từ file cũ)
  → Được chọn qua compile-time flag __ORCA_GIT_V6__
```

---

## ✅ FALSE ALARM — Conflicts không thực sự tồn tại

| # | Conflict Mô tả | Thực tế |
|---|---------------|--------|
| FA-1 | git.exec duplicate | `git-remote-handler.ts` đã có từ 07-29, không conflict |
| FA-2 | ProfileAwareAgentSpawner naming | 2 tier hoàn toàn khác nhau, không overlap |
| FA-3 | useWorkspace.ts | Re-export pattern ổn, không cần action |

---

## Compile-time Feature Flags — Thiết kế chi tiết

### Bước 1: Khai báo (`src/types/build-constants.d.ts`) — EXTEND file hiện tại

```typescript
// Thêm vào cuối file (không xóa gì):
declare const __ORCA_GIT_V6__: boolean       // true = dùng git-remote-handler-v6
declare const __ORCA_WORKSPACE_V6__: boolean  // true = dùng WorkspaceContextV6
```

### Bước 2: Inject trong `electron.vite.config.ts` — EXTEND define block

```typescript
// Thêm vào define block hiện tại (sau ORCA_DIAGNOSTICS_TOKEN_URL):
__ORCA_GIT_V6__: JSON.stringify(process.env.ORCA_FEATURE_GIT_V6 === 'true'),
__ORCA_WORKSPACE_V6__: JSON.stringify(process.env.ORCA_FEATURE_WORKSPACE_V6 === 'true'),
```

### Bước 3: Selector files [NEW]

```typescript
// src/relay/git-remote-handler-index.ts
declare const __ORCA_GIT_V6__: boolean
export * from __ORCA_GIT_V6__
  ? './git-remote-handler-v6'
  : './git-remote-handler'
```

```typescript
// src/renderer/src/context/WorkspaceContextBridge.ts
declare const __ORCA_WORKSPACE_V6__: boolean
export * from __ORCA_WORKSPACE_V6__
  ? './WorkspaceContextV6'
  : './WorkspaceContext'
```

### Bước 4: Cách dùng (command line)

```bash
# Dev v5 baseline (mặc định — không có gì mới):
pnpm dev

# Dev v6 features enabled:
ORCA_FEATURE_GIT_V6=true ORCA_FEATURE_WORKSPACE_V6=true pnpm dev

# Build release v6:
ORCA_FEATURE_GIT_V6=true ORCA_FEATURE_WORKSPACE_V6=true pnpm build
```

---

## Consolidated Action Plan (UPDATED — New File Strategy)

### 🔴 Phải làm TRƯỚC khi implement bất kỳ task nào

```
1. EXTEND build system (không xóa gì hiện có):
   - src/types/build-constants.d.ts: +2 dòng declare
   - electron.vite.config.ts define block: +2 entries
   - vite.server.config.ts define block: +2 entries (nếu cần cho relay/main tier)

2. Tạo selector files [NEW]:
   - src/relay/git-remote-handler-index.ts
   - src/renderer/src/context/WorkspaceContextBridge.ts

3. Quy tắc bất biến:
   ❌ git-remote-handler.ts (93 lines)    — GIỮ NGUYÊN
   ❌ WorkspaceContext.tsx (185 lines)    — GIỮ NGUYÊN
   ❌ agent-rpc-dispatch.ts (443 lines)   — GIỮ NGUYÊN (TASK-07 đã DONE)
```

### 🟡 Tạo file mới (implementation)

```
4. src/relay/git-remote-handler-v6.ts  [NEW]
   → git.status, git.diff, git.add, git.restore, git.commit
   → git.push, git.pull, git.branch.list, git.checkout
   → git.pr.create (proxy xuống agent — KHÔNG re-implement)

5. src/main/runtime/rpc/methods/git-remote-rpc.ts  [NEW]
   → Backend RPC routing layer (gọi relay.call() xuống Dev Server)

6. src/renderer/src/context/WorkspaceContextV6.tsx  [NEW]
   → Full v6: switchProject, event bus, pendingTasks
   → fileTree: FileTreeNode[] (type đúng)
   → currentWorktree, availableWorktrees

7. src/shared/ai-credential-contract.ts  [NEW]
   → Shared interface: CredentialReadResult, HealthCheckResult
   → Đồng bộ agent-credential-store.ts vs ai-provider-handler.ts

8. src/renderer/src/types/ai-provider-types-shared.ts  [NEW]
   → Re-export từ src/shared/ai-provider-types.ts
   → Components mới import từ đây
```

### 🟢 Verify sau implement

```bash
# v5 baseline (không flags):
pnpm tsc --noEmit
pnpm vitest run src/relay

# v6 features:
ORCA_FEATURE_GIT_V6=true ORCA_FEATURE_WORKSPACE_V6=true pnpm tsc --noEmit

# Kiểm tra file cũ không bị chỉnh:
git diff src/relay/git-remote-handler.ts       # should be empty (no changes)
git diff src/renderer/src/context/WorkspaceContext.tsx  # should be empty
git diff src/relay/agent-rpc-dispatch.ts       # should be empty
```

---

## Summary Matrix (UPDATED — New File + Compile-time Flag)

| Conflict | File(s) | Hệ thống | Severity | Resolution |
|----------|---------|---------|----------|-----------|
| C1 | `git-remote-handler.ts` | Backend T15 | ⚠️ HIGH | **NEW** `git-remote-handler-v6.ts` + `git-remote-handler-index.ts` (__ORCA_GIT_V6__) |
| C2 | `WorkspaceContext.tsx` | Backend T17 × Frontend | ⚠️ HIGH | **NEW** `WorkspaceContextV6.tsx` + `WorkspaceContextBridge.ts` (__ORCA_WORKSPACE_V6__) |
| C3 | `agent-rpc-dispatch.ts` | Agent CR-AG-09/10/11/12 | ⚠️ HIGH | ✅ **CLOSED** — TASK-07 DONE 2026-07-30T17:52 |
| C4 | `agent-credential-store.ts` vs `ai-provider-handler.ts` | Agent × Backend | ⚡ MED | **NEW** `ai-credential-contract.ts` shared interface |
| C5 | `git.pr.create` RPC schema | Backend × Agent | ⚡ MED | **NEW** `git-remote-handler-v6.ts` (routing-only, agent owns impl) |
| C6 | `renderer/types/` vs `src/shared/` | Backend × Frontend | ⚡ MED | **NEW** `ai-provider-types-shared.ts` (re-export, file cũ giữ nguyên) |
| NEW-A | `ProfileAwareAgentSpawner` naming | Backend × Agent | ℹ️ LOW | **NEW** `agent-spawner.ts` export `SubAgentSpawner` (tên khác, 2 tier khác) |
| NEW-B | `git-remote-handler.ts` thiếu commands | Backend | ℹ️ INFO | **NEW** `git-remote-handler-v6.ts` (~250 lines, chọn qua compile flag) |
| FA-1/2/3 | (3 false alarms đã đóng) | — | ✅ CLOSED | Không cần action |

---

## File Ownership Map (sau khi giải quyết conflicts)

```
[UNCHANGED — GIỮ NGUYÊN]
src/relay/git-remote-handler.ts          → Backend owns (v5 baseline, 93 lines)
src/relay/agent-git-handler.ts           → Agent owns  (git.pr.create line 244)
src/relay/agent-rpc-dispatch.ts          → Agent owns  (TASK-07 DONE, 443 lines)
src/relay/agent-credential-store.ts      → Agent owns  (deleteCredential done)
src/relay/ai-provider-handler.ts         → Backend owns (2-tier intentional)
src/renderer/src/context/WorkspaceContext.tsx  → Frontend owns (v5, 185 lines)
src/renderer/src/types/ai-provider-types.ts   → Frontend owns (v5, giữ nguyên)
src/shared/ai-provider-types.ts          → Backend owns (source of truth)

[NEW FILES]
src/relay/git-remote-handler-v6.ts       → Backend owns (NEW — v6 extensions)
src/relay/git-remote-handler-index.ts    → Backend owns (NEW — compile selector)
src/main/runtime/rpc/methods/git-remote-rpc.ts → Backend owns (NEW — routing layer)
src/relay/agent-spawner.ts               → Agent owns  (NEW — SubAgentSpawner)
src/renderer/src/context/WorkspaceContextV6.tsx   → Frontend owns (NEW — v6 spec)
src/renderer/src/context/WorkspaceContextBridge.ts → Frontend owns (NEW — compile selector)
src/shared/ai-credential-contract.ts     → Backend owns (NEW — shared interface C4)
src/renderer/src/types/ai-provider-types-shared.ts → Frontend owns (NEW — re-export)

[EXTEND (2 dòng mửi mỗi file)]
src/types/build-constants.d.ts           → Global (EXTEND: +2 flag declarations)
electron.vite.config.ts                  → Global (EXTEND: +2 define entries)
```
