# Integration Change Requests — Web Server + Remote Dev Server Mode

> **Context:** Orca Web mode (`ORCA_MULTI_USER=1`), kiến trúc:
> ```
> Browser (HTTPS) → Nginx → Orca Server (172.20.2.39)
>                                 ↓ SSH Relay
>                           Dev Server (172.20.2.31)
> ```
> **Status tổng quan:** ✅ **11/11 CRs Implemented** — 2026-07-25

---

## Phân loại integrations

### Category A — CLI-based (Cần chạy trên Dev Server) ✅ DONE

Integrations dùng binary CLI (`gh`, `glab`). Credentials lưu trên Dev Server.

| CR | Integration | Giải pháp | Status |
|----|------------|-----------|--------|
| [CR-GH-001](./CR-GH-001-preflight-remote-devserver.md) | GitHub | `devServerId` relay proxy trong `preflight.check` | ✅ Implemented |
| [CR-GH-002](./CR-GH-002-gh-auth-login-remote-pty.md) | GitHub | PTY `gh auth login` via `github.startAuthLogin()` | ✅ Implemented |
| [CR-INT-001](./CR-INT-001-gitlab-remote-devserver.md) | GitLab | `glab` relay proxy + `gitlab.startAuthLogin()` PTY | ✅ Implemented |

### Category B — HTTP API + Env Token (Multi-user conflict) ✅ DONE

Integrations dùng REST API với token từ `process.env.*` → thay bằng per-user `WebCredentialStore`.

| CR | Integration | Giải pháp | Status |
|----|------------|-----------|--------|
| [CR-INT-002](./CR-INT-002-api-token-integrations-multiuser.md) | Bitbucket, Azure DevOps, Gitea | `credentials.*` RPC + AES-256-GCM per-user store | ✅ Implemented |

### Category C — File-based Token (Session isolation needed) ✅ DONE

Integrations lưu token vào disk với path global → thay bằng per-user `WebCredentialStore`.

| CR | Integration | Giải pháp | Status |
|----|------------|-----------|--------|
| [CR-INT-003](./CR-INT-003-file-token-session-isolation.md) | Linear, Jira | `credentials.*` RPC + WebCredentialStore | ✅ Implemented |

### Infrastructure ✅ DONE

| CR | Component | Mô tả | Status |
|----|----------|--------|--------|
| [CR-INT-000](./CR-INT-000-integration-remote-overview.md) | Architecture | Overview + phân loại tất cả integrations | ✅ Documented |
| [CR-GH-003](./CR-GH-003-preflight-context-webmode.md) | Renderer | `devServerId` context routing cho `preflight.check` | ✅ Implemented |
| [CR-GH-004](./CR-GH-004-gh-token-session-isolation.md) | Multi-user | `GH_CONFIG_DIR` session isolation via env injection | ✅ Implemented |
| [CR-GH-005](./CR-GH-005-server-side-preflight-rpc.md) | RPC Core | `RpcMethodContext` + `devServerManager` injection | ✅ Implemented |
| [CR-INT-004](./CR-INT-004-unified-credential-manager.md) | Credentials | `WebCredentialStore` — AES-256-GCM unified storage | ✅ Implemented |
| [CR-INT-005](./CR-INT-005-preflight-check-full-remote.md) | Preflight | Full `preflight.check` extension + `mergePreflightStatuses` | ✅ Implemented |

---

## Thứ tự thực thi — Hoàn thành

```
Phase 1 — Infrastructure ✅
  CR-GH-005  →  RpcMethodContext + devServerManager injection
  CR-INT-004 →  WebCredentialStore (AES-256-GCM per-user)

Phase 2 — CLI integrations (Category A) ✅
  CR-GH-001  →  GitHub preflight proxy → Dev Server (relay)
  CR-GH-003  →  Renderer sends devServerId → mergePreflightStatuses
  CR-INT-001 →  GitLab preflight proxy + glab relay

Phase 3 — Auth flows ✅
  CR-GH-002  →  gh auth login via PTY (WebModeCliAuthSection)
  CR-GH-004  →  GH_CONFIG_DIR / GLAB_CONFIG_DIR session isolation

Phase 4 — API Token integrations (Category B+C) ✅
  CR-INT-002 →  Bitbucket/AzDO/Gitea CredentialInputForm + WebCredentialStore
  CR-INT-003 →  Linear/Jira CredentialInputForm + WebCredentialStore

Phase 5 — Full integration ✅
  CR-INT-005 →  preflight.check full extension (relay CLI + local API)
```

---

## Kiến trúc đã implement

```
Browser
  │ wss://...
  ▼
Nginx :443 → Orca Server :6768 (WebSocket RPC)

preflight.check { devServerId: "ds-abc" }
  │
  ├─ CLI check (Category A) → SSH Relay → Dev Server [✅]
  │    gh --version, gh auth status (GH_CONFIG_DIR per-user)
  │    glab --version, glab auth status (GLAB_CONFIG_DIR per-user)
  │    git --version
  │    ↓ mergePreflightStatuses() ưu tiên relay cho CLI cards
  │
  └─ API check (Category B+C) → Orca Server → External APIs [✅]
       Bitbucket: WebCredentialStore.getToken(userId, 'bitbucket')
       Azure DevOps: WebCredentialStore.getToken(userId, 'azure-devops')
       Gitea: WebCredentialStore.getToken(userId, 'gitea')
       Linear: WebCredentialStore.getToken(userId, 'linear')
       Jira: WebCredentialStore.getToken(userId, 'jira')

Frontend Settings UI [✅]
  ├─ GitHub/GitLab: WebModeCliAuthSection → PTY auth login
  └─ Bitbucket/AzDO/Gitea/Linear/Jira: CredentialInputForm
```

---

## Implementation Summary

| Layer | Files thay đổi | CRs |
|-------|---------------|-----|
| Backend RPC | `runtime-rpc.ts`, `methods/preflight.ts`, `methods/credentials.ts`, `methods/github-auth.ts`, `methods/gitlab-auth.ts` | GH-001, GH-002, GH-005, INT-001, INT-004 |
| Backend Store | `credentials/web-credential-store.ts` | INT-002, INT-003, INT-004 |
| Shared Types | `dev-server-types.ts` | INT-005 |
| Preload | `api-types.ts` | GH-002, INT-004 |
| Frontend Web | `web-preload-api.ts` | GH-002, INT-004 |
| Frontend UI | `WebModeCliAuthSection.tsx`, `CredentialInputForm.tsx` | GH-002, INT-002, INT-003 |
| Frontend Cards | `cli-source-control-integration-cards.tsx`, `token-source-control-integration-cards.tsx`, `task-tracker-integration-cards.tsx`, `jira-integration-card.tsx` | GH-002, INT-002, INT-003 |
| Frontend Store | `slices/preflight.ts`, `source-control-preflight-card-status.ts` | GH-001, GH-003, INT-005 |

**TypeScript:** 0 lỗi mới. Baseline: 53 (pre-existing).

---

## File structure

```
docs/crs/v2/github/
├── README.md                                    ← (this file)
├── CR-INT-000-integration-remote-overview.md    ← Architecture overview
│
├── Category A — CLI integrations
│   ├── CR-GH-001-preflight-remote-devserver.md
│   ├── CR-GH-002-gh-auth-login-remote-pty.md
│   └── CR-INT-001-gitlab-remote-devserver.md
│
├── Category B+C — Token integrations
│   ├── CR-INT-002-api-token-integrations-multiuser.md
│   └── CR-INT-003-file-token-session-isolation.md
│
└── Infrastructure
    ├── CR-GH-003-preflight-context-webmode.md
    ├── CR-GH-004-gh-token-session-isolation.md
    ├── CR-GH-005-server-side-preflight-rpc.md
    ├── CR-INT-004-unified-credential-manager.md
    └── CR-INT-005-preflight-check-full-remote.md
```
