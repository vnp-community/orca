# TASK-E2E-020: `/readyz` phản ánh cấu hình thật; rà guard nội bộ ở các service

**From Solution:** SOL-E2E-004 · **Resolves:** BUG-E2E-002
**Priority:** P1 · **Service/Area:** request-service và các service dùng internalcaller
**Files:**
- backend-go/services/request-service/cmd/server/*.go (readyz)
- deploy/dev/docker-compose.yml (healthcheck)
- backend-go/common/internalcaller/internalcaller.go (rà ngữ nghĩa token rỗng)
**Depends on:** TASK-E2E-018
**Status:** `[~]` MỘT PHẦN (cập nhật 2026-10-09, chưa commit)

---

## Context

Token rỗng hiện chỉ ghi log ERROR và service vẫn "chạy"; lỗi hiện thành `REQUEST_FORBIDDEN` mơ hồ.

## Việc cần làm

- [~] `/readyz` không sẵn sàng khi: OPA bundle chưa nạp, hai token rỗng (khi cờ luồng bật), DB không tới được. — _`/readyz` có kiểm tra token; chưa kiểm OPA bundle và DB_
- [ ] Compose: `healthcheck` gọi `/readyz` với `start_period` hợp lý. — _chưa thêm healthcheck vào compose_
- [ ] Rà `internalcaller.Guard` và mọi nơi dùng: token rỗng luôn từ chối. Ghi kết quả; nếu có nơi coi rỗng là "tắt guard" thì sửa và thêm test. — _chưa rà guard ở các service khác_

## Tiêu chí hoàn thành

- [ ] Tắt token → container không healthy trong 60 giây. — _chưa thử_
- [x] Test chứng minh guard rỗng từ chối.

## Kiểm thử

Unit test readyz; test guard rỗng.

## Kết quả thực thi

Một phần. `/readyz` có kiểm tra `internal_tokens` (request-service) + test. **Chưa làm**: rà guard nội bộ fail-closed ở các service khác.
