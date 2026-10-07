# AG-CV-TASK-081-18: Nối `quality.run`: kiểm tham số, kế hoạch, preflight, `ENV_NOT_READY`, gửi vào run manager

**From Solution:** [AG-CV-SOL-081-quality-profile-catalog-and-preflight](../solutions/AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) mục 5.4
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-run-start.ts` (mới), `.test.ts` (mới); điền dòng `quality.run` trong `quality-method-table.ts`
**Depends on:** AG-CV-TASK-081-06, 081-08, 081-15, 081-16
**Status:** [x] DONE

## Context

Hợp đồng §5.2: trả ngay < 1 s; `PROFILE_UNKNOWN`; `ENV_NOT_READY` chỉ khi **mọi** bước thiếu; thiếu một phần → bước `env_not_ready` vẫn nằm trong kế hoạch.

## Việc cần làm

1. `startRun(params, ctx)`: `resolveWorktreeRoot` → `isSafeBaseRef` → `planRun` → preflight từng bước (dùng cache) → bước thiếu đánh `env_not_ready` (không spawn); nếu tất cả bước thiếu → `CODEINTEL_ENV_NOT_READY data.missing[]` (và `reason` khi đồng nhất, vd `tool_incompatible`) và **không** tạo run.
2. Gọi `runManager.submit` và trả `{ runId, state, profile, scope, steps:[{id,title}], headCommit, dirtyFingerprint, queuePosition }` trong < 1 s (chạy nền; không `await` bước đầu).
3. `RUN_IN_PROGRESS` từ manager chuyển nguyên `runId`, `reason`.
4. Không bao giờ chạy preflight nặng (native `--check-only` 15 s) đồng bộ trước khi trả lời nếu cache nguội: giới hạn tổng 800 ms; quá → để manager preflight trong nền và đánh `env_not_ready` khi đến bước đó.

## Kiểm thử

Kịch bản: tên lạ (spy `executeStep` không gọi), mọi bước thiếu (ENV_NOT_READY), một phần thiếu (run chạy), worktree bận, queue đầy, `base_required`. Đo thời gian trả lời với preflight giả chậm 5 s < 1 s. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-run-start.test.ts`.

## Tiêu chí hoàn thành

- [x] `quality.run` trả < 1 s trong mọi kịch bản; tên lạ không spawn.
- [x] `ENV_NOT_READY` đúng điều kiện (mọi bước).

## Rủi ro

Quy tắc 800 ms là lựa chọn của solution (chưa đo); nếu cache lạnh, `ready` có thể bị đánh `env_not_ready` muộn trong run thay vì lúc nhận.
