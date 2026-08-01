# F30 — Remote Integration Management

| Trường | Giá trị |
|--------|---------|
| **ID** | F30 |
| **Tên** | Remote Integration Management |
| **Ưu tiên** | P1 |
| **Trạng thái** | ✅ Phát hành |
| **CRs** | [github/CR-GH-001~005, CR-INT-000~005](../crs/v2/github/) |
| **Phiên bản** | v4.1+ |
| **ADR References** | ADR-006 |
| **HLD References** | C3.9 |

---

## Mô tả

Orca Web Server (multi-user mode) quản lý **tích hợp với các công cụ phát triển** (GitHub, GitLab, Bitbucket, Linear, Jira, v.v.) theo kiến trúc **per-user credential isolation**. Mỗi user có credential riêng, không chia sẻ global environment variables. CLI integrations (GitHub, GitLab) được proxy qua SSH relay đến Dev Server.

---

## Vấn đề cần giải quyết

Trong multi-user web mode:
- **Global ENV vars** như `GITHUB_TOKEN`, `LINEAR_TOKEN` bị shared giữa tất cả users → xung đột, lộ credential
- **CLI tools** (`gh`, `glab`) đọc config file từ path cố định (`~/.config/gh/`) → mỗi user cần isolation riêng
- **Dev Server** là nơi thực thi git commands, nên CLI auth phải ở Dev Server, không phải Orca Server

---

## Phân loại integrations

### Category A — CLI-based (chạy trên Dev Server)

| Integration | CLI Tool | Giải pháp |
|------------|----------|-----------|
| **GitHub** | `gh` | Preflight proxy qua SSH relay; `GH_CONFIG_DIR=~/.config/gh/<userId>/` |
| **GitLab** | `glab` | Preflight proxy qua SSH relay; `GLAB_CONFIG_DIR=~/.config/glab/<userId>/` |

Auth flow: PTY `gh auth login` / `glab auth login` chạy trên Dev Server qua WebSocket PTY stream.

### Category B — HTTP API + Per-user Token

| Integration | Giải pháp |
|------------|-----------|
| **Bitbucket** | App password → `WebCredentialStore` per-user AES-256-GCM |
| **Azure DevOps** | PAT token → `WebCredentialStore` per-user AES-256-GCM |
| **Gitea** | API token → `WebCredentialStore` per-user AES-256-GCM |

### Category C — File-based Token (per-user isolation)

| Integration | Giải pháp |
|------------|-----------|
| **Linear** | API key → `WebCredentialStore` per-user AES-256-GCM |
| **Jira** | Basic auth token → `WebCredentialStore` per-user AES-256-GCM |

---

## Tính năng chi tiết

### WebCredentialStore — Per-User AES-256-GCM Storage (CR-INT-004)

```
Encryption:
  masterKey = scryptSync(userId + ':' + ORCA_CREDENTIAL_KEY, userId, {N:16384, r:8, p:1, keylen:32})
  iv = randomBytes(12)
  { ciphertext, authTag } = AES-256-GCM.encrypt(plaintext, masterKey, iv)

Storage:
  ~/.orca/users/<userId>/<service>.enc   (chmod 0600)
  service: github | gitlab | bitbucket | azure-devops | gitea | linear | jira
```

**RPC Methods** (`credentials.*`):
```typescript
credentials.set(service, token)     // encrypt + store
credentials.revoke(service)         // delete file
credentials.status(service)         // { configured: bool, lastValidated? }
credentials.list()                  // [service names only — KHÔNG trả token]
```

---

### Preflight Check — Remote & Local Merge (CR-INT-005)

```
Browser: preflight.check { devServerId }
    │
    ├── runLocalChecks():
    │     - git version (local)
    │     - API token format (Category B+C: check configured, không test)
    │
    ├── runRelayChecks(devServerId) via SSH:
    │     - GH_CONFIG_DIR=~/.config/gh/<userId>/ gh auth status
    │     - GLAB_CONFIG_DIR=~/.config/glab/<userId>/ glab auth status
    │     - node --version
    │     - disk space (df -h .)
    │
    └── mergePreflightStatuses(local, relay):
          relay overrides local cho CLI checks (relay là authoritative)
          fallback local-only nếu SSH fail (+ 'relay-connectivity' warning)
```

---

### CLI Auth Login Flow (CR-GH-002, CR-INT-001)

```
User click "Login with GitHub" trong Settings UI
    │
    ▼
github.startAuthLogin({ devServerId })
    │ WebSocket PTY spawn
    ▼
Dev Server: gh auth login  (terminal dialog)
    │ User xác nhận trong terminal emulator
    ▼
gh lưu token vào GH_CONFIG_DIR=~/.config/gh/<userId>/
    │
    ▼
preflight.check xác nhận → badge ✅
```

---

### Session Isolation — GH/GLAB Config Dir (CR-GH-004)

```typescript
// Mỗi SSH exec command inject env vars
const env = {
  GH_CONFIG_DIR: `/home/ubuntu/.config/gh/${userId}/`,
  GLAB_CONFIG_DIR: `/home/ubuntu/.config/glab-cli/${userId}/`,
}
// → mỗi user có config/auth riêng trên Dev Server
// → user A login GitHub không ảnh hưởng user B
```

---

### Frontend Settings UI

**Category A (GitHub/GitLab) — `WebModeCliAuthSection`:**
- Hiển thị preflight status (✅ Authenticated / ⚠️ Not logged in)
- Button "Login via Terminal" → mở PTY modal → chạy `gh auth login`
- Button "Logout" → `gh auth logout` via PTY

**Category B+C (Bitbucket/Linear/etc) — `CredentialInputForm`:**
- Input field nhập API token
- Save → `credentials.set(service, token)`
- Revoke → `credentials.revoke(service)`
- Status badge: configured / not configured

---

## Kiến trúc tổng thể (CR-INT-000)

```
Browser (wss://)
    │
Nginx :443 → Orca Server :6768
    │
    ├── preflight.check
    │      │ Category A (CLI) → SSH Relay → Dev Server
    │      │     gh/glab auth status (per-user env)
    │      └── Category B+C → Orca Server
    │               WebCredentialStore.get(userId, service)
    │               → HTTP API test call
    │
    ├── credentials.set/revoke/status/list
    │      └── WebCredentialStore (per-user AES-256-GCM)
    │
    └── github.startAuthLogin / gitlab.startAuthLogin
           └── SSH PTY → gh/glab auth login (interactive)
```

---

## Tiêu chí chấp nhận

- [x] `WebCredentialStore` AES-256-GCM, per-user, per-service file
- [x] `credentials.set/revoke/status/list` RPC hoạt động
- [x] `credentials.list()` không trả về token value (chỉ service names)
- [x] GitHub preflight proxy qua SSH relay, `GH_CONFIG_DIR` isolation
- [x] GitLab preflight proxy qua SSH relay, `GLAB_CONFIG_DIR` isolation
- [x] `gh auth login` PTY flow qua Dev Server
- [x] `glab auth login` PTY flow qua Dev Server
- [x] Bitbucket/AzureDevOps/Gitea `CredentialInputForm` UI
- [x] Linear/Jira `CredentialInputForm` UI
- [x] `mergePreflightStatuses` — relay override local cho CLI
- [x] `ORCA_CREDENTIAL_KEY` env var required, không hardcode
- [x] 0 TS errors mới. Baseline: 53 pre-existing
- [x] 11/11 CRs implemented

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Credential store | `src/main/credentials/web-credential-store.ts` |
| RPC: credentials | `src/main/runtime/rpc/methods/credentials.ts` |
| RPC: github-auth | `src/main/runtime/rpc/methods/github-auth.ts` |
| RPC: gitlab-auth | `src/main/runtime/rpc/methods/gitlab-auth.ts` |
| RPC: preflight | `src/main/runtime/rpc/methods/preflight.ts` |
| RpcMethodContext | `src/main/runtime/rpc/runtime-rpc.ts` |
| Shared types | `src/shared/dev-server-types.ts` |
| Frontend: CLI auth | `src/renderer/src/components/WebModeCliAuthSection.tsx` |
| Frontend: token form | `src/renderer/src/components/CredentialInputForm.tsx` |
| Frontend: GH cards | `src/renderer/src/.../cli-source-control-integration-cards.tsx` |
| Frontend: token cards | `src/renderer/src/.../token-source-control-integration-cards.tsx` |
| Frontend: task tracker | `src/renderer/src/.../task-tracker-integration-cards.tsx` |
| Frontend: preflight store | `src/renderer/src/slices/preflight.ts` |

**Env required:** `ORCA_CREDENTIAL_KEY` (AES key seed, min 32 chars)

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| Credential encrypt/decrypt | < 10ms |
| Preflight check (all integrations) | < 3s |
| Zero token leakage | `credentials.list()` không trả token |
| Per-user isolation | 100% — user A không đọc được credential user B |
