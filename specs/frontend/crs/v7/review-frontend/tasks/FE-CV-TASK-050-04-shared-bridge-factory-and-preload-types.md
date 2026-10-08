# FE-CV-TASK-050-04: `createCodeIntelBridge` và kiểu `PreloadApi.codeIntel`

**From Solution:** [FE-CV-SOL-050-types-and-runtime-bridge](../solutions/FE-CV-SOL-050-types-and-runtime-bridge.md) mục 4.2
**Priority:** P0
**Area:** frontend / shared + preload types
**File:** `frontend/src/shared/code-intel-bridge.ts` (mới), `frontend/src/preload/api-types.ts` (sửa, cạnh `mcp` :936), test `code-intel-bridge.test.ts`
**Depends on:** FE-CV-TASK-050-02, FE-CV-TASK-050-03
**Status:** [x] DONE (verified 2026-10-07: bridge.test 8 pass; bridge now normalizes raw push frames via parseCodeIntelPushEvent (dotted wire names quality.progress etc. map to internal camelCase))

## Context

- Mẫu: `McpBridgeApi` (`api-types.ts:928-936`), `createMcpApi` (`web/web-mcp-api.ts`): khung đầu ack `null`, khung sau object trần.
- `runtimeEnvironments.call` nhận `selector` (`api-types.ts:3285-3297`); đích local `runtime.call({method, params})`.
- U1/U2: một object `args[0]`, không `send`.

## Việc cần làm

1. Khai báo `CodeIntelBridgeApi`, `CodeIntelBridgeDeps` (chữ ký ở SOL mục 4.2) và `createCodeIntelBridge(deps)`.
2. `call`: `environmentId === null` ⇒ `callLocal`; ngược lại `callEnvironment({selector: environmentId,...})`; `params ?? {}`; trả phong bì nguyên.
3. `subscribeEvents`: local ⇒ `onUnsupported()` ngay + hàm huỷ rỗng; bỏ ack; chỉ chuyển object có `event` thuộc `CODE_INTEL_PUSH_EVENTS`; `{type:'end'}`/`onClose` ⇒ `onClose()`; huỷ trước khi handle về vẫn `unsubscribe` sau đó; sau huỷ không còn nhận khung.
4. Thêm `codeIntel: CodeIntelBridgeApi` vào `PreloadApi`.

## Kiểm thử

- `code-intel-bridge.test.ts`: local/environment; ack bị bỏ; event lạ bị bỏ; huỷ sớm; `onUnsupported`; không có hàm `send`.

## Tiêu chí hoàn thành

- [ ] Test xanh; `tsc` sạch; `api-types.ts` chỉ thêm một khoá.

## Rủi ro

- `desktop/src/preload/api-types.ts` là bản riêng (ngắn hơn): task 050-06 phải đồng bộ.
