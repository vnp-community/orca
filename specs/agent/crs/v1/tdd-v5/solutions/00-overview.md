# Agent TDD v5 — Implementation Solutions

**Ngày:** 2026-07-28  
**Phạm vi:** Triển khai `specs/agent/tdd/` vào `src/relay/agent-*.ts`  
**Mục tiêu:** Tích hợp agent vào monorepo Orca — chung TypeScript, esbuild, Vitest với backend  
**Trạng thái:** ✅ **HOÀN THÀNH** — Tất cả 12 solutions đã được triển khai

---

## Tổng quan giải pháp

> Agent v2.1 không còn là standalone Node.js script. Nó là một **TypeScript module** trong `src/relay/`, được build bằng esbuild, test bằng Vitest, typecheck bằng `tsc`.

### Phân rã công việc

| Solution | File(s) | Mức độ | Phụ thuộc | Trạng thái |
|---------|---------|--------|----------|-----------|
| [SOL-01](./01-build-integration.md) | `config/scripts/build-relay.mjs` EXTEND | 🟡 Trung bình | Không | ✅ DONE |
| [SOL-02](./02-agent-wire.md) | `src/relay/agent-wire.ts` NEW | 🟢 Đơn giản | SOL-01 | ✅ DONE |
| [SOL-03](./03-agent-config.md) | `src/relay/agent-config.ts` NEW | 🟢 Đơn giản | Không | ✅ DONE |
| [SOL-04](./04-agent-tool-registry.md) | `src/relay/agent-tool-registry.ts` NEW | 🟡 Trung bình | SOL-03 | ✅ DONE |
| [SOL-05](./05-agent-rpc-dispatch.md) | `src/relay/agent-rpc-dispatch.ts` NEW | 🟡 Trung bình | SOL-04 | ✅ DONE |
| [SOL-06](./06-agent-session.md) | `src/relay/agent-session.ts` NEW | 🟡 Trung bình | SOL-02, SOL-05 | ✅ DONE |
| [SOL-07](./07-agent-connections.md) | `agent-connection-direct.ts` + `agent-connection-relay.ts` NEW | 🟡 Trung bình | SOL-06 | ✅ DONE |
| [SOL-08](./08-agent-entry.md) | `src/relay/agent-entry.ts` NEW | 🟢 Đơn giản | SOL-07 | ✅ DONE |
| [SOL-09](./09-git-handler.md) | `src/relay/git-handler.ts` NEW | 🟡 Trung bình | SOL-05 | ✅ DONE |
| [SOL-10](./10-credential-store.md) | `src/relay/agent-credential-store.ts` NEW | 🔴 Phức tạp | SOL-05 | ✅ DONE |
| [SOL-11](./11-fs-extensions.md) | `src/relay/fs-agent-extensions.ts` NEW | 🟡 Trung bình | SOL-05 | ✅ DONE |
| [SOL-12](./12-tests.md) | `src/relay/__tests__/agent-*.test.ts` NEW | 🔴 Phức tạp | SOL-01~11 | ✅ DONE |

---

## Execution Order

```
Phase 1 — Foundation (không phụ thuộc nhau, làm song song):
  SOL-01  Build integration (build-relay.mjs + package.json)
  SOL-03  Agent config (typed env vars)

Phase 2 — Core protocol (phụ thuộc Phase 1):
  SOL-02  Agent wire (binary frame codec)
  SOL-04  Tool registry (typed tool definitions)

Phase 3 — Session & dispatch (phụ thuộc Phase 2):
  SOL-05  RPC dispatch (JSON-RPC router)
  SOL-06  Session handler (handshake + keepalive)

Phase 4 — Entry point (phụ thuộc Phase 3):
  SOL-07  Connection modes (direct-ws + relay-ws)
  SOL-08  Agent entry (main function)

Phase 5 — v5.0 Extensions (phụ thuộc Phase 4):
  SOL-09  Git handler (whitelisted git.exec + git.execStream)
  SOL-10  Credential store (AI API key AES-256-GCM)
  SOL-11  FS extensions (readDir/readFile/grep via existing handlers)

Phase 6 — Tests (chạy song song với mỗi Phase):
  SOL-12  Vitest test files cho mọi module
```

---

## Key Constraints (bất biến)

| Constraint | Lý do |
|-----------|-------|
| `shell: false` trong mọi spawn | Ngăn shell injection |
| Import `src/shared/agent-wire-protocol.ts` | Shared types với Orca Server |
| Import `src/relay/agent-exec-handler.ts` | REUSE existing — không viết lại |
| Import `src/relay/fs-handler-*.ts` | REUSE existing FS logic |
| `WireState` per-connection object | Testable, không còn module-level globals |
| `AgentConfig` injected vào mọi handler | Dependency injection pattern |
| `MessageType.Regular/KeepAlive` | Không dùng magic numbers `0x01`, `0x09` |
| File mode `0o600` cho credential files | Security hardening |
| Git subcommand whitelist enforced | Ngăn arbitrary git execution từ UI |

---

## Cấu trúc thư mục sau khi triển khai

```
src/relay/
├── agent-entry.ts              ← [SOL-08] Entry point
├── agent-config.ts             ← [SOL-03] Typed config
├── agent-wire.ts               ← [SOL-02] Binary frame codec
├── agent-session.ts            ← [SOL-06] Handshake + keepalive
├── agent-tool-registry.ts      ← [SOL-04] Tool definitions
├── agent-rpc-dispatch.ts       ← [SOL-05] JSON-RPC router + v5.0 methods
├── agent-connection-direct.ts  ← [SOL-07] Mode 1: Agent → Orca
├── agent-connection-relay.ts   ← [SOL-07] Mode 2: Orca → Agent
├── agent-credential-store.ts   ← [SOL-10] AI key AES-256-GCM (v5.0)
├── git-handler.ts              ← [SOL-09] git.exec + git.execStream (v5.0)
├── agent-exec-handler.ts       ← [REUSE] Already exists — use as-is
├── fs-handler-file-read.ts     ← [REUSE] Already exists
├── fs-handler-list-files.ts    ← [REUSE] Already exists
├── fs-handler-rg-availability.ts ← [REUSE] Already exists
└── __tests__/
    ├── agent-wire.test.ts      ← [SOL-12]
    ├── agent-config.test.ts    ← [SOL-12]
    ├── agent-tool-registry.test.ts ← [SOL-12]
    ├── agent-rpc-dispatch.test.ts  ← [SOL-12]
    ├── agent-session.test.ts       ← [SOL-12]
    ├── agent-connection-direct.test.ts ← [SOL-12]
    ├── agent-connection-relay.test.ts  ← [SOL-12]
    ├── git-handler.test.ts         ← [SOL-12]
    └── agent-credential-store.test.ts  ← [SOL-12]

config/scripts/
└── build-relay.mjs             ← [SOL-01] EXTEND with agent entry

out/relay/
└── agent.js                    ← Build output (deploy to dev server)
```
