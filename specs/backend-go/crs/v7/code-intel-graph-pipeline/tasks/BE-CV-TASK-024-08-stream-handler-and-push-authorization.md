# BE-CV-TASK-024-08: Handler `StreamCodeIntelEvents` và `PushAuthorization`

**From Solution:** BE-CV-SOL-024-event-distribution
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/adapter/grpc/server_code_intel_events.go`, `.../internal/usecase/push_authorization.go` (mới) và test
**Depends on:** TASK-024-02, TASK-024-07, BE-CV-SOL-013-authorization-flags-and-audit
**Status:** [ ] TODO

---

## Context

PQ-11: selectors 0..50, quyền `read` từng worktree/sự kiện (cache 10 s); tenant lấy từ metadata stream.

## Việc cần làm

1. Hàm lấy identity từ metadata stream (viết trong service).
2. Kiểm cờ, 0..50 selector, quyền từng selector (`PermissionDenied` toàn yêu cầu); rỗng → lọc từng sự kiện.
3. Đăng ký vào broadcaster; `CodeIntelPush` ≤ 1 KiB, không đồ thị/đường dẫn tuyệt đối.
4. Giới hạn luồng/replica theo `CODE_INTEL_MAX_STREAMS` phía gateway; phía service chỉ ghi nhận số luồng (metric `codeintel_push_subscribers`).

## Kiểm thử

- `go test ./internal/adapter/grpc/ -run StreamCodeIntelEvents -race`: không quyền → `PermissionDenied`, không push nào; 51 selector → `INVALID_PARAMS`; cờ tắt → `CODEINTEL_DISABLED`; tenant A không nhận sự kiện B; huỷ ctx giải phóng.

## Tiêu chí hoàn thành

- [ ] Các ca xanh. - [ ] Lỗi tra cứu quyền = từ chối.

## Rủi ro và lưu ý

- Gateway không tự mở lại luồng (SOL-040).
