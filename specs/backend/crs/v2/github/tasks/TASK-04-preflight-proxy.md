# TASK-04: Sửa `preflight.check` RPC — Proxy sang Dev Server relay

**Status:** ✅ DONE — 2026-07-25 (AC verified 2026-07-25)  
**Phase:** 3 — Orca Server Proxy  
**Priority:** 🔴 Critical  
**Depends on:** TASK-01 (RpcContext), TASK-03 (relay glab support)  
**Solution:** SOL-01-CLI-Preflight.md, SOL-05-Context-Injection.md  
**CRs:** CR-GH-001, CR-GH-003, CR-INT-001, CR-INT-005  
**Estimated effort:** ~45 phút

---

## Mục tiêu

Sửa `preflight.check` RPC method trên Orca Server để:
1. Khi có `devServerId` trong params → proxy CLI check sang SSH Relay trên Dev Server
2. Kết hợp kết quả CLI (gh, glab, git) từ relay với API integration status (Bitbucket, AzDO, Gitea) từ Orca Server

---

## Hiện trạng code

**File:** `src/main/runtime/rpc/methods/preflight.ts` (full file, 48 lines):

```typescript
export const PREFLIGHT_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'preflight.check',
    params: PreflightCheck,     // = z.object({ force: z.boolean().optional() })
    handler: async (params) => runPreflightCheck(params.force)
    // ↑ handler không nhận ctx, không thể proxy!
  }),
  ...
]
```

---

## Các bước thực thi

### Bước 1: Mở rộng `PreflightCheck` schema

```typescript
// src/main/runtime/rpc/methods/preflight.ts
const PreflightCheck = z.object({
  force: z.boolean().optional(),
  devServerId: z.string().optional()   // THÊM — từ Web mode UI
})
```

### Bước 2: Sửa handler `preflight.check`

```typescript
defineMethod({
  name: 'preflight.check',
  params: PreflightCheck,
  handler: async (params, ctx) => {    // THÊM ctx parameter
    if (params.devServerId && ctx.devServerManager) {
      // Web mode: proxy CLI check sang relay trên Dev Server
      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        throw new Error(`Dev server '${params.devServerId}' relay not connected`)
      }

      // 1. CLI status từ relay (gh, glab, git trên Dev Server)
      const cliStatus = await relay.call<{
        platform: NodeJS.Platform
        gh: { installed: boolean; authenticated: boolean; version?: string }
        glab: { installed: boolean; authenticated: boolean; version?: string }
        git: { installed: boolean; version?: string; hasUserName: boolean; hasUserEmail: boolean }
      }>('preflight.check', undefined, 30_000)

      // 2. API integration status từ Orca Server (sử dụng userId per-user credential)
      const [bitbucket, azureDevOps, gitea] = await Promise.all([
        getBitbucketAuthStatus(),
        getAzureDevOpsAuthStatus(),
        getGiteaAuthStatus()
      ])

      return {
        ...cliStatus,
        bitbucket,
        azureDevOps,
        gitea
      }
    }

    // Local mode fallback (Electron hoặc không có devServerId)
    return runPreflightCheck(params.force)
  }
})
```

### Bước 3: Thêm imports cần thiết

```typescript
// Thêm imports vào đầu file preflight.ts:
import {
  getBitbucketAuthStatus
} from '../../../bitbucket/client'
import {
  getAzureDevOpsAuthStatus
} from '../../../azure-devops/client'
import {
  getGiteaAuthStatus
} from '../../../gitea/client'
```

### Bước 4: Xác nhận `relay.call` signature

Cần tìm DevServerManager API để biết cách gọi relay:

```bash
grep -rn "getRelay\|relay\.call" /Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/dev-server/ --include="*.ts" | head -20
```

Nếu API khác, điều chỉnh code cho phù hợp với interface thực tế.

---

## Tests cần thêm/sửa

**File:** `src/main/runtime/rpc/methods/preflight.test.ts` (nếu có):

```typescript
describe('preflight.check', () => {
  describe('with devServerId', () => {
    it('calls relay.call("preflight.check") khi devServerId được cung cấp')
    it('merges relay CLI status với local API integration status')
    it('throws khi relay không connect')
  })
  describe('without devServerId', () => {
    it('gọi runPreflightCheck() (existing behavior)')
  })
})
```

---

## Acceptance Criteria

1. ✅ `preflight.check({ devServerId: "ds-abc" })` → relay nhận `preflight.check` call
2. ✅ Response chứa `gh.installed`, `glab.installed`, `git.installed` từ Dev Server
3. ✅ Response chứa `bitbucket.configured`, `azureDevOps.configured`, `gitea.configured` từ Orca Server (merged from relay result)
4. ✅ `preflight.check({})` (không có devServerId) → behavior cũ, không break existing tests

---

## Files cần sửa

- `src/main/runtime/rpc/methods/preflight.ts` — schema + handler
