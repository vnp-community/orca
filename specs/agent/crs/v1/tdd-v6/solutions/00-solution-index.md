# Orca Dev Server Agent — TDD v5 → v6 Solution Index

**Phiên bản:** v6.1 (thực thi TDD v5 đầy đủ: AI Agent CLIs + External API Connectors)
**Ngày:** 2026-07-30
**Nguồn TDD:** [specs/agent/tdd/v5/](../../tdd/v5/)
**Source code:** [src/relay/](../../../../../src/relay/)
**HLD:** [docs/hld/dev-server-architecture.md](../../../../../docs/hld/dev-server-architecture.md)

---

## Tổng quan

Phân tích source code hiện tại trong `src/relay/` cho thấy **phần lớn TDD v5 đã được implement**. Chiến lược v6 tập trung vào:

1. **Tái sử dụng tối đa** — không viết lại code đã hoạt động
2. **Extend không Replace** — thêm `pr.create`, `worktree.*`, health-check real API call
3. **Wire-up còn thiếu** — `agent-session.ts` capabilities list cần cập nhật
4. **New modules** — TDD-AG-12 (`agent-spawner.ts`) và TDD-AG-13 (`external-api-connector.ts`)
5. **Test coverage** — đủ tests theo targets trong TDD

---

## Trạng thái code hiện tại vs TDD yêu cầu

| TDD | Mô tả | Code đã có | Gap còn lại | Solution |
|-----|--------|-----------|------------|---------|
| [TDD-AG-01](../../tdd/v5/01-architecture.md) | Architecture & Process Model | `agent-entry.ts` ✅ | capabilities='ai.providers' thêm | [CR-AG-01](./CR-AG-01-architecture.md) |
| [TDD-AG-02](../../tdd/v5/02-wire-protocol.md) | Binary Wire Protocol | `agent-wire.ts` ✅ | Không | ✅ Done |
| [TDD-AG-03](../../tdd/v5/03-connection-modes.md) | Connection Modes | `agent-connection-direct.ts` + `agent-connection-relay.ts` ✅ | Không | ✅ Done |
| [TDD-AG-04](../../tdd/v5/04-handshake-session.md) | Handshake & Session | `agent-session.ts` ✅ | agentVersion='5.0.0', capabilities add | [CR-AG-04](./CR-AG-04-session.md) |
| [TDD-AG-05](../../tdd/v5/05-tool-registry.md) | Tool Registry | `agent-tool-registry.ts` ✅ | Không | ✅ Done |
| [TDD-AG-06](../../tdd/v5/06-tool-handlers.md) | Tool Handlers | `agent-exec-handler.ts` ✅ | Không | ✅ Done |
| [TDD-AG-07](../../tdd/v5/07-jsonrpc-dispatch.md) | JSON-RPC Dispatch | `agent-rpc-dispatch.ts` ✅ | Không | ✅ Done |
| [TDD-AG-08](../../tdd/v5/08-deployment.md) | Deployment | build scripts ✅ | Không | ✅ Done |
| [TDD-AG-09](../../tdd/v5/09-ai-credential-relay.md) | AI Credential Store | `agent-credential-store.ts` ✅ | healthCheck real API call + deleteCredential | [CR-AG-09](./CR-AG-09-credential-store.md) |
| [TDD-AG-10](../../tdd/v5/10-git-handler-extension.md) | Git Handler Extension | `agent-git-handler.ts` ✅ | `git.pr.create` via gh CLI + worktree.* | [CR-AG-10](./CR-AG-10-git-handler.md) |
| [TDD-AG-11](../../tdd/v5/11-fs-handler-extension.md) | FS Handler Extension | `fs-agent-extensions.ts` ✅ | `fs.stat`, `fs.glob`, `fs.writeFile` | [CR-AG-11](./CR-AG-11-fs-handler.md) |
| [TDD-AG-12](../../tdd/v5/12-agent-spawner.md) | ProfileAware Agent Spawner | ❌ Không có | Cần tạo mới `agent-spawner.ts` | [CR-AG-12](./CR-AG-12-agent-spawner.md) |
| [TDD-AG-13](../../tdd/v5/13-external-api-connectors.md) | External API Connectors | ❌ Không có | Cần tạo mới `external-api-connector.ts` | [CR-AG-13](./CR-AG-13-external-api-connectors.md) |

---

## Code Reuse Map (tái sử dụng là chính)

```
src/relay/                              TDD v5 Coverage
├── agent-config.ts           ──────→  TDD-AG-01 ✅ (REUSE 100%)
├── agent-connection-direct.ts──────→  TDD-AG-03 ✅ (REUSE 100%)
├── agent-connection-relay.ts ──────→  TDD-AG-03 ✅ (REUSE 100%)
├── agent-credential-store.ts ──────→  TDD-AG-09 ✅ (REUSE 95%, +deleteCredential)
│                             ──────→  TDD-AG-12 ✅ (REUSE readDecryptedKey)
├── agent-entry.ts            ──────→  TDD-AG-01 ✅ (REUSE 100%, minor update)
├── agent-exec-handler.ts     ──────→  TDD-AG-06 ✅ (REUSE 100%)
├── agent-git-handler.ts      ──────→  TDD-AG-10 ✅ (REUSE 90%, +pr.create +worktree.*)
│                             ──────→  TDD-AG-13 ✅ (REUSE SHELL_METACHARACTERS pattern)
├── agent-logger.ts           ──────→  ALL       ✅ (REUSE 100%)
├── agent-rpc-dispatch.ts     ──────→  TDD-AG-07 ✅ (REUSE 90%, +13 new routes)
├── agent-session.ts          ──────→  TDD-AG-04 ✅ (REUSE 90%, +capabilities)
├── agent-spawner.ts          ──────→  TDD-AG-12 ❌ NEW (ProfileAwareAgentSpawner)
├── agent-tool-registry.ts    ──────→  TDD-AG-05 ✅ (REUSE 100%)
├── agent-wire.ts             ──────→  TDD-AG-02 ✅ (REUSE 100%)
├── external-api-connector.ts ──────→  TDD-AG-13 ❌ NEW (GitHub + GitLab connectors)
├── fs-agent-extensions.ts    ──────→  TDD-AG-11 ✅ (REUSE 80%, +stat +glob +writeFile)
├── fs-handler-file-read.ts   ──────→  TDD-AG-11 ✅ (REUSE 100%)
├── fs-handler-list-files.ts  ──────→  TDD-AG-11 ✅ (REUSE 100%)
└── git-exec-validator.ts     ──────→  TDD-AG-10 ✅ (REUSE 100%)
```

---

## Tổng số CR cần tạo (v6.1)

| CR | Nội dung | Độ phức tạp | Files thay đổi |
|----|---------|------------|---------------|
| [CR-AG-01](./CR-AG-01-architecture.md) | Architecture — update entry capabilities | Low | 1 modify |
| [CR-AG-04](./CR-AG-04-session.md) | Session — version + capabilities update | Low | 1 modify |
| [CR-AG-09](./CR-AG-09-credential-store.md) | Credential — `deleteCredential` + real healthCheck | Medium | 2 files extend |
| [CR-AG-10](./CR-AG-10-git-handler.md) | Git — `git.pr.create` + `git.worktree.*` | Medium | 2 files extend |
| [CR-AG-11](./CR-AG-11-fs-handler.md) | FS — `fs.stat`, `fs.glob`, `fs.writeFile` | Medium | 2 files extend |
| [CR-AG-12](./CR-AG-12-agent-spawner.md) | **NEW** ProfileAwareAgentSpawner — AI Agent CLI host | **High** | **2 new + 2 modify** |
| [CR-AG-13](./CR-AG-13-external-api-connectors.md) | **NEW** External API Connectors — GitHub + GitLab | **Medium-High** | **2 new + 1 modify** |

**Tổng:** 7 CRs | ~700 lines code mới | ~70% reuse từ code hiện tại

| Module | Lines mới | Reuse |
|--------|----------|-------|
| CR-AG-01/04 (session update) | 4 lines | 100% |
| CR-AG-09 (credential extend) | ~80 lines | 90% |
| CR-AG-10 (git extend) | ~120 lines | 85% |
| CR-AG-11 (fs extend) | ~110 lines | 80% |
| CR-AG-12 (agent-spawner NEW) | ~300 lines | 60% |
| CR-AG-13 (external-api-connector NEW) | ~350 lines | 70% |
