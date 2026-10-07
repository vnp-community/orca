# AG-CV-TASK-081-08: Dispatcher `quality.*`, bảng method, validate chặt, mã lỗi mới

**From Solution:** [AG-CV-SOL-081-quality-runner-core](../solutions/AG-CV-SOL-081-quality-runner-core.md) mục 5.1,5.5
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-rpc-dispatch-quality.ts`, `quality-method-table.ts` (mới); sửa `agent-rpc-dispatch.ts`, `codeintel-errors.ts` (AG-CV-SOL-001); test `agent-rpc-dispatch-quality.test.ts`
**Depends on:** AG-CV-TASK-081-06, 081-07; AG-CV-SOL-001 (`codeintel-errors.ts`, `codeintel-repo-resolution.ts`)
**Status:** [x] DONE

## Context

Mẫu `agent-rpc-dispatch-misc.ts` (đã đọc): trả `null` nếu method không khớp; import động; `makeError`. Hợp đồng §2.1, §3.2, §5.

## Việc cần làm

1. `dispatchQualityRpc(rpc, config, log, ws, state)`: `null` nếu không bắt đầu `quality.`; bọc try/catch; `CodeIntelError` → `makeError(id, numericCode, message, { code, ...data })`; lỗi lạ → `ServerError` không stack.
2. `QUALITY_METHODS`: `quality.listProfiles|run|runStatus|cancel|results|coverage` (dòng `coverage` do AG-CV-SOL-083 điền; `listProfiles`/`run` nối ở task 17/18). `validate` từng method: `workspaceRoot` bắt buộc; khoá ngoài schema (kể cả `command|argv|args|env|cwd|timeout|tool|shell`) → `CODEINTEL_INVALID_PARAMS data.field`; `_trace` cho phép; `runId` regex; `offset ≥ 0`, `limit 1..500` (mặc định 500), `view ∈ findings|steps|log`, `stepId` bắt buộc khi `view=log`; chuỗi 1..512 ký tự, không NUL/điều khiển, không bắt đầu `-` (kể cả U+FF0D, U+2212 sau NFKC).
3. Mọi method: `ORCA_QUALITY_RUN=off` → `TOOL_UNAVAILABLE reason="quality_disabled"`; `win32` → `reason="unsupported_platform"`; `workspaceRoot` phân giải bằng `resolveWorktreeRoot` (nhóm B task 14), sai → `PATH_NOT_ALLOWED`.
4. Mỗi lần gọi đăng ký notifier hiện hành vào `codeintel-notification-sink.ts` (SOL-004).
5. Thêm 5 mã (`PROFILE_UNKNOWN`, `ENV_NOT_READY`, `RUN_IN_PROGRESS`, `RUN_NOT_FOUND`, `RUN_CANCELLED`) vào `codeintel-errors.ts` với `error.code` số đúng §3.2.
6. `runStatus`/`cancel`/`results`: `runId` không thuộc `workspaceRoot` → `RUN_NOT_FOUND`.
7. `agent-rpc-dispatch.ts`: thêm khối `dispatchQualityRpc` ngay sau khối `dispatchCodeIntelRpc` (trước `MethodNotFound`, dòng ~380); `extractTraceFields` nhánh `quality.` chỉ ghi `workspaceRoot` (cắt 60 ký tự), tên method, `profile`.
8. Timeout agent 10 s mỗi method (trừ `listProfiles` 40 s): quá → `CODEINTEL_TIMEOUT`.

## Kiểm thử

Test phản chiếu: duyệt schema của mọi `validate` khẳng định không chứa khoá cấm; ca tham số lạ/thiếu/kiểu sai; `limit:501`; `view=log` thiếu `stepId`; `runId` lạ; Windows giả; `ORCA_QUALITY_RUN=off`; `MockWs` nhận đúng khung JSON-RPC có `error.data.code`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/agent-rpc-dispatch-quality.test.ts src/relay/__tests__/agent-rpc-dispatch.test.ts`.

## Tiêu chí hoàn thành

- [x] Không method nào chấp nhận khoá cấm; `route()` vẫn trả `MethodNotFound` cho method lạ.
- [x] Không `quality.*` nào tự do spawn; `quality.run` tên lạ không gọi `executeStep` (test spy).

## Rủi ro

Sửa `agent-rpc-dispatch.ts` chạm mọi RPC: chạy toàn bộ `pnpm test` sau thay đổi. Phụ thuộc chữ ký sink của SOL-004 (chưa có).
