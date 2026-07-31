# Tasks — Dev Server v1 Bug Fixes

**Nguồn:** [solutions/](../solutions/)  
**Mục tiêu:** Chia nhỏ mỗi giải pháp thành các tác vụ độc lập, AI có thể thực thi từng cái mà không cần context từ cái khác.

---

## Danh Sách Tasks

| ID | Solution | Tiêu đề | File mục tiêu | Phụ thuộc | Est. |
|----|----------|---------|----------------|-----------|------|
| [TASK-DS-001](./TASK-DS-001-relay-ws-handshake.md) | SOL-001 | Fix relay-ws handshake — bỏ re-validate token | `agent.js` | — | ~~15'~~ ✅ |
| [TASK-DS-002](./TASK-DS-002-preflight-detect-agents.md) | SOL-002 | Impl `preflight.detectAgents` + `preflight.check` | `agent.js` | — | ~~45'~~ ✅ |
| [TASK-DS-003](./TASK-DS-003-preflight-git-identity.md) | SOL-002 | Impl `preflight.setGitIdentity` + `preflight.detectGhosttyConfig` | `agent.js` | TASK-DS-002 | ~~20'~~ ✅ |
| [TASK-DS-004](./TASK-DS-004-fs-methods.md) | SOL-002 | Impl `fs.listDirectory` + `fs.stat` + `fs.listWorkspaces` | `agent.js` | — | ~~30'~~ ✅ |
| [TASK-DS-005](./TASK-DS-005-git-clone.md) | SOL-002 | Impl `git.clone` (async với callback) | `agent.js` | — | ~~20'~~ ✅ |
| [TASK-DS-006](./TASK-DS-006-orca-url-env.md) | SOL-003 | Fix orcaUrl từ env var `ORCA_AGENT_WS_URL` | `dev-server-relay-bridge.ts` | — | ~~15'~~ ✅ |
| [TASK-DS-007](./TASK-DS-007-manager-reconnect-status.md) | SOL-004 | DevServerManager: direct-ws → 'connecting' on startup | `dev-server-manager.ts` | — | ~~20'~~ ✅ |
| [TASK-DS-008](./TASK-DS-008-relay-ws-reconnect.md) | SOL-004 | relay-ws auto-reconnect loop trong RelayBridge | `dev-server-relay-bridge.ts` | TASK-DS-007 | ~~45'~~ ✅ |
| [TASK-DS-009](./TASK-DS-009-curl-max-time.md) | SOL-005 | Thêm `--max-time 8` vào curl trong start.sh heredoc | `start-agent-direct.sh` | — | ~~15'~~ ✅ |
| [TASK-DS-010](./TASK-DS-010-service-file-merge.md) | SOL-005 | Merge service file: update `orca-agent.service` + sửa deploy script | `orca-agent.service`, `start-agent-direct.sh` | — | ~~20'~~ ✅ |
| [TASK-DS-011](./TASK-DS-011-keepalive-interval.md) | SOL-005 | Sửa keepalive interval: 8s → 5s | `agent.js` | — | ~~10'~~ ✅ |

---

## Thứ Tự Thực Hiện

```
Sprint 1 — Critical blockers (không phụ thuộc nhau, chạy song song):
  TASK-DS-001  relay-ws handshake fix
  TASK-DS-002  preflight.detectAgents + preflight.check
  TASK-DS-004  fs methods
  TASK-DS-005  git.clone
  TASK-DS-006  orcaUrl env fix
  TASK-DS-009  curl --max-time
  TASK-DS-011  keepalive 5s

Sprint 2 — Sau Sprint 1:
  TASK-DS-003  (sau TASK-DS-002)
  TASK-DS-007  manager reconnect status
  TASK-DS-010  service file merge

Sprint 3 — Sau Sprint 2:
  TASK-DS-008  (sau TASK-DS-007) relay-ws auto-reconnect
```

---

## Format Mỗi Task File

Mỗi TASK file có cấu trúc chuẩn:
1. **Mục tiêu** — một câu ngắn
2. **Context** — files cần đọc trước
3. **Exact change** — đoạn code cần tìm + code thay thế (copy-paste ready)
4. **Verify** — lệnh kiểm tra kết quả
5. **Definition of Done** — checklist rõ ràng
