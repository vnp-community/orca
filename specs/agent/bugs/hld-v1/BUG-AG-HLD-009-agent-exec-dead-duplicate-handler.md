# BUG-AG-HLD-009 — `handleAgentExec()` là dead code trùng chức năng với case `agent.exec` đang chạy thật, dễ sửa nhầm

**Mức độ:** 🟡 Medium
**Status:** 🔴 Open
**Module:** `agent/src/relay/agent-exec-handler.ts`, `agent/src/relay/agent-rpc-dispatch.ts`
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "RPC Dispatch & Agent Lifecycle" / "PTY Integration", xác nhận độc lập bởi 2 audit)

---

## Mô tả

Có 2 implementation cho RPC `agent.exec` (non-interactive subprocess execution, dùng bởi Task Graph Step Executor):

1. **Bản đang chạy thật**: inline `case 'agent.exec'` trong `agent-rpc-dispatch.ts:594-624` — nhận `binary`/`args`/`cwd`/`env` trực tiếp từ params, `spawnEnv = { ...process.env, ...extraEnv }` (dòng 623). Không có docblock giải thích vai trò.
2. **Dead code**: `handleAgentExec()` trong `agent/src/relay/agent-exec-handler.ts:355-451` — có docblock đầy đủ ("TG-001: Non-interactive subprocess execution... Called by: StepExecutors.executeAgent()... ProfileAwareAgentSpawner via relay.call('agent.exec', ...)"), tự parse `prompt`/`model`/`trustPreset`, tự `resolveAgentSpec()`, tự build `toolEnv` riêng (chỉ `HOME/PATH/TERM/NO_COLOR/ORCA_TASK_ID/ORCA_WORKTREE_PATH`, **không đọc `params.env`/`extraEnv`** — dòng 379-388).

Grep xác nhận `handleAgentExec` **không được import/dispatch ở bất kỳ đâu** ngoài file định nghĩa nó và test riêng (`agent-exec-handler-test-harness.ts`).

## Hậu quả

- Bản có tài liệu đầy đủ (dễ nhầm là "nguồn sự thật") lại **không chạy**; bản đang chạy thật lại không có docblock giải thích. Bất kỳ ai sửa `handleAgentExec()` để fix bug hoặc thêm tính năng cho `agent.exec` sẽ không thấy tác dụng gì khi test thực tế — dễ dẫn tới việc "fix xong nhưng bug vẫn còn" hoặc double-fix ở 2 nơi.
- Rủi ro thực tế: nếu tương lai có người merge nhầm `case 'agent.exec'` sang gọi `handleAgentExec()` (vì đây trông như bản "đúng chuẩn" hơn), hành vi sẽ đổi ngầm — mất khả năng nhận `env`/`extraEnv` từ caller (dòng 379-388 không đọc các field này).

## Bằng chứng

```
agent/src/relay/agent-rpc-dispatch.ts:594-624 → case 'agent.exec' thật, đang chạy
agent/src/relay/agent-exec-handler.ts:307-311 → docblock mô tả vai trò TG-001, ProfileAwareAgentSpawner
agent/src/relay/agent-exec-handler.ts:355-451 → handleAgentExec(), dead code
agent/src/relay/agent-exec-handler.ts:379-388 → toolEnv tự build, không đọc params.env/extraEnv
```

## Đề xuất fix

Chọn một trong hai:
- Xoá `handleAgentExec()` và toàn bộ `agent-exec-handler.ts` nếu không còn cần, di chuyển docblock mô tả TG-001 sang inline case trong `agent-rpc-dispatch.ts` để không mất ngữ cảnh.
- Hoặc wire `handleAgentExec()` vào dispatcher thật (thay thế inline case) nếu đây mới là implementation đúng ý định thiết kế — nhưng cần fix trước lỗi không đọc `params.env`/`extraEnv`.

## Tham khảo

- Audit: `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md` (mục agent.exec), `audit/agent/pty-ai-cli-vs-design-review.md` §2.4
