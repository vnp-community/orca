# FE-REQ-TASK-018-01: Kiểu dùng chung, registry luồng, hằng kênh, lỗi và parser

**From Solution:** [FE-REQ-SOL-018](../solutions/FE-REQ-SOL-018-request-frontend-foundation.md) mục 2.1, 2.2, 2.3, 2.4
**Priority:** P0
**Area:** frontend / shared
**File:** `frontend/src/shared/request-types.ts` (mới), `request-flow-registry.ts` (mới), `request-rpc-methods.ts` (mới), `request-errors.ts` (mới), `request-wire-parsers.ts` (mới) và test cùng tên `.test.ts`
**Depends on:** không (thuần kiểu, không cần backend)
**Status:** [x] DONE

## Context

- `frontend/src/shared/` đã có `task-types.ts`, `jira-types.ts`; không có gì cho Request (đã grep). Tên file theo khái niệm, không dùng `utils`/`common`.
- Hợp đồng kênh lấy từ CR-REQ-016 (2.3, 2.4, 2.8). Schema `Solution.options`/`Approval` chưa có CONTRACT (`specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` chưa tồn tại): parser phải chịu thiếu trường.
- README v6 mục 3.4 là nguồn của registry; mục 8 số 6 và 7 không đổi bảng này.

## Việc cần làm

1. `request-types.ts`: khai báo `RequestType` (11), `RequestStatus` (11), `RequestSize` (`S|M|L`), `RequestUrgency`, `TypeSource`, `RequestSourceProvider` (`jira|github|gitlab|linear|mcp|manual|webhook`), `ReturnedFromStage`, `OrcaRequest` (có `planTaskId?`, `links?`, `version?`), `RequestTypeHistoryEntry`, `RequestLink` + `RequestLinkReason`, `SolutionKind/Status/Option`, `Solution`, `ApprovalSubjectType`, `ApprovalStatus`, `Approval` (có `version`, `subjectDigest`), `BacklogView`, `RequestBacklogItem`, `TaskBacklogItem`, `ExecuteBacklogItem`, `RequestEvent {requestId,eventType,status,type,occurredAt}`. Mọi union có thêm `'unknown'` ở kiểu đầu ra của parser (không ở kiểu gửi đi).
2. `request-flow-registry.ts`: `REQUEST_FLOW_REGISTRY: Record<RequestType, {analysisKind, plan, phase, gates}>` đúng 11 dòng README 3.4; `REQUEST_STATUS_ORDER`; `LOW_CONFIDENCE_THRESHOLD = 0.6` (ghi comment "đề xuất, chưa có dữ liệu"); hàm `flowHasPhase(type, size)` (`always` hoặc `size_l` với `size==='L'`).
3. `request-rpc-methods.ts`: `REQUEST_RPC_METHODS` as const theo bảng SOL-018 2.2 (không có `request.flowSet`); export kiểu `RequestRpcMethod`.
4. `request-errors.ts`: `RequestRpcErrorKind`, `RequestRpcError`, `classifyRequestRpcError(err: unknown): RequestRpcError` theo bảng SOL-018 2.4; `splitErrorCode(message)` dùng `/^([A-Z][A-Z0-9_]+):\s*(.*)$/s`; nhận `RuntimeRpcCallError` bằng `instanceof` (import từ `renderer/src/runtime/runtime-rpc-result` bị cấm trong `shared/`: nhận bằng duck-typing `{code: string, message: string}` để `shared` không phụ thuộc renderer).
5. `request-wire-parsers.ts`: `parseRequest`, `parseSolution`, `parseApproval`, `parseBacklogItem(view, raw)`, `parseRequestEvent`. Enum lạ thành `'unknown'`, chuỗi thiếu thành `''`, mảng thiếu thành `[]`; `SolutionOption.raw` giữ phần dư; `options[i].id` thiếu thì dùng `String(i)`.

## Kiểm thử

- `request-flow-registry.test.ts`: bảng test 11 loại (analysisKind, plan, phase, gates) so với README 3.4; `flowHasPhase('bug','L')` true, `('bug','M')` false, `('change_request','S')` true.
- `request-errors.test.ts`: mỗi mã CR-REQ-016 2.8 vào đúng `kind`; message không có `CODE:` thành `network` hoặc `unknown` theo cờ; `method_not_found` thành `unsupported`.
- `request-wire-parsers.test.ts`: enum lạ, thiếu trường, `null` slice, `options` thiếu `id`.
- Chạy: `pnpm --filter orca-frontend test -- src/shared/request-` rồi `pnpm --filter orca-frontend exec tsc --noEmit -p tsconfig.json` (chưa kiểm chứng lệnh typecheck).

## Tiêu chí hoàn thành

- [ ] 5 file tồn tại, không `any`, không import từ `renderer/`.
- [ ] Registry khớp từng ô bảng 3.4 (test xanh).
- [ ] Mọi mã lỗi ở CR-016 2.8 có test ánh xạ.
- [ ] Parser không ném với đầu vào `{}`, `null`, enum lạ.

## Rủi ro và lưu ý

- Schema `Solution.options`, `Approval` là suy luận: khi CONTRACT ra, sửa ở đây một chỗ.
- Không thêm hằng kênh chưa có trong CR-016.
