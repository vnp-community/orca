# BE-CV-TASK-090-06: Golden, tái lập chéo tiến trình/dialect và fixture cho FE

**From Solution:** BE-CV-SOL-090-review-report-model
**Priority:** P2
**Service:** `code-intel-service`
**File:** `testdata/review-report/*.json`, test `review_report_determinism_test.go` (mới)
**Depends on:** BE-CV-TASK-090-05
**Status:** [ ] TODO

## Việc cần làm
1. Golden `ReviewReportModel` cho: đủ dữ liệu, thiếu cổng, thiếu overlay, tệp bị chặn nội dung, sơ đồ bị cắt.
2. Test chạy hai lần và hai tiến trình (subprocess) so digest; chạy dưới hai dialect repo giả.
3. Test chèn token giả vào mọi chuỗi nguồn ⇒ không còn trong JSON.
4. Fixture dùng chung với CR-CV-070 cho FE-CV-SOL-090.

## Kiểm thử / Tiêu chí hoàn thành
- [ ] digest bằng nhau chéo tiến trình; [ ] không đường dẫn tuyệt đối/email/lý do miễn trừ trong golden; [ ] hiệu năng 2 000 tệp đo (chưa hứa số).

## Rủi ro
- Golden đổi mỗi khi overlay đổi hình dạng; giữ builder tách khỏi nguồn.
