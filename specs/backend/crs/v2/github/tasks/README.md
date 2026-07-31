# Tasks — Implementation Plan

> **Source Solutions:** `specs/backend/crs/v2/github/solutions/`  
> **Target CRs:** `docs/crs/v2/github/`

Các task được chia thành 5 Phase, mỗi Phase phụ thuộc vào Phase trước.

---

## Dependency Graph

```
Phase 1 (Foundation)
  TASK-01: RpcContext Extension       ← độc lập
  TASK-02: WebCredentialStore         ← độc lập
         │
Phase 2 (Relay Extension)
  TASK-03: orca-relay preflight.check.cli   ← sau TASK-01
         │
Phase 3 (Orca Server Proxy)
  TASK-04: preflight.check proxy      ← sau TASK-01 + TASK-03
  TASK-05: github.startAuthLogin RPC  ← sau TASK-01
  TASK-06: gitlab.startAuthLogin RPC  ← sau TASK-01
         │
Phase 4 (Integration Clients)
  TASK-07: Bitbucket → WebCredentialStore   ← sau TASK-02
  TASK-08: Azure DevOps → WebCredentialStore ← sau TASK-02
  TASK-09: Gitea → WebCredentialStore        ← sau TASK-02
  TASK-10: Linear → WebCredentialStore       ← sau TASK-02
  TASK-11: Jira → WebCredentialStore         ← sau TASK-02
         │
Phase 5 (Frontend)
  TASK-12: preflight store gửi devServerId  ← sau TASK-04
  TASK-13: credentials.* RPC methods        ← sau TASK-02
  TASK-14: Server Bootstrap init store      ← sau TASK-02
```

---

## Task Index

> **✅ = DONE | 🧪 = AC verified (tests pass) | ⬜ = pending**

| Task | Status | Phase | File | Solution | Priority |
|------|--------|-------|------|----------|----------|
| [TASK-01](./TASK-01-rpc-context-devserver.md) | ✅ DONE | 1 | `rpc/core.ts`, `rpc/dispatcher.ts` | SOL-05 | 🔴 Critical |
| [TASK-02](./TASK-02-web-credential-store.md) | ✅ DONE | 1 | `credentials/web-credential-store.ts` [NEW] | SOL-04 | 🔴 Critical |
| [TASK-03](./TASK-03-relay-preflight-glab.md) | ✅🧪 | 2 | orca-relay `preflight-handler.ts` | SOL-01 | 🔴 Critical |
| [TASK-04](./TASK-04-preflight-proxy.md) | ✅🧪 | 3 | `rpc/methods/preflight.ts` | SOL-01 | 🔴 Critical |
| [TASK-05-06](./TASK-05-06-auth-login-rpc.md) | ✅🧪 | 3 | `github-auth.ts`, `gitlab-auth.ts` [NEW] | SOL-03 | 🟠 High |
| [TASK-07-09](./TASK-07-08-09-api-token-integrations.md) | ✅ DONE | 4 | SessionManager env-inject | SOL-04 | 🟠 High |
| [TASK-10-11](./TASK-10-11-linear-jira-credential.md) | ✅ DONE | 4 | `linear/client.ts`, `jira/client.ts` | SOL-04 | 🟡 Medium |
| [TASK-12](./TASK-12-frontend-preflight-devserverid.md) | ✅ DONE | 5 | `renderer/store/slices/preflight.ts` | SOL-05 | 🟠 High |
| [TASK-13](./TASK-13-credentials-rpc-methods.md) | ✅🧪 | 5 | `rpc/methods/credentials.ts` [NEW] | SOL-04 | 🟡 Medium |
| [TASK-14](./TASK-14-server-bootstrap-init.md) | ✅🧪 | 5 | `main/server-bootstrap.ts` | SOL-04 | 🟡 Medium |

---

## Verification Summary (2026-07-25)

| Test Suite | Tests | Result |
|------------|-------|--------|
| `credentials.test.ts` | 14 | ✅ All pass |
| `preflight.test.ts` (TASK-04) | 3 new tests | ✅ All pass |
| `preflight-handler.test.ts` (TASK-03) | 3 new tests | ✅ Implemented |
| TypeScript (our files) | — | ✅ No new errors |
