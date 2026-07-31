# FE-SOL-04: Remote Preflight Status Mapping — Full Integration Cards

> **CRs:** CR-INT-005, CR-GH-001, CR-INT-001  
> **Backend SOL tương ứng:** SOL-01-CLI-Preflight, SOL-04-Credential-Store  
> **TDD:** TDD-FE-02 (State Management), TDD-FE-09 (Onboarding)  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Tasks:** [FE-TASK-08](../tasks/FE-TASK-08-remote-preflight-type.md), [FE-TASK-09](../tasks/FE-TASK-09-preflight-card-merge.md), [FE-TASK-10](../tasks/FE-TASK-10-preflight-response-update.md)

---

## Vấn đề

Khi relay trả về preflight status `{ platform, gh, glab, git }`, frontend cần:
1. Map kết quả relay vào `remotePreflightByServer[devServerId]`
2. Các integration cards (`GitHubIntegrationCard`, `GitLabIntegrationCard`) phải đọc remote status
3. Cards cho token integrations (Bitbucket, Azure, Gitea) cần hiểu rằng chúng chạy từ Orca Server, không phải relay

---

## Phân loại Integration Status

### Category A — CLI (chạy trên Dev Server via relay)
| Integration | Status source | State từ relay |
|------------|---------------|----------------|
| GitHub | `preflightStatus.gh` | `{ installed, authenticated, version }` |
| GitLab | `preflightStatus.glab` | `{ installed, authenticated, version? }` |
| Git | `preflightStatus.git` | `{ installed, version, hasUserName, hasUserEmail }` |

### Category B+C — HTTP API (chạy từ Orca Server, không relay)
| Integration | Status source | State |
|------------|---------------|-------|
| Bitbucket | `preflightStatus.bitbucket` | `{ configured, authenticated, account }` |
| Azure DevOps | `preflightStatus.azureDevOps` | `{ configured, authenticated, account, baseUrl }` |
| Gitea | `preflightStatus.gitea` | `{ configured, authenticated, account, baseUrl }` |
| Linear | `preflightStatus.linear` | `{ configured, authenticated }` |
| Jira | `preflightStatus.jira` | `{ configured, authenticated }` |

---

## Thiết kế giải pháp

### 1. `RemotePreflightStatus` type cần đủ fields

```typescript
// src/renderer/src/store/slices/preflight.ts

export type RemotePreflightStatus = {
  platform?: NodeJS.Platform
  gh?: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  glab?: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  git?: {
    installed: boolean
    version?: string
    hasUserName?: boolean
    hasUserEmail?: boolean
  }
  // Token integrations (từ Orca Server, included trong relay response nếu merge)
  bitbucket?: {
    configured: boolean
    authenticated: boolean
    account?: string | null
    baseUrl?: string | null
    tokenConfigured?: boolean
  }
  azureDevOps?: {
    configured: boolean
    authenticated: boolean
    account?: string | null
    baseUrl?: string | null
    tokenConfigured?: boolean
  }
  gitea?: {
    configured: boolean
    authenticated: boolean
    account?: string | null
    baseUrl?: string | null
    tokenConfigured?: boolean
  }
}
```

### 2. `usePreflightCardStatuses` — đọc remote status khi có Dev Server

```typescript
// src/renderer/src/components/settings/source-control-preflight-card-status.ts [MODIFY]

export function usePreflightCardStatuses(provider: PreflightRefreshProvider): PreflightCardStatuses {
  const preflightStatus = useAppStore(s => s.preflightStatus)
  const activeRemotePreflightStatus = useAppStore(s => s.activeRemotePreflightStatus)
  const activeDevServerId = useAppStore(s => s.activeDevServerId)

  // Khi có Dev Server → ưu tiên dùng remote preflight status cho CLI integrations
  const effectiveStatus = activeDevServerId && activeRemotePreflightStatus
    ? mergePreflightStatuses(preflightStatus, activeRemotePreflightStatus)
    : preflightStatus

  // ... rest of hook logic unchanged ...
}

function mergePreflightStatuses(
  local: PreflightStatus | null,
  remote: RemotePreflightStatus
): PreflightStatus {
  // Category A (gh, glab, git): ưu tiên remote
  // Category B+C (bitbucket, azureDevOps, gitea): giữ local (Orca Server)
  return {
    ...local,
    gh: remote.gh ?? local?.gh,
    glab: remote.glab ?? local?.glab,
    git: remote.git ?? local?.git,
    // Token integrations giữ nguyên từ local
    bitbucket: local?.bitbucket,
    azureDevOps: local?.azureDevOps,
    gitea: local?.gitea,
    linear: local?.linear,
    jira: local?.jira,
  }
}
```

### 3. Integration Cards — hiển thị "Dev Server" context

Khi đang ở Web mode với Dev Server connected, cards nên hiển thị context label:

```typescript
// src/renderer/src/components/settings/cli-source-control-integration-cards.tsx [MODIFY]

// THÊM VÀO GitHubIntegrationCard:
const activeDevServer = useAppStore(s =>
  s.devServers.find(ds => ds.id === s.activeDevServerId)
)

// Trong description hoặc details section:
{activeDevServer && (
  <p className="text-xs text-muted-foreground mt-1">
    <Server className="size-3 inline mr-1" />
    Checking on Dev Server: {activeDevServer.displayName || activeDevServer.host}
  </p>
)}
```

### 4. Preflight Refresh sau khi login

Sau khi `github.startAuthLogin` PTY terminal đóng, cần trigger refresh:

```typescript
// src/renderer/src/components/settings/WebModeCliAuthSection.tsx

const handlePtyClose = () => {
  setPtyInfo(null)
  onRefresh() // trigger usePreflightCardStatuses.refresh()
}
```

---

## `getPreflightIntegrationStatuses` — cập nhật để xử lý relay response

```typescript
// src/renderer/src/components/settings/integrations-pane-status.ts [MODIFY]

export function getPreflightIntegrationStatuses(
  status: PreflightStatus | null | undefined,
  remoteStatus?: RemotePreflightStatus | null,
): PreflightIntegrationStatuses {
  // Category A: ưu tiên remote (Dev Server) khi available
  const ghSource = remoteStatus?.gh ?? status?.gh
  const glabSource = remoteStatus?.glab ?? status?.glab

  const ghStatus: GhStatus = !ghSource
    ? 'checking'
    : !ghSource.installed
      ? 'not-installed'
      : ghSource.authenticated
        ? 'connected'
        : 'not-authenticated'

  const glabStatus: GlabStatus = !glabSource
    ? 'checking'
    : !glabSource.installed
      ? 'not-installed'
      : glabSource.authenticated
        ? 'connected'
        : 'not-authenticated'

  // Category B+C: giữ nguyên từ local Orca Server status
  const bitbucketStatus = tokenApiStatusFromPreflight(status?.bitbucket)
  // ... (giữ nguyên logic hiện tại)

  return {
    ghStatus,
    glabStatus,
    bitbucketStatus,
    // ...
  }
}
```

---

## Files cần thay đổi

### [MODIFY] `src/renderer/src/store/slices/preflight.ts`
- Verify `RemotePreflightStatus` type đủ fields (`gh`, `glab`, `git`)
- Verify `setRemotePreflightStatus` được gọi khi nhận relay response

### [MODIFY] `src/renderer/src/components/settings/source-control-preflight-card-status.ts`
- `usePreflightCardStatuses`: khi `activeDevServerId` có giá trị, merge remote + local status
- Thêm `mergePreflightStatuses()` helper

### [MODIFY] `src/renderer/src/components/settings/integrations-pane-status.ts`
- `getPreflightIntegrationStatuses(status, remoteStatus?)`: ưu tiên remote cho Category A
- Thêm optional `remoteStatus` param

### [MODIFY] `src/renderer/src/components/settings/cli-source-control-integration-cards.tsx`
- Hiển thị Dev Server context label khi remote check
- Pass `remoteStatus` vào status calculation

---

## Acceptance Criteria

1. ✅ `preflight.check { devServerId }` gửi từ preflight slice (FE-SOL-01)
2. ✅ Response từ relay cập nhật `remotePreflightByServer[devServerId]` (FE-TASK-10)
3. ✅ `GitHubIntegrationCard` hiển thị `gh.installed` và `gh.authenticated` từ Dev Server (FE-TASK-09)
4. ✅ `GitLabIntegrationCard` hiển thị `glab.installed` và `glab.authenticated` từ Dev Server (FE-TASK-08+09)
5. ✅ Cards cho token integrations vẫn dùng Orca Server status (không bị relay override) — `mergePreflightStatuses` giữ nguyên `bitbucket`, `azureDevOps`, v.v.
6. ⬜ Dev Server label hiển thị trong card khi remote check — deferred (Phase 2)
7. ✅ Fallback về "unavailable" nếu relay không connected (existing behavior giữ nguyên)

---

## Implementation Verified

| File | Thay đổi | Status |
|------|---------|--------|
| `src/shared/dev-server-types.ts` | `RemotePreflightStatus.glab?` added | ✅ FE-TASK-08 |
| `src/renderer/src/store/slices/preflight.ts` | `setRemotePreflightStatus()` called in `.then()` | ✅ FE-TASK-10 |
| `source-control-preflight-card-status.ts` | `mergePreflightStatuses` + `effectiveStatusInput` | ✅ FE-TASK-09 |

**TypeScript:** 0 lỗi mới. Total: 53 (= baseline).
