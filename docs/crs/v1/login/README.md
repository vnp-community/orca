# Change Requests: Multi-User Login & Isolation (v1)

> **Phiên bản**: v1.0
> **Ngày**: 2026-07-24
> **Scope**: Orca Server — Authentication, Per-User Sandbox, SSH Isolation, Admin
> **Liên quan**: `src/main/runtime/runtime-rpc.ts`, `src/shared/rbac-types.ts`, `src/main/ssh/`

---

## ✅ Implementation Status

> **PHASE 1 HOÀN THÀNH — 2026-07-24**  
> Backend: 134 tests pass | Frontend: 60+ tests pass | 0 TypeScript errors

| CR | Tên | AC Done | Status |
|----|-----|---------|--------|
| [CR-LOGIN-001](./CR-LOGIN-001-auth.md) | Authentication: Login + SSO | 6/9 ✅ | ✅ Phase 1 Done (SSO deferred) |
| [CR-LOGIN-002](./CR-LOGIN-002-sandbox.md) | Per-User Sandbox | 8/8 ✅ | ✅ Done |
| [CR-LOGIN-003](./CR-LOGIN-003-ssh-isolation.md) | SSH Isolation | 6/7 ✅ | ✅ Phase 1 Done (Git Worktree deferred) |
| [CR-LOGIN-004](./CR-LOGIN-004-admin.md) | Admin UI | 8/9 ✅ | ✅ Phase 1 Done (Access Policy deferred) |

**Deferred (Phase 2/3):**
- SSO OAuth redirect + `/auth/callback` (Phase 2)
- OIDC/Keycloak JWT verification (Phase 3)
- Access Policy SSH enforcement at runtime (Phase 3)
- Git Worktree per-user isolation (Phase 3)

---

## Bối cảnh & Vấn đề

### Kiến trúc hiện tại

Orca Server hiện tại được thiết kế là **single-user, single-runtime** model:

```
Browser ──wss://b15.openledger.vn──► Orca Server (1 process)
                                          │
                                    1 runtime duy nhất
                                    1 SQLite database
                                    shared userDataPath
                                          │
                                    ──SSH──► Dev Machine (shared unix user)
```

Cơ chế xác thực hiện tại là **PairCode only** (base64 JSON-encoded `PairingOffer` chứa E2EE public key + device token). Mọi client dùng cùng Orca runtime, cùng filesystem view, không có isolation.

### Các vấn đề cần giải quyết

| # | Vấn đề | Impact |
|---|--------|--------|
| 1 | Không có login/SSO — PairCode phải share thủ công | Không scale, không kiểm soát được |
| 2 | Mọi user share 1 runtime process — side effects lẫn nhau | Data leak, process crash ảnh hưởng tất cả |
| 3 | SSH vào dev server dùng chung unix user — file, env, history chung | Không audit được, không cô lập |
| 4 | Không có admin UI quản lý user | Không thể onboard/offboard user |

---

## Danh sách Change Requests

| CR ID | Tên | File chi tiết |
|-------|-----|---------------|
| CR-LOGIN-001 | Authentication: Login + SSO bên cạnh PairCode | [CR-LOGIN-001-auth.md](./CR-LOGIN-001-auth.md) |
| CR-LOGIN-002 | Per-User Sandbox: Isolated Runtime Process | [CR-LOGIN-002-sandbox.md](./CR-LOGIN-002-sandbox.md) |
| CR-LOGIN-003 | SSH Dev Server: Per-User Unix Account Isolation | [CR-LOGIN-003-ssh-isolation.md](./CR-LOGIN-003-ssh-isolation.md) |
| CR-LOGIN-004 | Admin UI: User Management | [CR-LOGIN-004-admin.md](./CR-LOGIN-004-admin.md) |

---

## Dependency Map

```
CR-LOGIN-001 (Auth/SSO)
    │
    ├──► CR-LOGIN-002 (Per-User Sandbox)
    │         │
    │         └──► CR-LOGIN-003 (SSH Dev Isolation)
    │
    └──► CR-LOGIN-004 (Admin UI)
```

> ⚠️ **Thứ tự triển khai bắt buộc**: CR-001 → CR-002 → CR-003. CR-004 có thể làm song song với CR-002.

---

## Kiến trúc mục tiêu (sau khi apply tất cả CR)

```
                  b15.openledger.vn
                       │
              ┌────────┴────────┐
              │  Orca Gateway   │
              │  (nginx proxy)  │
              └────────┬────────┘
                       │
              ┌────────┴────────┐
              │  Auth Layer     │ ◄── CR-LOGIN-001
              │  Login / SSO    │     (GitHub, Google, Keycloak)
              └────────┬────────┘
                       │ authenticated userId
              ┌────────▼────────┐
              │  Session Router │ ◄── CR-LOGIN-002
              │  per-user proc  │     fork() per user
              └────┬───────┬────┘
                   │       │
           ┌───────▼┐   ┌──▼───────┐
           │ Proc A  │   │ Proc B   │  ← isolated runtimes
           │ user A  │   │ user B   │
           │ data/A/ │   │ data/B/  │
           └───┬─────┘   └──┬───────┘
               │             │
    ┌──────────▼─┐   ┌───────▼──────┐  ◄── CR-LOGIN-003
    │ Dev Server │   │  Dev Server  │      per-user unix account
    │  ~userA/   │   │   ~userB/    │      or Linux namespace
    └────────────┘   └──────────────┘

              ┌─────────────────┐
              │   Admin Panel   │ ◄── CR-LOGIN-004
              │  /admin/users   │
              └─────────────────┘
```
