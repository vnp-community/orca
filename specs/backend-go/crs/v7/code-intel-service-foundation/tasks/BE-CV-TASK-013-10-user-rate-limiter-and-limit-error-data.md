# BE-CV-TASK-013-10: Giới hạn thô theo `(tenant,user)` (L1) và dữ liệu lỗi hạn mức `CodedData`

**From Solution:** BE-CV-SOL-013-agent-call-gate-and-quotas
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/user_rate_limiter.go`, `internal/domain/coded_data.go` (nếu 013-07 chưa tạo), `_test.go` (mới)
**Depends on:** BE-CV-TASK-013-09
**Status:** [x] DONE

---

## Context

SOL-013-gate mục 2.D; PQ-02 (3) hậu tố `" | {json}"` ≤ 2 KiB; UI-API §2.3 `CODEINTEL_RATE_LIMITED` có `retryAfterSeconds?`, `scope?`. L1 chống bão request làm quá tải `project-service` (mỗi RPC chưa cache cần 2–4 lời gọi nội bộ).

## Việc cần làm

1. `UserRateLimiter.Allow(ctx) error`: `rate.Limiter` theo `(tenant,user)` (`CODEINTEL_USER_RPS`, burst); từ chối → `CODEINTEL_RATE_LIMITED` (`KindResourceExhausted`) với `retryAfterSeconds = ceil(Reserve().Delay())` tối thiểu 1; không tiêu thụ token khi từ chối (dùng `Reserve` rồi `Cancel`).
2. Dọn mục rảnh (map lớn): ticker 1 phút xoá mục không dùng 10 phút; dừng theo `ctx`.
3. `domain.CodedData` (nếu chưa có): `map[string]any`; hàm `WithData(err *apperrors.AppError, data CodedData)` giữ dữ liệu cho `toStatus` (013-07) mà không sửa `common/apperrors` (đóng gói trong kiểu lỗi bọc `*codedError` có `Unwrap`).
4. Nối vào pipeline bước 3 (013-07), cổng tiêm; `NoopLimiter` cho test.
5. Không lưu `userID` ở log mức INFO (chỉ băm ngắn hoặc không).

## Kiểm thử

- Unit (đồng hồ giả): 40 yêu cầu liên tiếp qua, yêu cầu 41 bị từ chối kèm `retryAfterSeconds ≥ 1`; sau 1 s lại qua; user A không ảnh hưởng user B, tenant khác độc lập; dọn mục rảnh; từ chối không tiêu token.
- `errors.As` lấy được `AppError` và `CodedData` qua lớp bọc.
- `go test -race ./services/code-intel-service/internal/usecase/... -run UserRateLimiter`

## Tiêu chí hoàn thành

- [x] L1 hoạt động và cô lập theo `(tenant,user)`.
- [x] Lỗi mang `retryAfterSeconds` tới hậu tố JSON.

## Rủi ro và lưu ý

- Bộ nhớ tăng theo số người dùng hoạt động; dọn định kỳ.
