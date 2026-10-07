# BE-CV-TASK-072-06: Ma trận quyền 49 RPC và từ chối đồng nhất

**From Solution:** BE-CV-SOL-072
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/adapter/grpc/rpc_permission_matrix.go` (mới, bảng dữ liệu), `.../internal/adapter/grpc/rpc_permission_matrix_test.go` (mới), `.../internal/usecase/uniform_denial_test.go` (mới), `.../internal/adapter/grpc/internal_caller_guard_test.go` (mới)
**Depends on:** BE-CV-SOL-013-authorization-flags-and-audit (OPA `code_intel.rego`), BE-CV-SOL-085-* (QualityGateService), BE-CV-SOL-073 task 03 (interceptor cờ)
**Status:** `[x] DONE`

---

## Context

- PQ-03 (5): quyền dự án ⇒ `CODEINTEL_NOT_AUTHORIZED` (không tồn tại, không phải thành viên, tenant khác: cùng message); id tài nguyên con ⇒ `CODEINTEL_NOT_FOUND`.
- Action OPA §3.1/§3.2 (`read`, `read_source`, `review_write`, `reindex`, `c4_write`, `quality_read`, `quality_waive`, `quality_profile_write`); §6.3: `c4_write`, `quality_profile_write` chỉ owner/admin; `quality_waive` member ≤ 7 ngày, không miễn `check`/`error`; O-18 (tạm: member không miễn `check`, `reindex` cho member có hạn mức).
- Guard nội bộ: `CODEINTEL_INTERNAL_CALLER_TOKEN` rỗng = chặn hết (`common/internalcaller`).

## Việc cần làm

1. Bảng `rpc_permission_matrix`: 49 dòng (`rpc`, `actions[]`, hành vi khi cờ tắt, vai trò kỳ vọng); sinh/đối chiếu với `code_intel.rego` (không tự bịa vai trò).
2. `TestEveryRPCHasPermissionRow`: RPC trong `ServiceDesc` (cả hai service) mà thiếu dòng ⇒ đỏ; dòng mà không có RPC ⇒ đỏ.
3. Test ma trận: với mỗi RPC × {không quyền project, tenant khác, member đọc, member ghi, admin, owner} kết quả đúng bảng.
4. `uniform_denial_test`: project không tồn tại và project tenant khác cho cùng mã + message + dạng lỗi; id con tenant khác ⇒ `NOT_FOUND`.
5. Ca: `expectedVersion` cũ ⇒ `CODEINTEL_VERSION_CONFLICT`; `DismissFinding` không miễn cổng với `error` (PQ-05); `WaiveFinding` member > 7 ngày hoặc `check` ⇒ lỗi; `DismissFinding` ghi `dismissed_by`.
6. `TestInternalCallerGuardBlocksWhenTokenEmpty`, và token sai ⇒ từ chối.
7. Audit: `GetSymbol` và mọi ghi tạo dòng audit qua cổng audit giả (kiểm trong giới hạn `auditclient`).

## Kiểm thử

- `go test ./internal/adapter/grpc/... ./internal/usecase/...`; không DB (repo giả).

## Tiêu chí hoàn thành

- [x] Thêm RPC mà quên dòng quyền thì CI đỏ.
- [x] Mọi từ chối cùng mã và message.

## Rủi ro và lưu ý

- Bảng vai trò gốc ở CR-013/085, ngoài ba hợp đồng (khoảng trống); oracle thời gian chỉ ghi nhận, không kiểm.
