# TASK-E2E-026: Nối stub AI (CLI giả và HTTP) vào CI và mở khoá test BLOCKED

**From Solution:** SOL-E2E-005 · **Resolves:** BUG-E2E-010
**Priority:** P1 · **Service/Area:** tests, ci
**Files:**
- tests/stages/e2e/orca-flows/suite_request_ai_stages.py
- tests/stages/e2e/orca-flows/suite_request_lifecycle.py
- backend-go/services/request-service/e2e/stubs (dùng lại)
**Depends on:** TASK-E2E-033, 035
**Status:** `[~]` MỘT PHẦN (cập nhật 2026-10-09, chưa commit)

---

## Context

`RQL-09`, `AIS-02`, `AIS-04` luôn BLOCKED nếu không có AI thật; không thể là cổng chặn.

## Việc cần làm

- [~] Dùng stub của TASK-E2E-035 để chạy các test đó. — _dùng Claude thật qua phiên CLI thay vì stub_
- [~] Kiểm tra nội dung phản hồi theo hợp đồng (enum loại, size, urgency; Solution có kind đúng). — _RQL-09/AIS-02/AIS-04/PLN kiểm nội dung cơ bản, chưa kiểm enum đầy đủ_
- [ ] Báo cáo ghi rõ test chạy bằng stub, không đánh giá chất lượng AI. — _chưa có chế độ stub_

## Tiêu chí hoàn thành

- [ ] `python3 run_acceptance.py --strict --ai-stub` cho 0 BLOCKED trừ MCP và tuỳ chọn. — _cần TASK-035_

## Kiểm thử

Chạy thật.

## Kết quả thực thi

Một phần, đổi hướng. Thay vì stub, bộ e2e chạy Claude thật qua phiên CLI của dev server: RQL-09, AIS-02, AIS-04 và PLN-01..08 PASS. **Chưa làm**: chạy được trong CI không có tài khoản Claude (cần TASK-035). AIS-01/03 vẫn BLOCKED khi có AI (đường lỗi được phủ ở test đơn vị).
