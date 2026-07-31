# Solutions — Dev Server v1 Bug Fixes

**Module:** `deploy/dev/agent/agent.js` + `src/main/dev-server/`  
**TDD Refs:** [TDD-05 §4](../../tdd/05-ssh-relay.md), [TDD-13 §3,4](../../tdd/13-dev-server-onboarding.md)  
**Bugs được fix:** BUG-DS-001 → BUG-DS-008  
**Status:** ✅ Tất cả giải pháp đã implement — 2026-07-27

---

## Danh Sách Giải Pháp

| ID | Fix Bug | Tiêu đề | Files thay đổi | Priority | Status |
|----|---------|---------|----------------|----------|--------|
| [SOL-DS-001](./SOL-DS-001-relay-ws-handshake-fix.md) | BUG-DS-001 | Fix relay-ws handshake token validation | `agent.js` | 🔴 P0 | ✅ DONE |
| [SOL-DS-002](./SOL-DS-002-agent-relay-rpc-methods.md) | BUG-DS-002 | Implement relay RPC methods trong agent | `agent.js` | 🔴 P0 | ✅ DONE |
| [SOL-DS-003](./SOL-DS-003-orca-url-config.md) | BUG-DS-003 | Fix orcaUrl từ env config | `dev-server-relay-bridge.ts` | 🟠 P1 | ✅ DONE |
| [SOL-DS-004](./SOL-DS-004-reconnect-status.md) | BUG-DS-004,005 | Reconnect status + relay-ws auto-reconnect | `dev-server-manager.ts`, `dev-server-relay-bridge.ts` | 🟠 P1 | ✅ DONE |
| [SOL-DS-005](./SOL-DS-005-daemon-hardening.md) | BUG-DS-006,007,008 | Daemon hardening: curl timeout, service merge, keepalive | `start-agent-direct.sh`, `orca-agent.service`, `agent.js` | 🟡 P2 | ✅ DONE |

---

## Implementation Summary

### Files Đã Thay Đổi

| File | Thay đổi |
|------|----------|
| `deploy/dev/agent/agent.js` | +245 dòng: BUG-DS-001 handshake fix, helpers, 10 RPC methods mới, keepalive 5s |
| `src/main/dev-server/dev-server-relay-bridge.ts` | orcaUrl env var, relay-ws auto-reconnect (+95 dòng), 2 private fields |
| `src/main/dev-server/dev-server-manager.ts` | Constructor startup status, `restoreConnections()` method (+28 dòng) |
| `deploy/dev/scripts/start-agent-direct.sh` | curl `--max-time 8 --retry 2` |
| `deploy/dev/agent/orca-agent.service` | File logging, `TimeoutStopSec=15s` |
| `deploy/dev/scripts/connect-agent.sh` | `agent_logs()` với file+journald fallback |

### RPC Methods Đã Thêm Vào agent.js

```
preflight.detectAgents        ← BUG-DS-002
preflight.check               ← BUG-DS-002
preflight.setGitIdentity      ← BUG-DS-002
preflight.detectGhosttyConfig ← BUG-DS-002
preflight.detectWindowsTerminalCapabilities ← BUG-DS-002
fs.listDirectory              ← BUG-DS-002
fs.stat                       ← BUG-DS-002
fs.listWorkspaces             ← BUG-DS-002
git.clone (async)             ← BUG-DS-002
```

### Verify Commands

```bash
# TypeScript type check (server files):
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit --project tsconfig.json

# Syntax check (agent.js):
/Users/binhnt/.nvm/versions/node/v24.13.1/bin/node --check deploy/dev/agent/agent.js

# Deploy lên dev server (172.20.2.31):
bash deploy/dev/scripts/connect-agent.sh --deploy
```

---

## Thứ Tự Thực Hiện (Đã Hoàn Thành)

```
Sprint 1 — Unblock core functionality: ✅
  SOL-DS-001  (15 phút) — relay-ws handshake fix        → TASK-DS-001
  SOL-DS-002  (2 giờ)   — agent relay RPC methods       → TASK-DS-002, 003, 004, 005

Sprint 2 — UX và reliability: ✅
  SOL-DS-003  (15 phút) — orcaUrl config                → TASK-DS-006
  SOL-DS-004  (65 phút) — reconnect + auto-reconnect    → TASK-DS-007, 008

Sprint 3 — Operational: ✅
  SOL-DS-005  (45 phút) — daemon hardening              → TASK-DS-009, 010, 011
```

---

## TDD Alignment

Giải pháp tuân thủ:
- **TDD-13 §3**: `DevServerManager` API contract (connect, disconnect, getRelay)
- **TDD-13 §4**: IPC handler interface (`onboarding.*`, `repoRemote.*`)
- **TDD-05 §5**: `SshChannelMultiplexer` request/response protocol
- **TDD-05 §6**: `relay-protocol.ts` — `KEEPALIVE_SEND_MS = 5_000`, `TIMEOUT_MS = 20_000`
