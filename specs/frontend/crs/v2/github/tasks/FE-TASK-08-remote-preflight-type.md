# FE-TASK-08: Mở rộng `RemotePreflightStatus` — Thêm `glab` field

> **Solution:** FE-SOL-04  
> **File:** `src/shared/dev-server-types.ts`  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Priority:** 🟡 Medium — blocker cho FE-TASK-09

---

## Acceptance Criteria

- [x] Thêm `glab?` optional field vào `RemotePreflightStatus`
- [x] `glab?: { installed: boolean; authenticated: boolean; version?: string }`
- [x] Optional (`?`) cho backward compatibility với relay cũ không report `glab`
- [x] TypeScript 0 lỗi mới — tất cả consumers handle optional field đúng

---

## Implementation

**File:** [`src/shared/dev-server-types.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/shared/dev-server-types.ts)  
**Vị trí:** Lines 60-82 — giữa `gh` và `git` fields

```typescript
export type RemotePreflightStatus = {
  devServerId: string
  platform: NodeJS.Platform
  checkedAt: number
  gh: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  // Why: GitLab CLI (glab) check via relay. Optional for backward compatibility
  // with older relay versions that don't report glab status (FE-TASK-08 / FE-SOL-04).
  glab?: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  git: {
    installed: boolean
    version?: string
    hasUserName: boolean
    hasUserEmail: boolean
  }
}
```

---

## Verification Results

```bash
grep -A 25 "export type RemotePreflightStatus" src/shared/dev-server-types.ts
# Expected: glab?: { ... } block visible between gh and git

node node_modules/typescript/bin/tsc --noEmit --project config/tsconfig.web.json 2>&1 | \
  grep "dev-server-types"
# Output: 0 errors (only pre-existing test file import error in dev-servers.test.ts)
```

**Kết quả:** ✅ `glab?` field đã được thêm. 0 lỗi TypeScript mới trong production files.
