# FE-CV-TASK-057-04: Hook `useCodeIntelErd` và khoá slice ERD

**From Solution:** [FE-CV-SOL-057](../solutions/FE-CV-SOL-057-erd-lens.md) mục 2.2
**Priority:** P0
**Area:** frontend / renderer hooks + store
**File:** `frontend/src/renderer/src/hooks/useCodeIntelErd.ts` (mới) + test; `frontend/src/renderer/src/store/slices/code-intel.ts` (sửa nhỏ: khoá `selectedErdTable`, `erdService`, `erdServiceHistory` và action `selectErdTable`, `setErdService`, `goBackErdService`); `frontend/src/renderer/src/store/slices/*worktree-purge*.test.ts` (mẫu rò rỉ)
**Depends on:** FE-CV-SOL-050-store-and-query-hooks (`useCodeIntelQuery`, `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`, `store-test-helpers.ts`); G4 fake backend (FE-CV-TASK-073-02) hoặc mock `codeIntelClient.call`
**Status:** [x] DONE

## Context

- UI-API §3.1: `erd` nhận `(sel), service?, dialect?, base?, head?, includeAccess?, includeInferred?`; không `service` ⇒ `{services: ErdServiceInfo[]}`; phong bì phẳng (U6, PQ-12).
- Đã đọc ở SOL-057: SOL-050 sở hữu slice; task này chỉ thêm khoá, không thêm slice mới. Mỗi khoá theo `worktreeId` phải vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` và có test rò rỉ.

## Việc cần làm

1. `useCodeIntelErd({worktreeId, service, dialect})` trả `{services, model, reload}` (SOL-057 2.2): pha 1 gọi `erd` với `{}` (lấy danh sách), pha 2 chỉ chạy khi có `service` (`includeAccess:true`, `includeInferred:false`, `base/head` từ `scope`).
2. Chọn mặc định: không `service` ⇒ service đầu có `tableCount>0`; không `dialect` ⇒ `ErdServiceInfo.dialects[0]`.
3. Bỏ phản hồi cũ bằng số thứ tự yêu cầu khi đổi `service`/`dialect`; huỷ khi gỡ.
4. Không tự tải lại khi push `changed` (SOL-050 đánh dấu cũ; hook chỉ phơi `isStale` để lens hiện chip); tải lại khi `codeIntelResyncCounter` đổi.
5. Action `setErdService(worktreeId, service)` đẩy service cũ vào `erdServiceHistory` (cho "Quay lại"); `selectErdTable(worktreeId, tableKey|null)`; thêm hai khoá vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`.
6. Hook **không** toast; lỗi trả `CodeIntelUiError` đã phân loại (U5).

## Kiểm thử

- `useCodeIntelErd.test.tsx` (`renderHook`, mock `codeIntelClient.call`): hai pha; đổi service nhanh bỏ kết quả cũ; `CODEINTEL_TIMEOUT` có `inProgress` giữ trạng thái `loading` + `stage`; `enabled=false` không gọi; không tải lại khi `changed`.
- Slice: `setErdService` lịch sử; xoá worktree dọn `erdService*` ở **cả** `removeWorktree` và `buildWorktreePurgeState` (mẫu `bulk-worktree-purge-terminal-maps-leak.test.ts`).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/hooks/useCodeIntelErd src/renderer/src/store/slices`.

## Tiêu chí hoàn thành

- [ ] Hook dùng `useCodeIntelQuery`, không gọi `window.api` trực tiếp.
- [ ] Khoá mới có test rò rỉ.
- [ ] Không gọi kênh nào khi `effective.codeIntelEnabled=false`.

## Rủi ro

- `projectId` (O-1) phải có; nếu SOL-050 trả `null` thì hook giữ `idle` và lens hiện "chưa gắn dự án".
