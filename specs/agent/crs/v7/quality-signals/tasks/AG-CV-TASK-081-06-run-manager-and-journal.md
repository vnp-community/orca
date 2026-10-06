# AG-CV-TASK-081-06: Máy trạng thái run, khoá worktree, hàng đợi, nhật ký (`quality-run-manager.ts`)

**From Solution:** [AG-CV-SOL-081-quality-runner-core](../solutions/AG-CV-SOL-081-quality-runner-core.md) mục 5.4
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-run-manager.ts`, `quality-run-id.ts`, `quality-run-journal.ts`, `quality-limits.ts` (mới) và test tương ứng
**Depends on:** AG-CV-TASK-081-04, 081-05
**Status:** [ ] TODO

## Context

Hợp đồng §5.2-5.4. Manager nhận kế hoạch đã dựng (`RunPlan`, nhóm B task 15/18) qua interface nên test được bằng kế hoạch giả.

## Việc cần làm

1. `quality-limits.ts`: `maxConcurrentRuns=1`, `queueMax=ORCA_QUALITY_QUEUE_MAX` (4), `runTimeoutMs=2_700_000`, `resultTtlMs=ORCA_QUALITY_RESULT_TTL_MS` (3_600_000), `runsPerWorktree=20`; giá trị sai bị bỏ + log.
2. `quality-run-id.ts`: `qr_` + ULID (Crockford base32, `crypto.randomBytes`); hợp lệ theo `^qr_[0-9A-Za-z]{8,40}$`.
3. `createRunManager(deps: { executeStep; parse: StepParser | undefined; heavyGate; journal; emit; now; fingerprintOf(root): Promise<string> })`: `submit(plan): { runId, state, queuePosition, dirtyFingerprint } | throws RUN_IN_PROGRESS`; `getStatus(root, runId)`, `cancel(root, runId)`, `list`.
4. Khoá theo `realpath(workspaceRoot)`; cùng worktree còn run `queued|running|cancelling` → lỗi `worktree_busy` kèm `runId`; tổng `queued` ≥ `queueMax` → `queue_full`.
5. Chạy tuần tự các bước; bước `env_not_ready` có sẵn trong kế hoạch được ghi nhận, không spawn. Trạng thái bước: `passed|findings|failed|timeout|cancelled|skipped|env_not_ready` theo hợp đồng §5.3; run `failed` nếu có bước `failed|timeout|env_not_ready`; có phát hiện không làm `failed`.
6. `runTimeoutMs`: diệt bước hiện tại, bước còn lại `skipped` (`skipReason:"run_timeout"`), run `failed`, `errorCode:"CODEINTEL_TIMEOUT"`.
7. `dirtyFingerprint` đầu/cuối; khác → `workTreeChangedDuringRun:true`.
8. Journal `~/.orca/quality/runs/<runId>.json` (thư mục `0700`, tệp `0600`, ghi nguyên tử bằng tệp tạm + `rename`), chứa `runId, workspaceRoot(realpath), state, startedAt, finishedAt, steps[], pgid`; giữ ≥ 50 bản (xoá cũ nhất); khi agent khởi động, run còn `running|queued|cancelling` → `interrupted` (không chạy lại, không diệt pgid mồ côi, log cảnh báo).
9. `cancel`: idempotent; run đã kết thúc trả trạng thái cuối; đang chạy → `cancelling`, abort executor, kết thúc `cancelled`, giữ phát hiện đã parse (`partial`). Dọn thư mục tạm khi hết TTL; khi khởi động xoá `orca-quality-*` cũ hơn 2 giờ.

## Kiểm thử

Test dùng executor giả: tuần tự, khoá worktree, hàng đợi 4+1 → `queue_full`, huỷ giữa chừng, timeout run, `interrupted` sau khởi động lại (đọc journal thư mục tạm), giữ ≥ 50 bản, fingerprint đổi → cờ, `runId` của worktree khác → `RUN_NOT_FOUND`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-run-manager.test.ts src/relay/quality-run-journal.test.ts`.

## Tiêu chí hoàn thành

- [ ] Mọi chuyển trạng thái hợp lệ có test; hai run cùng worktree không bao giờ chạy song song.
- [ ] Journal không chứa phát hiện/log.

## Rủi ro

Hành vi khi hai manager cùng ghi journal (nhiều agent) chưa xử lý. `HOME` không ghi được → log và chạy không journal (run vẫn hoạt động).
