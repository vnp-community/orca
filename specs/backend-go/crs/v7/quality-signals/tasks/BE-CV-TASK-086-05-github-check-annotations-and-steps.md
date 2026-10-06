# BE-CV-TASK-086-05: GitHub annotations và steps theo yêu cầu

**From Solution:** BE-CV-SOL-086-scm-commit-checks
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/adapter/github/commit_check_details.go`, test (mới)
**Depends on:** BE-CV-TASK-086-04
**Status:** [ ] TODO

## Context
CR-086 2.1/2.6: chỉ check thất bại, ≤ 50 annotation/check, không `logTail`.

## Việc cần làm
1. `include_annotations`: `GET /repos/{r}/check-runs/{id}/annotations?per_page=50`.
2. `include_steps`: `GET /repos/{r}/actions/jobs/{id}` khi có `workflowRun`.
3. Che `message` ≤ 1 KiB; bỏ `raw_details`.
4. Đếm lời gọi: chỉ cho check thất bại.

## Kiểm thử
- Check thành công không phát sinh lời gọi; thất bại 60 annotation → 50; lỗi 404 một check không làm hỏng cả danh sách.

## Tiêu chí hoàn thành
- [ ] Canary token trong `message` bị che.

## Rủi ro
Mỗi check thất bại một lời gọi: ngân sách do caller.
