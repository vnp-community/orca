# Luồng Dữ liệu — Remote Source Control Integrations

**Domain:** Remote Source Control Integrations  
**Nghiệp vụ:** BL-INT-01 → BL-INT-03  
**Kiến trúc tham chiếu:** HLD v1 — C3.9, C4.6, ADR-006, F30 Remote Integrations

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Dev Server (Remote) | Runtime | Nơi chạy git, gh CLI, glab CLI |
| Orca Relay Binary | Remote Bridge | Proxy CLI auth và API calls |
| WebCredentialStore | Security | AES-256-GCM per-user per-service token storage |
| Main Process | Business Logic | CliAuthProxy, RemoteIntegrationService |
| GitHub/GitLab REST API | External | Source control operations |
| SSH Relay | Transport | Main → Dev Server relay |

---

## BL-INT-01 — CLI Auth Proxy (GitHub/GitLab qua SSH Relay)

```
Carlos (trên Dev Server, chạy gh/glab CLI)
    │
    ▼
[Dev Server] gh CLI gọi: `gh auth token` (cần credential)
    │ gh CLI gọi git-credential-orca helper (được cấu hình bởi Orca)
    ▼
[Orca Relay — CliAuthProxy.handleCredentialRequest()]
    │ Relay protocol: { type: 'credential.request', service: 'github.com', userId }
    ▼
[Main Process — CliAuthProxy.serve()]
    ├─ Nhận credential request từ relay (qua SSH tunnel)
    ├─ Load token: WebCredentialStore.get('github', userId)
    │   Decrypt: AES-256-GCM(key=scrypt(ORCA_CREDENTIAL_KEY, userId+'github'), cipher)
    └─ Relay response: { type: 'credential.response', token: '<decrypted_token>' }
    │
    ▼
[Orca Relay] trả token cho gh CLI
    gh CLI thực thi: gh pr create ...

Luồng:
Dev Server (gh CLI) → Orca Relay credential helper
                   → relay protocol → SSH tunnel → Main Process
                   → WebCredentialStore (decrypt AES-256-GCM)
                   → relay response → Relay → gh CLI
```

---

## BL-INT-02 — WebCredentialStore (API Token Management)

```
Người dùng (Carlos/Alex) muốn lưu GitHub token
    │
    ▼
[Renderer] Settings → Integrations → GitHub → "Add Token"
    Input: { service: 'github', token: 'ghp_xxx' }
    │ contextBridge.invoke('credential.store', { service, token })
    ▼
[Main Process — WebCredentialStore.store()]
    ├─ Derive encryption key: scrypt(ORCA_CREDENTIAL_KEY, userId + service)
    ├─ Encrypt: AES-256-GCM(key, token) → { ciphertext, iv, authTag }
    └─ Write: ~/.orca/users/<userId>/credentials.enc
        (JSON map: { 'github': { ciphertext, iv, authTag } })

RETRIEVE:
    Main Process → WebCredentialStore.get(service, userId)
    ├─ Read ~/.orca/users/<userId>/credentials.enc
    ├─ Decrypt: AES-256-GCM(key, ciphertext) → plaintext token
    └─ Return token (in-memory only, never persisted as plaintext)

DELETE:
    contextBridge.invoke('credential.delete', { service })
    → Remove entry từ credentials.enc

Luồng:
User → Renderer → IPC → Main → AES-256-GCM encrypt → write credentials.enc

Retrieve (internal):
Main → read credentials.enc → AES-256-GCM decrypt → plaintext token (in-memory)
```

---

## BL-INT-03 — Preflight Status Merge (Local + Remote)

```
Người dùng (Carlos/Alex) trước khi tạo PR hoặc merge
    │
    ▼
[Renderer] click "Preflight Check" trong Code Review panel
    │ contextBridge.invoke('preflight.check', { worktreeId })
    ▼
[Main Process — PreflightService.check()]
    │
    ├── LOCAL checks (parallel):
    │   ├─ git status (uncommitted changes)     ← Git CLI local
    │   ├─ git log HEAD..origin/main (behind?)  ← Git CLI local
    │   └─ npm test / cargo test (local tests)  ← child_process
    │
    ├── REMOTE checks (via relay, parallel):
    │   ├─ relay.call('git.status', { repoPath })
    │   ├─ GitHub API: GET /repos/{owner}/{repo}/commits/{sha}/check-runs
    │   │   ← CI status checks
    │   └─ GitHub API: GET /repos/{owner}/{repo}/pulls?head={branch}
    │       ← existing PRs for branch
    │
    ├── MERGE results:
    │   { local: { clean, upToDate, testsPassed },
    │     remote: { ciStatus, existingPr, reviewStatus } }
    └─ emit: preflight:completed { result }
    │
    ▼
[Renderer] Preflight panel:
    ✅ Local: clean, up-to-date, tests passed
    ⚠️  CI: 1/3 checks failing
    ℹ️  No existing PR

Luồng:
User → Renderer → IPC → Main
                       → Git CLI (local checks)
                       → relay RPC (remote git status)
                       → GitHub REST API (CI status + PR lookup)
                       → Merge results → Renderer
```

---

## Sơ đồ tổng quan — Remote Integration

```
┌─────────────┐   IPC   ┌──────────────────────────────────────────┐
│  Renderer   │◄───────►│  Main Process                            │
│  Integrations│         │  WebCredentialStore (AES-256-GCM)       │
│  Preflight  │         │  CliAuthProxy                            │
│  Settings   │         │  PreflightService                        │
└─────────────┘         └───────┬──────────────┬────────────────────┘
                                │              │
                       ┌────────▼──┐   ┌───────▼──────────────────┐
                       │credentials│   │  SSH Relay → Dev Server  │
                       │.enc (AES) │   │  CliAuthProxy (relay)    │
                       └───────────┘   └───────┬──────────────────┘
                                               │
                                   ┌───────────┼─────────────────────┐
                                   │           │                     │
                          ┌────────▼──┐ ┌──────▼──┐        ┌────────▼──┐
                          │ gh CLI    │ │ glab CLI│        │GitHub API │
                          │ (on dev   │ │(on dev  │        │REST/GraphQL│
                          │ server)   │ │ server) │        └───────────┘
                          └───────────┘ └─────────┘

Credential flow:
Local: credentials.enc ─ scrypt key ─ AES-256-GCM ─ plaintext (in-memory only)
Remote: Main → relay → dev server (credential helper injection for gh/glab)
```
