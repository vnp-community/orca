# FE-TASK-09: `mergePreflightStatuses` — Ưu tiên Relay Status cho CLI Cards

> **Solution:** FE-SOL-04  
> **File:** `src/renderer/src/components/settings/source-control-preflight-card-status.ts`  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Depends on:** FE-TASK-08  
> **Priority:** 🟡 Medium

---

## Acceptance Criteria

- [x] Import `PreflightStatus` từ `api-types`, `RemotePreflightStatus` từ `dev-server-types`
- [x] `mergePreflightStatuses(local, remote)` helper function được thêm vào
  - `gh` → `remote.gh ?? local.gh` (ưu tiên Dev Server relay)
  - `glab` → `remote.glab ?? local.glab` (ưu tiên Dev Server relay)
  - `git` → `remote.git ?? local.git` (ưu tiên Dev Server relay)
  - `bitbucket`, `azureDevOps`, `gitea`, `linear`, `jira` → giữ nguyên local (Orca Server)
- [x] `usePreflightCardStatuses` đọc `activeRemotePreflightStatus` từ `useAppStore`
- [x] `effectiveStatusInput = statusInput && activeRemotePreflightStatus ? merge(...) : statusInput`
- [x] `getPreflightIntegrationStatuses(effectiveStatusInput, ...)` dùng merged status
- [x] Khi `activeRemotePreflightStatus = null` → behavior không đổi (local status)
- [x] TypeScript 0 lỗi mới

---

## Implementation

**File:** [`src/renderer/src/components/settings/source-control-preflight-card-status.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/settings/source-control-preflight-card-status.ts)

**Thêm (sau imports cũ, lines 10-33):**
```typescript
import type { PreflightStatus } from '../../../../preload/api-types'
import type { RemotePreflightStatus } from '../../../../shared/dev-server-types'

// Why: When a Dev Server is connected, CLI tools (gh, glab, git) run on the Dev Server,
// not on the Orca Server container. This helper merges the relay result into the local
// preflight status so CLI integration cards reflect the Dev Server state while token
// integrations (bitbucket, azureDevOps, etc.) keep reading from Orca Server.
function mergePreflightStatuses(
  local: PreflightStatus | null,
  remote: RemotePreflightStatus
): PreflightStatus | null {
  if (!local) return local
  return {
    ...local,
    // Category A — CLI: ưu tiên relay (Dev Server)
    gh: remote.gh ?? local.gh,
    glab: remote.glab ?? local.glab,
    git: remote.git ?? local.git,
    // Category B/C — Token APIs: giữ nguyên từ Orca Server local status
  } as PreflightStatus
}
```

**Trong `usePreflightCardStatuses` (lines ~79-103):**
```typescript
const activeRemotePreflightStatus = useAppStore((s) => s.activeRemotePreflightStatus)

const effectiveStatusInput = statusInput && activeRemotePreflightStatus
  ? mergePreflightStatuses(statusInput, activeRemotePreflightStatus)
  : statusInput

return {
  statuses: getPreflightIntegrationStatuses(effectiveStatusInput, refreshingProviders),
  unavailable,
  refresh
}
```

---

## Verification Results

```bash
grep -n "activeRemotePreflightStatus\|mergePreflightStatuses\|effectiveStatusInput" \
  src/renderer/src/components/settings/source-control-preflight-card-status.ts
# Expected: 5+ lines

node node_modules/typescript/bin/tsc --noEmit --project config/tsconfig.web.json 2>&1 | \
  grep "source-control-preflight"
# Output: (empty — 0 errors)
```

**Kết quả:** ✅ `mergePreflightStatuses` helper + `effectiveStatusInput` logic đã được thêm. 0 lỗi TypeScript mới.
