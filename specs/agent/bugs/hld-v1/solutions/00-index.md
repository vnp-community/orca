# Solutions — Agent HLD v1 Bug Fixes

**Module:** `agent/src/relay/**`, `agent/src/main/git/**`, `agent/src/main/ipc/**`
**TDD Refs:** [TDD v5 §09 AI Credential Relay](../../../tdd/v5/09-ai-credential-relay.md), [§10 Git Handler Extension](../../../tdd/v5/10-git-handler-extension.md), [§11 FS Handler Extension](../../../tdd/v5/11-fs-handler-extension.md), [§12 Agent Spawner](../../../tdd/v5/12-agent-spawner.md), [§13 External API Connectors](../../../tdd/v5/13-external-api-connectors.md), [§07 JSON-RPC Dispatch](../../../tdd/v5/07-jsonrpc-dispatch.md)
**Bugs được fix:** BUG-AG-HLD-001 → BUG-AG-HLD-012 (toàn bộ 12/12)
**Status:** ✅ Đã implement 2026-08-09 qua [tasks/](../tasks/) — 16/16 task code done, 14/16 verify đầy đủ bằng typecheck, 2 task (SOL-007, SOL-008) code done nhưng chưa smoke-test (thiếu binary CLI `opencode`/`gemini` trong môi trường thực thi). Xem [tasks/00-index.md](../tasks/00-index.md) để biết trạng thái chi tiết từng task.

---

## Danh Sách Giải Pháp

| ID | Fix Bug | Tiêu đề | Files thay đổi | Priority | Status |
|----|---------|---------|-----------------|----------|--------|
| [SOL-AG-HLD-001](./SOL-AG-HLD-001-ai-complete-resolvedapikey-fallback.md) | BUG-AG-HLD-001 | `ai.complete` fallback qua `resolvedApiKey` thay vì chỉ đọc `process.env` | `ai-complete-handler.ts`, `agent-rpc-dispatch.ts` | 🔴 P0 | 🔴 TODO |
| [SOL-AG-HLD-002](./SOL-AG-HLD-002-buildagentenv-fail-fast-no-resolvedapikey.md) | BUG-AG-HLD-002 | `buildAgentEnv` fail-fast rõ nghĩa thay vì inject ciphertext Layer-1 | `agent-spawner.ts` | 🔴 P0 | 🔴 TODO |
| [SOL-AG-HLD-012](./SOL-AG-HLD-012-remove-dead-ai-provider-handler.md) | BUG-AG-HLD-012 | Xoá `ai-provider-handler.ts` (dead code, claim mã hoá sai) | `ai-provider-handler.ts` (xoá) | 🟢 P2 | 🔴 TODO |
| [SOL-AG-HLD-003](./SOL-AG-HLD-003-per-client-git-identity-env.md) | BUG-AG-HLD-003 | Git identity per-`clientId` registry + env per-call thay vì `git config --global` | `preflight-handler.ts`, `git-handler.ts`, `git-identity-registry.ts` (mới), `agent-git-handler.ts` | 🟠 P1 | 🔴 TODO |
| [SOL-AG-HLD-004](./SOL-AG-HLD-004-unify-pr-create-handler.md) | BUG-AG-HLD-004 | Hợp nhất `git.pr.create`/`github.pr.create` về 1 handler có idempotency-check | `agent-rpc-dispatch.ts`, `agent-git-handler.ts` | 🟠 P1 | 🔴 TODO |
| [SOL-AG-HLD-005](./SOL-AG-HLD-005-route-gh-glab-through-runner-breaker.md) | BUG-AG-HLD-005 | Route `gh`/`glab` PR/MR create qua circuit breaker của `runner.ts` | `external-api-connector.ts`, `runner.ts` | 🟠 P1 | 🔴 TODO |
| [SOL-AG-HLD-011](./SOL-AG-HLD-011-wire-missing-rpc-cases.md) | BUG-AG-HLD-011 | Wire 3 case còn thiếu (`gitlab.mr.list`, `github.auth.status`, `gitlab.auth.status`) | `agent-rpc-dispatch.ts` | 🟢 P2 | 🔴 TODO |
| [SOL-AG-HLD-006](./SOL-AG-HLD-006-agent-spawn-cols-rows.md) | BUG-AG-HLD-006 | Thêm `cols`/`rows` optional vào `agent.spawn`, backward-compatible | `agent-spawner.ts` | 🟡 P1.5 | 🔴 TODO |
| [SOL-AG-HLD-007](./SOL-AG-HLD-007-resume-opencode-gemini-verified-flags.md) | BUG-AG-HLD-007 | Resume OpenCode (`--session <id>`)/Gemini (`--resume <id>`) bằng flag đã verify | `agent-spawner.ts` | 🟡 P1.5 | 🔴 TODO |
| [SOL-AG-HLD-008](./SOL-AG-HLD-008-trustpreset-wiring.md) | BUG-AG-HLD-008 | Wire trust preset bằng `YOLO_TUI_AGENT_ARGS` đã verify sẵn trong codebase | `agent-spawner.ts` | 🟡 P1.5 | 🔴 TODO |
| [SOL-AG-HLD-009](./SOL-AG-HLD-009-remove-dead-handleagentexec.md) | BUG-AG-HLD-009 | Xoá phần dead code `handleAgentExec`/TG-001 trong `agent-exec-handler.ts` (giữ `AgentExecHandler` class — vẫn live) | `agent-exec-handler.ts`, `agent-rpc-dispatch.ts` (docblock) | 🟡 P2 | 🔴 TODO |
| [SOL-AG-HLD-010](./SOL-AG-HLD-010-fs-watch-linux-recursive-polyfill.md) | BUG-AG-HLD-010 | Polyfill recursive watch trên Linux (walk + `fs.watch` per-dir) — không dùng cluster parcel-watcher | `fs-agent-extensions.ts` | 🟢 P2 | 🔴 TODO |

---

## Phát hiện quan trọng phát sinh trong lúc viết giải pháp (khác/chi tiết hơn bug report gốc)

Các agent viết giải pháp đã đọc code thật + xác nhận lại bằng GitNexus, phát hiện thêm một số điều chỉnh so với mô tả ban đầu trong bug report — ghi nhận ở đây để tránh nhầm lẫn khi implement:

- **BUG-AG-HLD-005**: `ghExecFileAsync`/`glabExecFileAsync` trong `runner.ts` hoá ra có **0 caller** trong toàn bộ `agent/` (kể cả `commit-message-text-generation.ts` mà bug report từng nêu là caller duy nhất) — breaker đang **chết hoàn toàn**, không phải "chỉ bảo vệ 1 tính năng phụ" như mô tả gốc. Giải pháp đã điều chỉnh: chỉ cần route `handleGitHubPrCreate`/`handleGitLabMrCreate` qua breaker (Phase 1-2), bỏ Phase 3 (route `git-handler.ts`) vì `GitHandler` không gọi `gh`/`glab` trực tiếp.
- **BUG-AG-HLD-007**: Tìm được bằng chứng cú pháp resume thật trong `agent/src/shared/agent-session-resume.ts` (`getAgentResumeArgv()`) — **Gemini dùng `--resume <id>`** (không phải "Partial" như BL-AG-03 ghi), **OpenCode dùng `--session <id>`** (không phải `resume <id>` như BL-AG-03 ghi). Tài liệu BL-AG-03 sai cú pháp, không chỉ sai mức hỗ trợ.
- **BUG-AG-HLD-008**: Root cause sâu hơn báo cáo gốc — `AgentSpawnRequest` (interface RPC production thật) **không hề có field `trustPreset`**; field này chỉ tồn tại trên `AgentEnvRequest`, một interface không có caller production nào construct. Nghĩa là chỉ sửa `buildAgentEnv` là chưa đủ — cần thêm field vào đúng interface RPC thật trước. Đồng thời xác nhận: args mặc định hiện tại của cả 5 agent **đều an toàn** (không tự động skip permission), nên rủi ro bảo mật giả định trong bug report không có thật ở trạng thái hiện tại.
- **BUG-AG-HLD-009**: Không nên xoá cả file `agent-exec-handler.ts` như đề xuất ban đầu — `class AgentExecHandler` (dòng 130-305) là **code sống**, phục vụ RPC method khác (`agent.execNonInteractive`/`agent.cancelExec`, có 4 caller thật + 2 test suite). Chỉ phần `handleAgentExec()`/`parseAgentExecRequest()`/`AgentExecRequest` (dòng 307-451, gắn với TG-001) mới là dead code cần xoá.
- **BUG-AG-HLD-010**: Đề xuất ban đầu "dùng chung cluster `@parcel/watcher`" bị loại sau khi kiểm tra build pipeline thật — `agent/build.mjs` bundle 1 file duy nhất, không có build step cho `parcel-watcher-process-entry.js`, và entry-path resolver phụ thuộc cứng `electron.app` (không có trong runtime `node out/agent.js`). Giải pháp thay bằng polyfill walk-based tự viết, không cần dependency mới.

## Thứ Tự Triển Khai Đề Xuất

```
Sprint 1 — Bảo mật/Correctness (P0): 🔴 TODO
  SOL-AG-HLD-001 — ai.complete resolvedApiKey fallback
  SOL-AG-HLD-002 — buildAgentEnv fail-fast (quyết định kiến trúc, cần review trước khi code)

Sprint 2 — Git identity & PR/MR reliability (P1): 🔴 TODO
  SOL-AG-HLD-003 — per-client git identity
  SOL-AG-HLD-004 — hợp nhất PR create handler
  SOL-AG-HLD-005 — route gh/glab qua circuit breaker

Sprint 2.5 — Agent spawn UX (P1.5): 🔴 TODO
  SOL-AG-HLD-006 — cols/rows
  SOL-AG-HLD-007 — resume OpenCode/Gemini (cần smoke test CLI thật trước khi merge — xem lưu ý trong file)
  SOL-AG-HLD-008 — trust preset wiring (cần thêm field vào AgentSpawnRequest trước)

Sprint 3 — Dọn dẹp / gap nhỏ (P2): 🔴 TODO
  SOL-AG-HLD-009 — xoá dead code handleAgentExec
  SOL-AG-HLD-010 — fs.watch Linux polyfill
  SOL-AG-HLD-011 — wire 3 RPC case còn thiếu
  SOL-AG-HLD-012 — xoá ai-provider-handler.ts
```

## Verify Chung (áp dụng sau khi implement bất kỳ solution nào)

```bash
# Type check
pnpm run typecheck:node

# Test package agent/
pnpm test -- agent/src/relay

# Build thử agent bundle
pnpm run build:agent

# GitNexus — xác nhận thay đổi chỉ ảnh hưởng đúng phạm vi kỳ vọng
# (bắt buộc theo AGENTS.md / CLAUDE.md trước khi commit)
```

## TDD Alignment

Giải pháp tuân thủ:
- **TDD v5 §09**: `AiCredentialStore` — mô hình double-encryption, "agent never sees plaintext" (cơ sở cho quyết định fail-fast ở SOL-002 thay vì tự giải mã Layer-1).
- **TDD v5 §10, §13**: Git Handler Extension + External API Connectors — per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`, CLI-based invocation, idempotency cho PR/MR create.
- **TDD v5 §11**: FS Handler Extension — polling fallback đã có sẵn phía client theo thiết kế, hỗ trợ giải pháp polyfill ở SOL-010 không phá vỡ contract hiện tại.
- **TDD v5 §12**: Agent Spawner — `AGENT_SPECS`, `buildAgentEnv`, resume-support-by-agent.
- **TDD v5 §07**: JSON-RPC Dispatch — pattern đăng ký `case` trong `route()`.

---

*Toàn bộ 12 file solution đều ở trạng thái tài liệu (`Status: 🔴 TODO`) — chưa có thay đổi nào trong `agent/src/`. Xem từng file để lấy code diff cụ thể trước khi implement.*
