# FE-CV-TASK-050-05: Cài đặt web `web-code-intel-api.ts`

**From Solution:** [FE-CV-SOL-050-types-and-runtime-bridge](../solutions/FE-CV-SOL-050-types-and-runtime-bridge.md) mục 4.1
**Priority:** P0
**Area:** frontend / renderer web
**File:** `frontend/src/renderer/src/web/web-code-intel-api.ts` (mới), `web/web-preload-api.ts` (sửa đúng **một dòng** cạnh `mcp:` :797), test `web-code-intel-api.test.ts`
**Depends on:** FE-CV-TASK-050-04
**Status:** [ ] TODO

## Context

- `web-preload-api.ts` có `eslint-disable max-lines` từ trước (dòng 1-3): không thêm disable mới.
- `callEnvironmentEnvelope(selector, method, params, timeoutMs)` (:3498) trả phong bì, `getClientForEnvironment(env).subscribe` (:802), `resolveEnvironment(selector)`; những hàm này là nội bộ file: truyền vào qua transport như `createMcpApi({...})`.
- Proxy `withFallback` (:4430): không phát hiện tính năng bằng `typeof`.

## Việc cần làm

1. `createCodeIntelApi(transport)` gọi `createCodeIntelBridge` với `callLocal` (trả phong bì `ok:false` giả `method_not_found` vì web không có runtime local), `callEnvironment` (`callEnvironmentEnvelope`), `subscribeEnvironment` (`client.subscribe(method, params, callbacks)`).
2. Trong `web-preload-api.ts` thêm `codeIntel: createCodeIntelApi({ callEnvironmentEnvelope, subscribe })` cạnh `mcp`.
3. Không có môi trường hoạt động ⇒ phong bì lỗi (không ném).

## Kiểm thử

- `web-code-intel-api.test.ts` (mẫu `web-mcp-api.test.ts`): `ok:false` giữ `error.message` nguyên; ack bị bỏ; không môi trường ⇒ không ném.

## Tiêu chí hoàn thành

- [ ] `window.api.codeIntel.call` ở web trả phong bì kể cả lỗi; một dòng thêm vào `web-preload-api.ts`.

## Rủi ro

- `runtimeCallQueuePool` có thể xếp hàng các lời gọi cùng `(env, method)`: chưa kiểm chứng ảnh hưởng tới `status` polling.
