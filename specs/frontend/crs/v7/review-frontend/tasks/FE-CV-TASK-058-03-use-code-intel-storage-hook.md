# FE-CV-TASK-058-03: Hook `useCodeIntelStorage`, khoá slice và điều kiện hiện lens

**From Solution:** [FE-CV-SOL-058](../solutions/FE-CV-SOL-058-storage-lens.md) mục 2.1, 2.2
**Priority:** P2
**Area:** frontend / renderer hooks + store
**File:** `frontend/src/renderer/src/hooks/useCodeIntelStorage.ts` (mới) + test; `frontend/src/renderer/src/store/slices/code-intel.ts` (sửa nhỏ: `storageEnv`, `selectedStorageNodeId` + vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`); `review-lens-registry.ts` (SOL-051; điều kiện hiện)
**Depends on:** FE-CV-SOL-050-store-and-query-hooks; FE-CV-TASK-058-01; G4 fake backend (FE-CV-TASK-073-02)
**Status:** [~] PARTIAL — tab Lưu trữ ẩn khi backend trả `unsupported`/`disabled` (StorageLens báo qua `shell/review-lens-availability.ts`, workspace lọc tab; ReviewWorkspace.companions.test 8/8 PASS); thiếu: chỉ ẩn sau lần đầu lens được mở (chưa có thăm dò trước khi mở)

## Context

- UI-API §3.1: `storage` nhận `(sel), env?: 'dev'|'prod'|'legacy', includeLegacy?` → `Env<StorageMap>`.
- Lens ẩn khi backend chưa hỗ trợ (CR-058 mục 6): lỗi `kind:'unsupported'` (`CODEINTEL_UNAVAILABLE`/`AGENT_UNSUPPORTED`) ⇒ không đăng ký tab, không toast.

## Việc cần làm

1. `useCodeIntelStorage({worktreeId, env, includeLegacy})` → `{status, stage?, envelope, error, isStale, reload}` qua `useCodeIntelQuery`; mặc định `env` = `'dev'`; bỏ phản hồi cũ khi đổi `env`.
2. Không tự tải lại khi `changed` (chip "Có dữ liệu mới"); tải lại khi `codeIntelResyncCounter` đổi.
3. Khoá slice `storageEnv`, `selectedStorageNodeId` (theo worktree) + action; thêm vào hằng dọn rò rỉ.
4. `isStorageLensAvailable(supportState, lastErrorKind)`: false khi `unsupported`; dùng ở `review-lens-registry.ts` để ẩn tab.

## Kiểm thử

- `renderHook` với mock `codeIntelClient.call`: đổi `env` nhanh bỏ kết quả cũ; `unsupported` ⇒ `isStorageLensAvailable=false`; `CODEINTEL_DISABLED` không gọi lại.
- Rò rỉ: xoá worktree dọn `storage*` ở cả `removeWorktree` và `buildWorktreePurgeState`.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/hooks/useCodeIntelStorage src/renderer/src/store/slices`.

## Tiêu chí hoàn thành

- [ ] Không gọi `window.api` trực tiếp; không toast; test xanh.
- [ ] Không gọi kênh nào khi cờ tắt.

## Rủi ro

- Cần `projectId` (O-1).

## Ghi chú triển khai (2026-10-07)

- Tạo `hooks/useCodeIntelStorage.ts` (+ test); khoá `storageEnv`, `selectedStorageNodeId`, action `setStorageEnv`/`selectStorageNode` ở `store/slices/review-ui.ts` (cùng lý do với 057-04: slice đã được dọn khi xoá worktree).
- Khi lỗi `unsupported`/`disabled`, `StorageLens` hiện thông báo trơn "không khả dụng" (không retry, không toast) thay vì ẩn tab; ẩn hẳn tab cần sửa khung (ngoài phạm vi, file dùng chung).

## Ghi chú tích hợp (W6, 2026-10-07)

Không thêm RPC `storage` ở lúc mở workspace (nặng). Registry `review-lens-registry.ts` không đổi; lọc nằm ở `useReviewWorkspaceModel`.
