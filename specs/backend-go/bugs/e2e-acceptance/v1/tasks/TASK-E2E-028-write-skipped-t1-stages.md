# TASK-E2E-028: Viết các stage e2e Go đang skip (Solution, Plan, CommitPlan, ReportTaskOutcome)

**From Solution:** SOL-E2E-006 · **Resolves:** OBS-D
**Priority:** P2 · **Service/Area:** request-service
**Files:**
- backend-go/services/request-service/e2e/scenarios_flows_test.go
- backend-go/services/request-service/e2e/scenarios_cross_cutting_test.go
- backend-go/services/request-service/e2e/stubs
**Depends on:** TASK-E2E-026
**Status:** `[ ]` CHƯA LÀM (cập nhật 2026-10-09, chưa commit)

---

## Context

49 stage tự skip với nhãn "implemented now … extend this stage" (TASK-REQ-025-04).

## Việc cần làm

- [ ] Mỗi stage gọi RPC thật với tham số đúng (hiện gửi rỗng nên nhận `REQUEST_ID_REQUIRED`). — _chưa làm_
- [ ] Dùng stub AI xác định để Solution/Plan sinh được. — _chưa làm_
- [ ] Kiểm `WantStatus` sau từng stage; hoàn tất đường E01–E12. — _chưa làm_

## Tiêu chí hoàn thành

- [ ] Số stage skip theo nhãn đó = 0; 11 loại có kịch bản đi trọn tới `completed`. — _chưa làm_

## Kiểm thử

Chạy gói e2e hai dialect.

## Kết quả thực thi

Chưa làm. Giai đoạn Plan đã có (SOL-V2-001), nên có thể viết các stage Go còn skip; để vòng sau.
