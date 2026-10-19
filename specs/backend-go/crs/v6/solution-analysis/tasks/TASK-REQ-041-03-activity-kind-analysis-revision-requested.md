# TASK-REQ-041-03: Activity kind `analysis.revision_requested`

**From Solution:** [BE-REQ-SOL-041](../solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P2
**Service:** `request-service`
**File:** `internal/domain/request_activity.go` (sửa), `internal/adapter/{postgres,mysql}/request_activity.go` (sửa), `migrations/{postgres,mysql}/00NN_analysis_runs_revision_index.*` (mới, nếu cần), `internal/adapter/contracttest/activity_contract.go` (sửa)
**Depends on:** TASK-REQ-041-01
**Status:** [ ] TODO

---

## Context

- Activity là `UNION ALL` mỗi dialect; kind thêm mới phải có mặt ở cả hai và trong tập kind hợp lệ.
- `analysis_runs.feedback` chứa nội dung người dùng: **không được chọn** vào activity.

## Việc cần làm

1. Hằng `ActivityAnalysisRevisionRequested = "analysis.revision_requested"`, thêm vào danh sách kind hợp lệ và ánh xạ `severity=info`.
2. Nhánh `UNION` ở cả hai dialect: từ `request.analysis_runs` với `feedback IS NOT NULL AND feedback <> ''`; `id = run.id || ':revision'`; thời điểm `started_at`; tác nhân `user`, `actor_id` của run; `subject_type='solution'`, `subject_id = solution_id`; `summary` cố định; các cột còn lại rỗng. **Không có cột `feedback` trong câu lệnh.**
3. Chạy `EXPLAIN` trên Request có ≥ 50 lượt phân tích ở cả hai dialect; nếu cần thì migration chỉ mục `(tenant_id, request_id, started_at)`; nếu không cần thì ghi kết quả EXPLAIN vào PR và không thêm migration.
4. Cập nhật `CONTRACT-request-ui-api.md` mục 2.10: kind mới.

## Kiểm thử

- Hợp đồng `activity_contract`: Request có 3 vòng chỉnh → đúng 3 dòng `analysis.revision_requested`, thứ tự và phân trang keyset ổn định, lọc `kinds` theo kind mới.
- Test khẳng định câu lệnh SQL không chứa cột `feedback` (so khớp văn bản câu lệnh) và dòng trả về không chứa nội dung phản hồi.
- Hai dialect cho cùng kết quả.

## Tiêu chí hoàn thành

- [ ] Mỗi lần chỉnh để lại một dòng activity không có nội dung.
- [ ] Postgres và MySQL cho kết quả giống nhau.
- [ ] EXPLAIN đã ghi; chỉ mục thêm khi cần.

## Rủi ro và lưu ý

- Đổi `UNION` làm nặng hơn mọi lần đọc activity; đo trước/sau.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
