# AG-CV-TASK-081-09: Thông báo `quality.progress`/`quality.finished` và capability `quality`

**From Solution:** [AG-CV-SOL-081-quality-runner-core](../solutions/AG-CV-SOL-081-quality-runner-core.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-progress-emitter.ts` (mới) + test; sửa `agent-session-capabilities.ts`, `agent/src/shared/agent-wire-protocol.ts`
**Depends on:** AG-CV-TASK-081-06, 081-08
**Status:** [ ] TODO

## Context

Hợp đồng §6.3, §6.4 (có `workspaceRoot`, PQ-17), §1.3. Thông báo qua `codeintel-notification-sink.ts`; WS chưa mở thì bỏ. Run tiếp tục khi WS đóng; `agent-session.stop()` không đụng run (đã đọc `agent-session.ts:226`).

## Việc cần làm

1. `createProgressEmitter(deps: { emit(method, params): void; now; minIntervalMs=1000 })`: `onStage(run, stage, stepIndex)` luôn phát khi đổi `stage`; sự kiện trùng stage bị gộp ≤ 1/giây/run; `message` ≤ 200 ký tự đã che; `percent = floor(100 * completedSteps / stepCount)` hoặc `null` khi `stepCount=0`; `stage = "step:<id>"`, `stepIndex` 1-based.
2. `emitFinished(run)`: `{runId, workspaceRoot, status, summary, steps:[{id,status,exitCode,durationMs,truncated}], headCommit, dirtyFingerprint, workTreeChangedDuringRun, startedAt, finishedAt, errorCode}`; luôn phát đúng một lần.
3. Capability `quality`: thêm vào `buildCapabilities` khi không `win32`, `ORCA_QUALITY_RUN !== "off"` và có ≥ 1 binary catalog tìm thấy trên `qualityToolPath` (kiểm nhẹ `fs.access`, ≤ 200 ms, không chạy `--version`); KHÔNG thêm vào `STATIC_CAPABILITIES_FALLBACK`. Nới `AgentCapability` thành `string` (hoặc thêm literal) mà không đổi hành vi hiện có.
4. Sau khi nối lại, backend gọi `runStatus`; không có cơ chế phát lại thông báo.

## Kiểm thử

Test: đồng hồ giả throttle; đổi stage luôn phát; `finished` một lần; `percent` null khi 0 bước; WS đóng (sink không notifier) không ném; capability có/không theo env và nền tảng; `STATIC_CAPABILITIES_FALLBACK` không đổi. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-progress-emitter.test.ts src/relay/__tests__/agent-session.test.ts`.

## Tiêu chí hoàn thành

- [ ] Payload khớp ví dụ hợp đồng §6.3/6.4 (có `workspaceRoot`).
- [ ] Capability `quality` đúng điều kiện; test `agent-session` cũ vẫn xanh.

## Rủi ro

Điều kiện capability mức máy chủ là đề xuất (câu hỏi mở 4 của solution). Run `interrupted` sau restart không có `quality.finished` (không có notifier); backend phát hiện qua `runStatus`.
