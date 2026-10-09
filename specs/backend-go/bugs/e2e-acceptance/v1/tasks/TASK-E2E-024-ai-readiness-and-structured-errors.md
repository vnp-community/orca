# TASK-E2E-024: Kiểm tra sẵn sàng AI bằng khả năng thật và mã lỗi có cấu trúc

**From Solution:** SOL-E2E-005 · **Resolves:** BUG-E2E-010
**Priority:** P0 · **Service/Area:** request-service
**Files:**
- backend-go/services/request-service/internal/usecase (preflight AI)
- backend-go/services/request-service/internal/domain (mã lỗi)
- backend-go/services/request-service/internal/adapter/grpc (ánh xạ)
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_dispatch.go (ánh xạ lỗi)
**Depends on:** TASK-E2E-034
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Thông báo hiện tại là "classifier error"; người vận hành không biết phải làm gì. Sau quyết định, tín hiệu sẵn sàng là `ai.sources` của dev server (TASK-E2E-034), không phải bảng tài khoản provider.

## Việc cần làm

- [x] Trước bước AI: gọi `GetDevServerCapabilities` của dev server của project; sẵn sàng nếu `ai.sources` không rỗng hoặc có tài khoản provider.
- [~] Mã: `REQUEST_AI_NOT_CONFIGURED`, `REQUEST_AI_AGENT_UNAVAILABLE`, `REQUEST_AI_SESSION_EXPIRED`, `REQUEST_AI_QUOTA_EXHAUSTED`, `REQUEST_AI_CREDENTIAL_UNAVAILABLE` (nguồn 2). — _thiếu `REQUEST_AI_CREDENTIAL_UNAVAILABLE` vì nguồn 2 không làm; thêm `REQUEST_AI_BUSY`_
- [x] Ghi mã vào run phân loại/sinh thay cho chuỗi chung; hiển thị qua `classificationReason`/`errorCode`.
- [x] Thiếu trường `ai.sources` (agent cũ) → vẫn thử gọi, không chặn.
- [x] Giữ thoái hoá an toàn.

## Tiêu chí hoàn thành

- [x] Chưa đăng nhập CLI: `classificationReason = REQUEST_AI_NOT_CONFIGURED`.
- [x] Test unit cho từng nhánh.

## Kiểm thử

Unit test; `RQL-09` có lý do đúng mã khi chưa đăng nhập.

## Kết quả thực thi

Xong. Thăm dò sẵn sàng bằng khả năng thật (`AIReadiness`), mã lỗi `REQUEST_AI_NOT_CONFIGURED/AGENT_UNAVAILABLE/SESSION_EXPIRED/QUOTA_EXHAUSTED/BUSY`, ánh xạ ở infra-fleet và request-service; test từng mã. e2e RQL-09 PASS với Claude thật.
