# CR-INT-000: Tổng quan — Remote Integration Architecture cho Web + Dev Server mode

**ID:** CR-INT-000  
**Priority:** 🔴 Critical (Architecture Document)  
**Type:** Architecture Overview  
**Status:** ✅ Implemented & Documented — 2026-07-25  
**Note:** Architecture overview doc — all CRs in scope implemented.

---

## Bối cảnh

Orca Web mode (`ORCA_MULTI_USER=1`) có kiến trúc 3-tier:

```
Browser (HTTPS)
    │ wss://b15.openledger.vn
    ▼
Nginx :443
    │
    ▼
Orca Server Container (172.20.2.39)
    │ SSH Relay
    ▼
Dev Server (172.20.2.31)  ← Nơi code thực sự tồn tại
```

---

## Phân loại Integration theo nơi cần thực thi

### Category A: CLI-based (Cần chạy trên Dev Server)

Các integrations này dùng binary CLI (`gh`, `glab`) được cài trên Dev Server.
Credentials được lưu trên Dev Server (file-based).

| Integration | CLI | Auth method | Config location |
|------------|-----|-------------|----------------|
| **GitHub** | `gh` | `gh auth login` → `~/.config/gh/hosts.yml` | Dev Server |
| **GitLab** | `glab` | `glab auth login` → `~/.config/glab/config.yml` | Dev Server |

**Vấn đề:** `preflight.check` → `execLocalPreflightCommand('gh/glab')` chạy trên **Orca Server container**, không phải Dev Server.

**Fix:** Proxy `preflight.check` qua SSH relay → Dev Server (CR-GH-001, CR-INT-001)

---

### Category B: HTTP API + Env Token (Cần inject trên Orca Server, nhưng phải per-user)

Các integrations này dùng REST API với token từ `process.env.*`. Trong Web mode, env vars thuộc về Orca Server process (shared giữa mọi users).

| Integration | Env Vars | Auth type |
|------------|---------|-----------|
| **Bitbucket** | `ORCA_BITBUCKET_ACCESS_TOKEN`, `ORCA_BITBUCKET_API_TOKEN`, `ORCA_BITBUCKET_EMAIL` | OAuth2 / App password |
| **Azure DevOps** | `ORCA_AZURE_DEVOPS_TOKEN`, `ORCA_AZURE_DEVOPS_API_BASE_URL` | PAT token |
| **Gitea** | `ORCA_GITEA_TOKEN`, `ORCA_GITEA_API_BASE_URL` | API token |

**Vấn đề:** Env vars là global → tất cả users dùng chung credentials → multi-user conflict.

**Fix:** Per-user credential store trên Orca Server (session-scoped env injection) (CR-INT-002)

---

### Category C: File-based Token (Orca Server, nhưng cần session isolation)

Các integrations này lưu encrypted token vào disk của Orca Server.

| Integration | Token path | Encryption |
|------------|-----------|-----------|
| **Linear** | `~/.orca/linear-token.enc` | `safeStorage` (Electron) |
| **Jira** | `~/.orca/jira-tokens/{siteId}.enc` | `safeStorage` (Electron) |

**Vấn đề:** Trong Web/headless mode, `safeStorage` (Electron API) không available → plaintext fallback.
Đường dẫn file là global user home → không isolated per user session.

**Fix:** Per-user token storage trong data path (CR-INT-003)

---

## Tóm tắt Change Requests

| CR | Integration | Category | Vấn đề | Ưu tiên |
|----|------------|----------|--------|---------|
| CR-GH-001~005 | GitHub | A | CLI trên container | 🔴 Critical |
| [CR-INT-001](./CR-INT-001-gitlab-remote-devserver.md) | GitLab | A | CLI trên container | 🔴 Critical |
| [CR-INT-002](./CR-INT-002-api-token-integrations-multiuser.md) | Bitbucket, Azure DevOps, Gitea | B | Shared env vars | 🟠 High |
| [CR-INT-003](./CR-INT-003-file-token-session-isolation.md) | Linear, Jira | C | File token not isolated | 🟠 High |
| [CR-INT-004](./CR-INT-004-unified-credential-manager.md) | All | — | Unified credential management | 🟡 Medium |
| [CR-INT-005](./CR-INT-005-preflight-check-full-remote.md) | All | — | Full preflight.check extension | 🟡 Medium |

---

## Kiến trúc mục tiêu

```
┌─────────────────────────────────────────────────────────┐
│                    Orca Server                           │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │  Category B+C: API Token Integrations            │   │
│  │  (Bitbucket, AzDO, Gitea, Linear, Jira)          │   │
│  │                                                   │   │
│  │  CredentialStore.getToken(userId, 'bitbucket')   │   │
│  │  → Decrypt(session-scoped token)                 │   │
│  │  → HTTP API call                                 │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │  Category A: CLI Integrations Proxy              │   │
│  │  (GitHub, GitLab)                                │   │
│  │                                                   │   │
│  │  relay.call('preflight.check', {devServerId})    │   │
│  │        │                                         │   │
│  └────────┼────────────────────────────────────────┘   │
│           │ SSH Relay                                    │
└───────────┼─────────────────────────────────────────────┘
            ▼
┌─────────────────────────────────────────────────────────┐
│                    Dev Server                            │
│                                                          │
│  gh --version, gh auth status (GH_CONFIG_DIR session)   │
│  glab --version, glab auth status                        │
│  git operations, clone, PR creation                      │
└─────────────────────────────────────────────────────────┘
```
