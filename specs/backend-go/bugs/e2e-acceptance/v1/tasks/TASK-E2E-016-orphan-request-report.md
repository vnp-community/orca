# TASK-E2E-016: Báo cáo và xử lý Request mồ côi

**From Solution:** SOL-E2E-003 · **Resolves:** BUG-E2E-009
**Priority:** P2 · **Service/Area:** request-service, ops
**Files:**
- deploy/dev/scripts/report-orphan-requests.sh (mới)
- backend-go/services/request-service/internal/usecase/ (metric)
- docs/guides/request/runbook-request-flow.md
**Depends on:** TASK-E2E-015
**Status:** `[~]` MỘT PHẦN (cập nhật 2026-10-09, chưa commit)

---

## Context

Request đã tạo trước bản sửa có thể trỏ project không tồn tại; chúng tính vào hạn mức tenant.

## Việc cần làm

- [~] Truy vấn chỉ đọc: Request mà `project_id` không có trong `project.projects` cùng tenant (hai DB khác nhau nên đi qua `ResolveProjectAccess` theo lô, không join chéo DB). — _script đi qua gateway (`request.list` + `project.get`), không dùng RPC theo lô_
- [~] Huỷ có lý do `orphan_project` qua đường admin (`request.cancel`) sau khi người có thẩm quyền duyệt danh sách. — _script có `--cancel` với lý do `orphan_project`; chưa chạy trên dữ liệu thật_
- [ ] Metric `orca_request_orphan_total`; mục runbook. — _chưa làm metric và runbook_

## Tiêu chí hoàn thành

- [x] Có danh sách và cách xử lý; không xoá dữ liệu.

## Kiểm thử

Chạy trên dữ liệu thử có Request mồ côi tạo trước đó.

## Kết quả thực thi

Một phần. Có `tests/stages/e2e/orca-flows/report_orphan_requests.py` (chỉ đọc; `--cancel` huỷ với lý do `orphan_project`, không xoá) và nó đã tìm thấy 27 Request mồ côi trên môi trường local. **Chưa làm**: metric `orca_request_orphan_total` và mục runbook (cần quét xuyên tenant). Tôi không chạy `--cancel` vì có Request không do phiên này tạo.
