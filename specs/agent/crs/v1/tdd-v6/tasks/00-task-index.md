# Task Index — TDD v5 → v6 Implementation Tasks

**Phiên bản:** v6.1
**Ngày:** 2026-07-30
**Source solutions:** [../solutions/](../solutions/)
**Source code:** [src/relay/](../../../../../src/relay/)

> ✅ **ALL 12 TASKS COMPLETED** — 2026-07-30T18:29
> 🧪 **Total test coverage:** 214 tests passing across 5 test files

---

## Tổng quan

Mỗi task được thiết kế để AI Agent **thực thi độc lập** mà không cần context từ task khác.
Mỗi task file chứa: mô tả rõ ràng, file cần đọc, code cần viết/sửa chính xác, verify steps.

## Thứ tự thực thi khuyến nghị

```
PHASE 1 — Simple updates (không dependency)
  TASK-01 → agent-session.ts (2 dòng)
  TASK-02 → agent-credential-store.ts (extend)

PHASE 2 — Extend existing modules
  TASK-03 → agent-git-handler.ts (extend)
  TASK-04 → fs-agent-extensions.ts (extend)

PHASE 3 — New modules (independent)
  TASK-05 → external-api-connector.ts (new file)
  TASK-06 → agent-spawner.ts (new file)

PHASE 4 — Wire-up routes (sau khi PHASE 1-3 xong)
  TASK-07 → agent-rpc-dispatch.ts (add all new routes)

PHASE 5 — Tests
  TASK-08 → agent-credential-store tests
  TASK-09 → agent-git-handler tests
  TASK-10 → fs-agent-extensions tests
  TASK-11 → external-api-connector tests
  TASK-12 → agent-spawner tests
```

## Dependency Map

```
TASK-01 (session) ──────────────────────────────────→ TASK-07
TASK-02 (credential) ──────→ TASK-08 (test)
                        └──→ TASK-07 (dispatch route)
                        └──→ TASK-06 (spawner imports readDecryptedKey)
TASK-03 (git) ─────────────→ TASK-09 (test)
                        └──→ TASK-07 (dispatch routes)
TASK-04 (fs) ──────────────→ TASK-10 (test)
                        └──→ TASK-07 (dispatch routes)
TASK-05 (external-api) ────→ TASK-11 (test)
                        └──→ TASK-07 (dispatch routes)
TASK-06 (spawner) ─────────→ TASK-12 (test)
                        └──→ TASK-07 (dispatch routes)
TASK-07 (dispatch) ← ALL previous tasks must be done first
```

## Task List

| Task | File thay đổi | Phase | Tests | Status |
|------|--------------|-------|-------|--------|
| [TASK-01](./TASK-01-session-update.md) | `agent-session.ts` MODIFY | 1 | — | ✅ DONE |
| [TASK-02](./TASK-02-credential-extend.md) | `agent-credential-store.ts` EXTEND | 1 | — | ✅ DONE |
| [TASK-03](./TASK-03-git-handler-extend.md) | `agent-git-handler.ts` EXTEND | 2 | — | ✅ DONE |
| [TASK-04](./TASK-04-fs-handler-extend.md) | `fs-agent-extensions.ts` EXTEND | 2 | — | ✅ DONE |
| [TASK-05](./TASK-05-external-api-connector.md) | `external-api-connector.ts` NEW | 3 | — | ✅ DONE |
| [TASK-06](./TASK-06-agent-spawner.md) | `agent-spawner.ts` NEW | 3 | — | ✅ DONE |
| [TASK-07](./TASK-07-rpc-dispatch-routes.md) | `agent-rpc-dispatch.ts` EXTEND (+16 routes) | 4 | — | ✅ DONE |
| [TASK-08](./TASK-08-test-credential.md) | `__tests__/agent-credential-store.test.ts` | 5 | **29/29** ✅ | ✅ DONE |
| [TASK-09](./TASK-09-test-git-handler.md) | `__tests__/agent-git-handler.test.ts` | 5 | **58/58** ✅ | ✅ DONE |
| [TASK-10](./TASK-10-test-fs-extensions.md) | `__tests__/fs-agent-extensions.test.ts` | 5 | **42/42** ✅ | ✅ DONE |
| [TASK-11](./TASK-11-test-external-api.md) | `__tests__/external-api-connector.test.ts` NEW | 5 | **39/39** ✅ | ✅ DONE |
| [TASK-12](./TASK-12-test-agent-spawner.md) | `__tests__/agent-spawner.test.ts` NEW | 5 | **46/46** ✅ | ✅ DONE |

## Files Created / Modified

### New Files
- `src/relay/external-api-connector.ts` — GitHub & GitLab CLI connectors (~340 lines)
- `src/relay/agent-spawner.ts` — ProfileAware PTY spawner with state machine (~360 lines)
- `src/relay/__tests__/external-api-connector.test.ts` — 39 tests
- `src/relay/__tests__/agent-spawner.test.ts` — 46 tests

### Modified Files
- `src/relay/agent-session.ts` — version 5.0.0, expanded capabilities
- `src/relay/agent-credential-store.ts` — +deleteCredential, +readDecryptedKey, real healthCheck
- `src/relay/agent-git-handler.ts` — +git.pr.create, +worktree.{list,add,remove}
- `src/relay/fs-agent-extensions.ts` — +fs.stat, +fs.glob, +fs.writeFile
- `src/relay/agent-rpc-dispatch.ts` — 9 routes → 25 routes
- `src/relay/__tests__/agent-credential-store.test.ts` — +13 new tests (29 total)
- `src/relay/__tests__/agent-git-handler.test.ts` — +34 new tests (58 total)
- `src/relay/__tests__/fs-agent-extensions.test.ts` — +22 new tests (42 total)
## Tổng quan

Mỗi task được thiết kế để AI Agent **thực thi độc lập** mà không cần context từ task khác.
Mỗi task file chứa: mô tả rõ ràng, file cần đọc, code cần viết/sửa chính xác, verify steps.

## Thứ tự thực thi khuyến nghị

```
PHASE 1 — Simple updates (không dependency)
  TASK-01 → agent-session.ts (2 dòng)
  TASK-02 → agent-credential-store.ts (extend)

PHASE 2 — Extend existing modules
  TASK-03 → agent-git-handler.ts (extend)
  TASK-04 → fs-agent-extensions.ts (extend)

PHASE 3 — New modules (independent)
  TASK-05 → external-api-connector.ts (new file)
  TASK-06 → agent-spawner.ts (new file)

PHASE 4 — Wire-up routes (sau khi PHASE 1-3 xong)
  TASK-07 → agent-rpc-dispatch.ts (add all new routes)

PHASE 5 — Tests
  TASK-08 → agent-credential-store tests
  TASK-09 → agent-git-handler tests
  TASK-10 → fs-agent-extensions tests
  TASK-11 → external-api-connector tests
  TASK-12 → agent-spawner tests
```

## Dependency Map

```
TASK-01 (session) ──────────────────────────────────→ TASK-07
TASK-02 (credential) ──────→ TASK-08 (test)
                        └──→ TASK-07 (dispatch route)
                        └──→ TASK-06 (spawner imports readDecryptedKey)
TASK-03 (git) ─────────────→ TASK-09 (test)
                        └──→ TASK-07 (dispatch routes)
TASK-04 (fs) ──────────────→ TASK-10 (test)
                        └──→ TASK-07 (dispatch routes)
TASK-05 (external-api) ────→ TASK-11 (test)
                        └──→ TASK-07 (dispatch routes)
TASK-06 (spawner) ─────────→ TASK-12 (test)
                        └──→ TASK-07 (dispatch routes)
TASK-07 (dispatch) ← ALL previous tasks must be done first
```

## Task List

| Task | File thay đổi | Phase | Lines | Complexity |
|------|--------------|-------|-------|-----------|
| [TASK-01](./TASK-01-session-update.md) | `agent-session.ts` MODIFY | 1 | 2 | Low |
| [TASK-02](./TASK-02-credential-extend.md) | `agent-credential-store.ts` EXTEND | 1 | ~80 | Medium |
| [TASK-03](./TASK-03-git-handler-extend.md) | `agent-git-handler.ts` EXTEND | 2 | ~120 | Medium |
| [TASK-04](./TASK-04-fs-handler-extend.md) | `fs-agent-extensions.ts` EXTEND | 2 | ~110 | Medium |
| [TASK-05](./TASK-05-external-api-connector.md) | `external-api-connector.ts` NEW | 3 | ~350 | Medium-High |
| [TASK-06](./TASK-06-agent-spawner.md) | `agent-spawner.ts` NEW | 3 | ~300 | High |
| [TASK-07](./TASK-07-rpc-dispatch-routes.md) | `agent-rpc-dispatch.ts` EXTEND | 4 | ~80 | Medium |
| [TASK-08](./TASK-08-test-credential.md) | `__tests__/agent-credential-store.test.ts` NEW | 5 | ~150 | Medium |
| [TASK-09](./TASK-09-test-git-handler.md) | `__tests__/agent-git-handler.test.ts` NEW | 5 | ~120 | Medium |
| [TASK-10](./TASK-10-test-fs-extensions.md) | `__tests__/fs-agent-extensions.test.ts` NEW | 5 | ~150 | Medium |
| [TASK-11](./TASK-11-test-external-api.md) | `__tests__/external-api-connector.test.ts` NEW | 5 | ~130 | Medium |
| [TASK-12](./TASK-12-test-agent-spawner.md) | `__tests__/agent-spawner.test.ts` NEW | 5 | ~160 | Medium |
