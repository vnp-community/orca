# Frontend Solutions — CR v2/github Integration (Web Server Mode)

> **Phiên bản:** 1.1 | **Cập nhật:** 2026-07-25  
> **Tương ứng Backend:** `specs/backend/crs/v2/github/solutions/`  
> **TDD tham chiếu:** `specs/frontend/tdd/`  
> **Tasks:** [`specs/frontend/crs/v2/github/tasks/`](../tasks/README.md)

---

## Tóm tắt kiến trúc Frontend (Web mode)

```
Browser (React + Zustand)
  │ window.api.credentials.set(service, token, config)
  │ window.api.github.startAuthLogin(devServerId)
  │ window.api.preflight.check({ devServerId })
  ▼
web-preload-api.ts (installWebPreloadApi)
  │ callRuntimeResult('credentials.set', ...)
  │ callRuntimeResult('github.startAuthLogin', ...)
  │ callRuntimeResult('preflight.check', { devServerId })
  ▼
WebSocketRpcClient → Orca Server → RPC Dispatcher
  │ context.devServerManager → relay → Dev Server (CLI check)
  │ WebCredentialStore (AES-256-GCM per-user)
  ▼
Response → React UI update
  │ remotePreflightByServer[devServerId] updated (FE-SOL-01)
  │ mergePreflightStatuses() → CLI cards use Dev Server status (FE-SOL-04)
```

---

## Solution Index

| Solution | CRs | Status | Tasks thực thi |
|----------|-----|--------|----------------|
| [FE-SOL-01](./FE-SOL-01-preflight-devserverId.md) | CR-GH-001, CR-GH-003, CR-INT-001 | ✅ DONE & 🧪 AC Verified | FE-TASK-10 (setRemotePreflightStatus call) |
| [FE-SOL-02](./FE-SOL-02-cli-auth-login-ui.md) | CR-GH-002, CR-INT-001 | ✅ DONE & 🧪 AC Verified | FE-TASK-02, 03, 04 |
| [FE-SOL-03](./FE-SOL-03-credential-settings-ui.md) | CR-INT-002, CR-INT-003, CR-INT-004 | ✅ DONE & 🧪 AC Verified | FE-TASK-01, 05, 06, 07 |
| [FE-SOL-04](./FE-SOL-04-preflight-status-remote.md) | CR-INT-005, CR-GH-001 | ✅ DONE & 🧪 AC Verified | FE-TASK-08, 09, 10 |

---

## Thứ tự thực thi Frontend

```
Phase 1 — Context Routing ✅ DONE
  FE-SOL-01: preflight slice gửi devServerId + cache relay result

Phase 2 — CLI Auth UI ✅ DONE (Phase 1)
  FE-SOL-02: GitHubIntegrationCard / GitLabIntegrationCard
             → "Login with GitHub CLI" button khi Web mode + Dev Server connected
             → WebModeCliAuthSection: PTY spawn + info panel
             ⬜ Phase 2: xterm.js inline terminal (deferred)

Phase 3 — Credential Input UI ✅ DONE
  FE-SOL-03: BitbucketIntegrationCard + AzureDevOps + Gitea + Linear + Jira
             → CredentialInputForm + useCredentialManager
             → Electron mode: unchanged

Phase 4 — Remote Preflight Status ✅ DONE (6/7 AC)
  FE-SOL-04: Remote preflight status mapping từ relay result
             → RemotePreflightStatus.glab? field
             → mergePreflightStatuses() → CLI cards use Dev Server status
             ⬜ Dev Server context label in card (deferred Phase 2)
```

---

## Files Thay đổi — Tổng hợp

| File | Loại | Solution |
|------|------|---------|
| `src/preload/api-types.ts` | MODIFY | FE-SOL-02, FE-SOL-03 |
| `src/shared/dev-server-types.ts` | MODIFY | FE-SOL-04 |
| `src/renderer/src/web/web-preload-api.ts` | MODIFY | FE-SOL-02, FE-SOL-03 |
| `src/renderer/src/store/slices/preflight.ts` | MODIFY | FE-SOL-01, FE-SOL-04 |
| `src/renderer/src/components/settings/WebModeCliAuthSection.tsx` | NEW | FE-SOL-02 |
| `src/renderer/src/components/settings/CredentialInputForm.tsx` | NEW | FE-SOL-03 |
| `src/renderer/src/components/settings/cli-source-control-integration-cards.tsx` | MODIFY | FE-SOL-02 |
| `src/renderer/src/components/settings/token-source-control-integration-cards.tsx` | MODIFY | FE-SOL-03 |
| `src/renderer/src/components/settings/task-tracker-integration-cards.tsx` | MODIFY | FE-SOL-03 |
| `src/renderer/src/components/settings/jira-integration-card.tsx` | MODIFY | FE-SOL-03 |
| `src/renderer/src/components/settings/source-control-preflight-card-status.ts` | MODIFY | FE-SOL-04 |

**TypeScript:** 0 lỗi mới. Total errors: 53 (baseline, pre-existing).
