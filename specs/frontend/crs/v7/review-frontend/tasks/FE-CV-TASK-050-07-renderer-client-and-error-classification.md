# FE-CV-TASK-050-07: `codeIntelClient` và `classifyCodeIntelError`

**From Solution:** [FE-CV-SOL-050-types-and-runtime-bridge](../solutions/FE-CV-SOL-050-types-and-runtime-bridge.md) mục 4.3
**Priority:** P0
**Area:** frontend / renderer runtime
**File:** `frontend/src/renderer/src/runtime/runtime-code-intel-client.ts` (mới), `runtime/code-intel-error-classification.ts` (mới), tests cùng tên
**Depends on:** FE-CV-TASK-050-03, FE-CV-TASK-050-04, FE-CV-TASK-050-09
**Status:** [x] DONE (verified 2026-10-07: code-intel-client.test + code-intel-error-classification.test 38 pass; client resolves selector (no network when unsupported), injects projectId/worktreeId, LocalCodeIntelError carries kind. File names differ from spec (code-intel-client.ts))

## Context

- `isConnectivityLikeRpcError` (`store/slices/connectivity-status.ts:72`), `maybeTriggerConnectivityPollAfterRpcFailure` (cùng file; **đọc chữ ký trước**).
- U1 (một object), U3 (khoá cấm), §2.4 (giới hạn byte), PQ-04 (`{projectId, worktreeId}`).

## Việc cần làm

1. `classifyCodeIntelError(response|error): CodeIntelRpcError {kind, code, message, data, retryable}`: tách tiền tố bằng 050-03; không có tiền tố ⇒ `error.code` thô: `method_not_found` -> `unsupported`, `forbidden` -> `forbidden`, kết nối -> `offline`, còn lại `unknown`. `error.code === 'internal'` không phải mã.
2. `codeIntelClient.call(worktreeId, method, params, opts)`: `resolveCodeIntelSelector` (`unsupported` ⇒ ném ngay, không gọi); từ chối khoá cấm U3 và `Array` params; kiểm byte UTF-8 `JSON.stringify(args0)` theo `CODE_INTEL_METHOD_LIMITS` ⇒ `validation` cục bộ; gọi bridge; ok ⇒ `parseCodeIntelEnvelope` (view) hoặc trả nguyên; `AbortSignal` bỏ kết quả (không huỷ RPC).
3. `offline` ⇒ gọi `maybeTriggerConnectivityPollAfterRpcFailure`.
4. Danh sách kênh **không** có phong bì: `status`, `reindex`, `reindexStatus`, `reviewState.*`, `c4.*`, `bindRepo`, `settings.*`, `dismissFinding`.

## Kiểm thử

- `code-intel-error-classification.test.ts`: từng kind; `internal` bị bỏ qua.
- `runtime-code-intel-client.test.ts`: không `projectId` ⇒ không gọi mạng; khoá cấm; vượt giới hạn byte; huỷ; kết quả sai hình dạng ⇒ `tool-failed`.

## Tiêu chí hoàn thành

- [ ] Phủ mọi kind của bảng §2.3; không gọi mạng khi `unsupported`; không `any`.

## Rủi ro

- `ensureRuntimeEnvironmentCompatible` không được gọi (bridge đi đường phong bì): nếu môi trường cũ không có kênh, lỗi `method_not_found` được coi `unsupported`.
