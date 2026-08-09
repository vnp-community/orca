# Tasks — Agent HLD v1 Bug Fixes

**Nguồn:** [solutions/](../solutions/)
**Mục tiêu:** Chia nhỏ mỗi giải pháp thành các tác vụ độc lập, AI có thể thực thi từng cái mà không cần context từ cái khác.
**Trạng thái tổng:** ✅ **16/16 task đã implement code** — 2026-08-09. Đã bổ sung `agent/vitest.config.ts` (theo convention `mobile/vitest.config.ts`) + script `"test": "vitest run"` trong `agent/package.json` — **`vitest` giờ chạy được**: 3620/3648 test pass (2 lỗi còn lại có từ trước, không liên quan tới 16 fix này — 1 ở `pty-handler.test.ts`, 1 ở `feature-interactions.test.ts` do agent/ không có thư mục `renderer/`). Trong lúc chạy test thật đã phát hiện + sửa 2 test cũ trong `agent-spawner.test.ts`/`sub-agent-spawner.test.ts` giả định hành vi cũ của `buildAgentEnv` (inject ciphertext) — đã cập nhật theo hành vi mới (fail-fast) của TASK-AG-HLD-002, kèm 4 test case mới cho đúng hành vi đó. 2 task (012, 014) code xong nhưng **chưa smoke-test** (môi trường không có binary `opencode`/`gemini`) — xem cột Trạng Thái. Toàn bộ 16 task cũng đã confirm bằng `npx tsc --noEmit -p agent/tsconfig.json` (98 lỗi pre-existing không đổi) và `node agent/build.mjs` (build thành công, 582 KB).

---

## Danh Sách Tasks

| ID | Solution | Tiêu đề | File mục tiêu | Phụ thuộc | Est. | Trạng Thái |
|----|----------|---------|----------------|-----------|------|------------|
| [TASK-AG-HLD-001](./TASK-AG-HLD-001-ai-complete-resolvedapikey-fallback.md) | SOL-001 | `ai.complete` fallback qua `resolvedApiKey` | `ai-complete-handler.ts`, `agent-rpc-dispatch.ts` | — | 240' | ✅ DONE |
| [TASK-AG-HLD-002](./TASK-AG-HLD-002-buildagentenv-fail-fast.md) | SOL-002 | `buildAgentEnv` fail-fast rõ nghĩa thay vì inject ciphertext | `agent-spawner.ts` | — | 150' | ✅ DONE ⚠️ review bảo mật |
| [TASK-AG-HLD-003](./TASK-AG-HLD-003-remove-ai-provider-handler.md) | SOL-012 | Xoá `ai-provider-handler.ts` (dead code) | `ai-provider-handler.ts` | — | 60' | ✅ DONE |
| [TASK-AG-HLD-004](./TASK-AG-HLD-004-git-identity-registry.md) | SOL-003 | Tạo `git-identity-registry.ts` + wire `preflight.setGitIdentity` | `git-identity-registry.ts` (mới), `preflight-handler.ts` | — | 60' | ✅ DONE |
| [TASK-AG-HLD-005](./TASK-AG-HLD-005-git-handler-use-identity-registry.md) | SOL-003 | `GitHandler.commit()` đọc identity theo `clientId`, truyền qua env | `git-handler.ts` | TASK-AG-HLD-004 | 45' | ✅ DONE |
| [TASK-AG-HLD-006](./TASK-AG-HLD-006-block-global-git-identity-override.md) | SOL-003 | Chặn `git config --global user.name/email` qua `git.exec` | `agent-git-handler.ts` | — | 30' | ✅ DONE |
| [TASK-AG-HLD-007](./TASK-AG-HLD-007-unify-pr-create-handler.md) | SOL-004 | Hợp nhất `git.pr.create` → `handleGitHubPrCreate` (có idempotency) | `agent-rpc-dispatch.ts`, `agent-git-handler.ts` | — | 60' | ✅ DONE |
| [TASK-AG-HLD-008](./TASK-AG-HLD-008-route-github-pr-create-through-breaker.md) | SOL-005 | Route `handleGitHubPrCreate` qua `ghExecFileAsync` (circuit breaker) | `external-api-connector.ts` | TASK-AG-HLD-007 | 90' | ✅ DONE |
| [TASK-AG-HLD-009](./TASK-AG-HLD-009-route-gitlab-mr-create-through-breaker.md) | SOL-005 | Route `handleGitLabMrCreate` qua `glabExecFileAsync` (circuit breaker) | `external-api-connector.ts` | — | 60' | ✅ DONE |
| [TASK-AG-HLD-010](./TASK-AG-HLD-010-wire-missing-rpc-cases.md) | SOL-011 | Wire 3 case còn thiếu (`gitlab.mr.list`, `github.auth.status`, `gitlab.auth.status`) | `agent-rpc-dispatch.ts` | — | 30' | ✅ DONE |
| [TASK-AG-HLD-011](./TASK-AG-HLD-011-agent-spawn-cols-rows.md) | SOL-006 | Thêm `cols`/`rows` optional vào `agent.spawn` | `agent-spawner.ts` | — | 60' | ✅ DONE |
| [TASK-AG-HLD-012](./TASK-AG-HLD-012-resume-opencode-gemini.md) | SOL-007 | Resume OpenCode (`--session`)/Gemini (`--resume`) — cần smoke test CLI thật | `agent-spawner.ts` | — | 90' | 🟡 CODE DONE, smoke test pending |
| [TASK-AG-HLD-013](./TASK-AG-HLD-013-add-trustpreset-to-spawn-request.md) | SOL-008 | Thêm field `trustPreset` vào `AgentSpawnRequest` (interface RPC thật) | `agent-spawner.ts`, `agent-rpc-dispatch.ts` | — | 30' | ✅ DONE |
| [TASK-AG-HLD-014](./TASK-AG-HLD-014-wire-trustpreset-to-buildargs.md) | SOL-008 | Wire `trustPreset` vào `buildArgs` bằng `YOLO_TUI_AGENT_ARGS` | `agent-spawner.ts` | TASK-AG-HLD-013 | 90' | 🟡 CODE DONE, smoke test pending |
| [TASK-AG-HLD-015](./TASK-AG-HLD-015-remove-dead-handleagentexec.md) | SOL-009 | Xoá dead code `handleAgentExec`/TG-001 (giữ `AgentExecHandler` class) | `agent-exec-handler.ts`, `agent-rpc-dispatch.ts` | — | 35' | ✅ DONE |
| [TASK-AG-HLD-016](./TASK-AG-HLD-016-fs-watch-linux-polyfill.md) | SOL-010 | Polyfill recursive `fs.watch` trên Linux | `fs-agent-extensions.ts` | — | 75' | ✅ DONE |

**Tổng effort ước tính:** ~1125 phút (~19 giờ) cho 16 task — đã thực thi 2026-08-09.

---

## Thứ Tự Thực Hiện

```
Sprint 1 — Bảo mật/Correctness, không phụ thuộc nhau (chạy song song):
  TASK-AG-HLD-001  ai.complete resolvedApiKey fallback
  TASK-AG-HLD-002  buildAgentEnv fail-fast
  TASK-AG-HLD-003  xoá ai-provider-handler.ts
  TASK-AG-HLD-004  git-identity-registry.ts (mới)
  TASK-AG-HLD-006  chặn git config --global override
  TASK-AG-HLD-007  hợp nhất PR create handler
  TASK-AG-HLD-009  route GitLab MR qua breaker
  TASK-AG-HLD-010  wire 3 RPC case còn thiếu
  TASK-AG-HLD-011  agent.spawn cols/rows
  TASK-AG-HLD-013  thêm field trustPreset vào AgentSpawnRequest
  TASK-AG-HLD-015  xoá dead code handleAgentExec
  TASK-AG-HLD-016  fs.watch Linux polyfill

Sprint 2 — Sau Sprint 1 (có phụ thuộc):
  TASK-AG-HLD-005  (sau TASK-AG-HLD-004) git-handler dùng identity registry
  TASK-AG-HLD-008  (sau TASK-AG-HLD-007) route GitHub PR create qua breaker
  TASK-AG-HLD-014  (sau TASK-AG-HLD-013) wire trustPreset vào buildArgs

Sprint 3 — Cần smoke test CLI thật trước khi merge (rủi ro cao hơn, làm riêng/cuối):
  TASK-AG-HLD-012  resume OpenCode/Gemini — có phương án dự phòng nếu smoke test fail
  TASK-AG-HLD-014  wire trustPreset — có bước smoke test opencode bắt buộc
```

**Lưu ý về xung đột file khi chạy song song:**
- TASK-AG-HLD-008 và TASK-AG-HLD-009 cùng sửa `external-api-connector.ts` — mỗi task đã tự chứa 2 phương án TÌM/THAY BẰNG cho đoạn import để không phụ thuộc thứ tự chạy trước/sau, nhưng nên review/merge tuần tự để tránh conflict merge thật.
- TASK-AG-HLD-011 và TASK-AG-HLD-012/013/014 đều sửa `agent-spawner.ts` — an toàn chạy song song vì chạm vùng code khác nhau (`AgentSpawnRequest`/PTY size vs `AGENT_SPECS.buildArgs`), nhưng khuyến nghị review theo thứ tự 011 → 012 → 013 → 014 để giảm nguy cơ merge conflict.

---

## Task cần thận trọng đặc biệt (rủi ro cao hơn mức trung bình)

- **TASK-AG-HLD-002**: quyết định kiến trúc bảo mật (fail-fast thay vì tự giải mã Layer-1) — nên review với người phụ trách bảo mật trước khi merge, không chỉ code review thông thường.
- **TASK-AG-HLD-012, TASK-AG-HLD-014**: bắt buộc smoke test với binary CLI thật (`opencode`, `gemini`) trước khi merge — cú pháp flag được suy ra từ code, chưa xác nhận runtime. Có phương án dự phòng (revert + disable UI + hạ tài liệu) nếu smoke test fail cho riêng từng agent.
- **TASK-AG-HLD-008, TASK-AG-HLD-009**: refactor call site gọi `gh`/`glab` — cần test kỹ luồng PR/MR create thật (không chỉ unit test) vì đổi cách exec process.

---

## Format Mỗi Task File

Mỗi TASK file có cấu trúc chuẩn (giống `specs/backend/bugs/dev-server-v1/tasks/`):
1. **Mục tiêu** — một câu ngắn
2. **Context** — files cần đọc trước
3. **Thay Đổi Cần Thực Hiện** — đoạn code cần tìm (TÌM) + code thay thế (THAY BẰNG), copy-paste ready, đã đối chiếu với code thật hiện tại
4. **Verify** — lệnh kiểm tra kết quả
5. **Definition of Done** — checklist `- [ ]` (tất cả đang ở trạng thái chưa làm)

## Tham Khảo

- [Bugs gốc](../00-index.md)
- [Giải pháp đầy đủ](../solutions/00-index.md)
- [Audit tổng `agent/` vs thiết kế](../../../../../audit/agent/agent-vs-design-review.md)
