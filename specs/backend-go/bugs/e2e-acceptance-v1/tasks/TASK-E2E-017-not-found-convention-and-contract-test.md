# TASK-E2E-017: Quy ước "không có → NotFound" và test hợp đồng xuyên service

**From Solution:** SOL-E2E-003 · **Resolves:** BUG-E2E-007, OBS-A
**Priority:** P2 · **Service/Area:** task, project, workflow, request
**Files:**
- backend-go/services/task-service/internal/usecase/delete_task.go (đã sửa)
- backend-go/services/task-service/internal/adapter/{postgres,mysql}/repository.go (đã sửa)
- tests/stages/e2e/orca-flows/suite_not_found_contract.py (mới)
- specs/backend-go/ (ghi quy ước)
**Depends on:** TASK-E2E-014
**Status:** `[~]` MỘT PHẦN (cập nhật 2026-10-09, chưa commit)

---

## Context

Quy tắc: repo trả sentinel miền bọc `%w`; usecase ánh xạ `errors.Is` sang `NotFound`; không bọc lỗi repo thành `Internal` khi chưa phân loại.

## Việc cần làm

- [~] Rà các repo `Delete`/`Update`/`Get` của các service chính tìm `fmt.Errorf("… not found")` không dùng sentinel. — _mới rà task delete và project; chưa rà mọi service_
- [~] Sửa tương tự task delete cho các chỗ tìm thấy (mỗi service một commit). — _đã sửa task delete và project get_
- [x] Suite `orca-flows` mới: id lạ → `NotFound` cho task, project, workflow execution, request, clarification.
- [ ] Ghi quy ước vào tài liệu kiến trúc service. — _chưa ghi vào tài liệu kiến trúc_

## Tiêu chí hoàn thành

- [x] Suite hợp đồng xanh.
- [x] Không còn lỗi 5xx cho id lạ trong log e2e.

## Kiểm thử

Suite hợp đồng mới trong `orca-flows`.

## Kết quả thực thi

Một phần. Suite `notfound` (NFC-01..08) PASS cho task, project, workflow execution, request, clarification, approval; sửa project id sai định dạng. **Chưa làm**: rà toàn bộ repo `Delete/Update/Get` của mọi service và ghi quy ước vào tài liệu kiến trúc.
