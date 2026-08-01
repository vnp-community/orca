# CR-INT-005: `preflight.check` — Full Extension cho tất cả integrations (Web mode)

**ID:** CR-INT-005  
**Priority:** 🟡 Medium  
**Component:** `src/main/runtime/rpc/methods/preflight.ts`, relay binary  
**Depends on:** CR-GH-001, CR-GH-005, CR-INT-001, CR-INT-002  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-01-CLI-Preflight, SOL-04-Credential-Store, FE-SOL-04  
**Tasks:** TASK-01 (backend preflight), FE-TASK-08, FE-TASK-09, FE-TASK-10

## Acceptance Criteria — Verified

1. ✅ `preflight.check` với `devServerId` → git/gh/glab check qua relay → Dev Server — methods/preflight.ts L32-40
2. ✅ `preflight.check` với token trong WebCredentialStore → bitbucket/azDO/gitea check từ Orca Server — credentials.ts
3. ✅ `preflight.check` không có `devServerId` → backward compatible (local check) — params.devServerId optional (L16)
4. ✅ `preflightStatusError = null` khi tất cả checks thành công
5. ✅ Integration cards hiển thị đúng status — mergePreflightStatuses() ưu tiên relay cho CLI, local cho API
6. ⧞ Tests cover 4 scenarios — deferred (unit tests chưa được implement)

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | `methods/preflight.ts` | devServerId optional → relay proxy (L11-40) |
| Backend | `methods/credentials.ts` | Bitbucket/AzDO/Gitea status via WebCredentialStore |
| Shared | `dev-server-types.ts` | RemotePreflightStatus.glab? added (FE-TASK-08) |
| Frontend | `slices/preflight.ts` | setRemotePreflightStatus() in .then() (FE-TASK-10) |
| Frontend | `source-control-preflight-card-status.ts` | mergePreflightStatuses() (FE-TASK-09) |


---

## Vấn đề

`preflight.check` hiện tại trả về status cho: `git`, `gh`, `glab`, `bitbucket`, `azureDevOps`, `gitea`.

Trong Web + Dev Server mode, mỗi integration cần được check từ đúng nguồn:

| Integration | Check location | Check method | Current (Web mode) |
|------------|---------------|-------------|-------------------|
| `git` | Dev Server | `git --version` via relay | ❌ Orca Server |
| `gh` | Dev Server | `gh auth status` via relay | ❌ Orca Server container |
| `glab` | Dev Server | `glab auth status` via relay | ❌ Orca Server container |
| `bitbucket` | Orca Server | HTTP API call (token từ store) | ❌ global env var |
| `azureDevOps` | Orca Server | HTTP API call (token từ store) | ❌ global env var |
| `gitea` | Orca Server | HTTP API call (token từ store) | ❌ global env var |

---

## Proposed Full `preflight.check` Flow (Web + Dev Server mode)

```typescript
// src/main/runtime/rpc/methods/preflight.ts
defineMethod({
  name: 'preflight.check',
  params: z.object({
    force: z.boolean().optional(),
    devServerId: z.string().optional(),
  }),
  handler: async (params, context) => {
    const { devServerId } = params
    const { userId, sessionId, devServerManager } = context
    
    // ── Part 1: CLI integrations (Category A) → proxy to Dev Server ──
    let cliStatus: Pick<PreflightStatus, 'git' | 'gh' | 'glab'> | null = null
    
    if (devServerId) {
      const relay = devServerManager.getRelay(devServerId)
      if (!relay) throw new Error(`Dev server '${devServerId}' not connected`)
      
      const ghConfigDir = sessionId ? `/tmp/orca-sessions/${sessionId}/gh` : undefined
      const glabConfigDir = sessionId ? `/tmp/orca-sessions/${sessionId}/glab` : undefined
      
      cliStatus = await relay.call<Pick<PreflightStatus, 'git' | 'gh' | 'glab'>>(
        'preflight.check.cli',   // ← New relay-side endpoint (CLI-only)
        {
          force: params.force,
          env: {
            ...(ghConfigDir ? { GH_CONFIG_DIR: ghConfigDir } : {}),
            ...(glabConfigDir ? { GLAB_CONFIG_DIR: glabConfigDir } : {})
          }
        },
        30_000
      )
    } else {
      // Fallback: check locally on Orca Server
      const [gitProbe, ghProbe, glabProbe] = await Promise.all([
        detectCommandRuntime('git'),
        detectCommandRuntime('gh'),
        detectCommandRuntime('glab')
      ])
      cliStatus = {
        git: { installed: gitProbe.installed },
        gh: { installed: ghProbe.installed, authenticated: ... },
        glab: { installed: glabProbe.installed, authenticated: ... }
      }
    }
    
    // ── Part 2: API token integrations (Category B) → check on Orca Server ──
    // Sử dụng per-user credentials từ WebCredentialStore (CR-INT-002, CR-INT-004)
    const [bitbucket, azureDevOps, gitea] = await Promise.all([
      getBitbucketAuthStatus({ userId }),       // reads from WebCredentialStore
      getAzureDevOpsAuthStatus({ userId }),     // reads from WebCredentialStore
      getGiteaAuthStatus({ userId })            // reads from WebCredentialStore
    ])
    
    return {
      ...cliStatus,
      bitbucket,
      azureDevOps,
      gitea
    }
  }
})
```

---

## Relay-side endpoint: `preflight.check.cli`

Hiện tại, relay binary xử lý `preflight.check` gọi vào `runPreflightCheck()` — đây check toàn bộ bao gồm cả Bitbucket/AzDO/Gitea (từ env vars trên relay machine).

Cần tách thành 2 endpoints trên relay:

**`preflight.check.cli`** (mới): chỉ check git, gh, glab
```typescript
// orca-relay binary (Dev Server side)
handlers['preflight.check.cli'] = async (params) => {
  const env = { ...process.env, ...params.env }
  
  const [gitInstalled, ghInstalled, glabInstalled] = await Promise.all([
    isCommandAvailable('git', { env }),
    isCommandAvailable('gh', { env }),
    isCommandAvailable('glab', { env })
  ])
  
  const [ghAuth, glabAuth] = await Promise.all([
    ghInstalled ? runCommandCatch('gh', ['auth', 'status'], { env }) : false,
    glabInstalled ? runCommandCatch('glab', ['auth', 'status'], { env }) : false
  ])
  
  return {
    git: { installed: gitInstalled },
    gh: { installed: ghInstalled, authenticated: ghAuth },
    glab: { installed: glabInstalled, authenticated: glabAuth }
  }
}
```

**`preflight.check`** (giữ nguyên, backward compatible): check toàn bộ (cho Electron mode)

---

## Extended `PreflightStatus` type

```typescript
// src/shared/types.ts (hoặc src/shared/preflight-types.ts)
export type PreflightStatus = {
  git: { installed: boolean; version?: string }
  gh: { installed: boolean; authenticated: boolean; account?: string }
  glab?: { installed: boolean; authenticated: boolean; account?: string }
  bitbucket?: {
    configured: boolean     // token exists
    authenticated: boolean  // token validated via API
    account: string | null
  }
  azureDevOps?: {
    configured: boolean
    authenticated: boolean
    account: string | null
    baseUrl: string | null
    tokenConfigured: boolean
  }
  gitea?: {
    configured: boolean
    authenticated: boolean
    account: string | null
    baseUrl: string | null
    tokenConfigured: boolean
  }
  // Future: Linear, Jira status (if needed in preflight)
}
```

---

## Renderer: Gửi đúng params trong Web mode

**File:** `src/renderer/src/store/slices/preflight.ts`
```typescript
// Trong refreshPreflightStatus():
const context = getLocalPreflightContext(get())
// context = { devServerId: 'ds-abc123' } trong Web + DevServer mode

const rpcParams = {
  force: force || undefined,
  devServerId: context?.devServerId,  // [CR-INT-005] include for CLI routing
}

const request = (
  runtimeTarget.kind === 'environment'
    ? callRuntimeRpc<PreflightStatus>(runtimeTarget, 'preflight.check', rpcParams)
    : window.api.preflight.check(preflightArgs)
)
```

---

## Test coverage

**`preflight.check` integration test matrix:**

| Scenario | git | gh | glab | bitbucket | Result |
|---------|-----|-----|------|-----------|--------|
| No dev server, no tokens | ❌ | ❌ | ❌ | ❌ | All false |
| Dev server connected, `gh` installed+auth | ✅ relay | ✅ relay | - | ❌ | gh OK |
| Dev server connected, token in store | ✅ relay | ✅ relay | - | ✅ store | All OK |
| Dev server disconnected | ✅ local | ✅ local | - | ✅ store | CLI from local |

---

## Files cần thay đổi

### [MODIFY] `src/main/runtime/rpc/methods/preflight.ts`
- Split `preflight.check` thành CLI part (relay) + API part (Orca Server)
- Handler nhận `devServerId`, `userId`, `sessionId` từ context

### [MODIFY] `src/main/ipc/preflight.ts`
- `getBitbucketAuthStatus(opts?: { userId? })` — accept userId
- `getAzureDevOpsAuthStatus(opts?: { userId? })` — accept userId  
- `getGiteaAuthStatus(opts?: { userId? })` — accept userId

### [MODIFY] orca-relay binary
- Add `preflight.check.cli` endpoint (CLI-only check)
- Accept `env` override param

### [MODIFY] `src/renderer/src/store/slices/preflight.ts`
- Pass `devServerId` từ context vào RPC call

### [MODIFY] `src/main/runtime/rpc/methods/preflight.test.ts`
- Test với/không có `devServerId`
- Mock relay call cho CLI status
- Test API token status via `WebCredentialStore`

---

## Acceptance Criteria

1. `preflight.check` với `devServerId` → git/gh/glab check qua relay → Dev Server
2. `preflight.check` với `devServerId` + token trong store → bitbucket check qua HTTP từ Orca Server
3. `preflight.check` không có `devServerId` → backward compatible (local check)
4. `preflightStatusError = null` khi tất cả checks thành công
5. Integration cards hiển thị đúng status cho tất cả integrations
6. Tests cover cả 4 scenarios trong test matrix

## Related

- CR-GH-001: GitHub CLI check
- CR-GH-005: Context injection infrastructure
- CR-INT-001: GitLab CLI check
- CR-INT-002: API token integrations
- CR-INT-004: Unified credential store
