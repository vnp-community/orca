# TASK-REQ-041-03: Activity kind `analysis.revision_requested`

**From Solution:** [BE-REQ-SOL-041](../solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P2
**Service:** `request-service`
**File:** `internal/domain/request_activity.go` (sửa), `internal/adapter/{postgres,mysql}/request_activity.go` (sửa), `migrations/postgres/0100_analysis_runs_revision_index.*` (mới; MySQL không cần), `internal/adapter/contracttest/activity_revision_contract.go` (mới), `internal/adapter/contracttest/execution_repository_contract.go` (đăng ký kịch bản)
**Depends on:** TASK-REQ-041-01
**Status:** [x] DONE (2026-10-19)

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

- [x] Mỗi lần chỉnh để lại một dòng activity không có nội dung.
- [x] Postgres và MySQL cho kết quả giống nhau.
- [x] EXPLAIN đã ghi; chỉ mục thêm khi cần.

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

---

## Ghi chú triển khai

**Blast radius (không có GitNexus; dùng grep).** `BuildActivityEntry` chỉ được gọi ở `usecase/list_request_activity.go:105`; `AllActivityKinds` chỉ được dùng ở `cleanActivityKinds` (cùng file, dòng 153) nên bộ lọc `kinds` của gateway và MCP nhận kind mới mà không phải sửa. `activityUnion` / `activityBranches` chỉ dùng trong chính hai adapter. Kind lạ vẫn rơi vào nhánh `default` của `BuildActivityEntry` (summary = kind), nên client cũ không hỏng.

**Tệp đã đổi.**
- `internal/domain/request_activity.go`: hằng `ActivityAnalysisRevisionRequested`, thêm vào `AllActivityKinds`, nhánh trong `BuildActivityEntry` (summary cố định `Requested changes to the analysis`, severity info mặc định, `details = {solutionId}`). `request_activity_test.go`: hai test mới.
- `internal/adapter/postgres/request_activity.go` và `internal/adapter/mysql/request_activity.go`: nhánh `UNION ALL` thứ 11 đọc `analysis_runs` có `feedback IS NOT NULL AND feedback <> ''`; `id = id || ':revision'`, `started_at`, `'user'`, `COALESCE(actor_id,'')`, `'solution'`, `COALESCE(solution_id,'')`, các cột còn lại rỗng/0. `feedback` chỉ nằm trong `WHERE`, không nằm trong danh sách cột. MySQL: `activityBranches` 10 -> 11.
- `internal/adapter/contracttest/activity_revision_contract.go` (mới): `AssertActivityUnionOmitsFeedback` (so khớp văn bản câu lệnh: danh sách chọn của nhánh không có `feedback`; ngoài vị trí lọc `feedback IS NOT NULL AND feedback <> ''` không chỗ nào khác trong cả câu UNION nhắc tới cột này), hai kịch bản hợp đồng. `activity_revision_sql_test.go`: kiểm chứng chính phép kiểm (5 câu lệnh sai phải bị từ chối). `request_activity_sql_test.go` ở mỗi adapter gọi phép kiểm trên `activityUnion` thật. `execution_repository_contract.go`: đăng ký 2 kịch bản vào danh sách.
- `migrations/postgres/0100_analysis_runs_revision_index.{up,down}.sql`: chỉ mục `(tenant_id, request_id, started_at)`.
- `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` mục 2.10: kind mới.

**Kịch bản hợp đồng (chạy trên Postgres 16 và MySQL 8).** `Activity_RevisionRequested_OneLinePerRound_NoFeedback`: Request có 3 vòng chỉnh (hai vòng cùng `started_at` để thử tie-break keyset), cộng một lượt đầu không phản hồi, một lượt phản hồi rỗng, một lượt của Request khác và một lượt của tenant khác cùng `request_id`: đúng 3 dòng; trường từng dòng; thứ tự mới trước; phân trang keyset theo 1, 2 (chỉ kind mới) và theo 4 (toàn bộ feed, 15 dòng cũ + 3) trùng với một lần đọc; lọc `kinds` (một mình và trộn kind khác), `subjectId`, `since`, `actorKind=agent` (0 dòng); không hàng nào, không `ActivityEntry` nào chứa chuỗi đánh dấu phản hồi. `Activity_RevisionRequested_NullSolutionAndActor`: lượt chưa có `solution_id`/`actor_id` vẫn đọc được (id rỗng), thử `COALESCE`.

**Lệnh đã chạy.**
- `cd backend-go/services/request-service && go build ./... && go vet ./internal/domain/ ./internal/adapter/...` xanh.
- `go test ./internal/domain/ ./internal/adapter/contracttest/ ./internal/adapter/postgres/ ./internal/adapter/mysql/ -race -count=1` xanh (gồm `TestTenantScope` và test văn bản SQL).
- `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -count=1 -p 1`: cả hai `ok` (postgres 50.7s, mysql 101.5s), gồm toàn bộ hợp đồng, RLS và migration up/down/up với 0100.

**EXPLAIN (>= 50 lượt phân tích).** Dữ liệu: Request đích có 60 lượt (40 có phản hồi), cộng 40 Request khác x 250 lượt cùng tenant, tổng 10 060 dòng `analysis_runs`; `ANALYZE` trước khi đo; Postgres chạy bằng vai `app` (RLS bật) trong transaction có `app.tenant_id`.
- Postgres, trước chỉ mục: nhánh mới đứng riêng là `Seq Scan on analysis_runs`, `Rows Removed by Filter: 10020`, `Buffers: shared hit=222`, 0.776 ms. Vì chỉ mục idempotency hiện có là chỉ mục từng phần (`WHERE idempotency_key IS NOT NULL`) nên không dùng được cho `(tenant_id, request_id)`, chi phí tăng tuyến tính theo toàn bảng. Cả câu `ListActivity` kèm nhánh mới: 0.78 đến 0.93 ms; cùng câu khi chưa có nhánh: 0.153 ms. Sau chỉ mục `analysis_runs_by_request`: `Index Scan using analysis_runs_by_request`, `Buffers: shared hit=4`, 0.050 ms (nhánh đứng riêng); cả câu 0.25 đến 0.30 ms (vẫn lớn hơn 0.168 ms của câu chưa có nhánh vì thêm 40 dòng cần sắp xếp). Kết luận: cần chỉ mục -> migration 0100.
- MySQL 8: nhánh dùng `Index lookup on analysis_runs using analysis_runs_idem (tenant_id, request_id)` cả khi chưa lẫn khi đã có chỉ mục thử `(tenant_id, request_id, started_at)` (optimizer vẫn chọn `analysis_runs_idem`, cùng chi phí 19.9, ~0.08 ms; cả câu 0.12 đến 0.41 ms, chênh lệch là do khởi động nguội). Khoá duy nhất `analysis_runs_idem` không phải chỉ mục từng phần nên đã phủ tiền tố cần dùng. Kết luận: không cần chỉ mục, **không thêm migration MySQL** (chỉ mục thứ hai chỉ tốn chi phí ghi). Tệp tạm đo EXPLAIN đã xoá.

**Lệch so với đề bài.**
1. Migration chỉ có ở Postgres (số 0100); thư mục MySQL không có 0100. Hai thư mục hiện khác nhau ở số này; nếu agent khác thêm migration 0100 cho MySQL thì không xung đột về nội dung, chỉ cần đánh số lại cho khớp nếu muốn song hành.
2. "Câu lệnh không chứa cột `feedback`": cột `feedback` vẫn xuất hiện trong mệnh đề `WHERE` (bắt buộc để lọc theo đề bài mục 2); điều được khẳng định là nó không nằm trong danh sách chọn và không nơi nào khác trong câu UNION nhắc đến nó.
3. Kịch bản mới nằm ở tệp mới `activity_revision_contract.go` (không thêm vào `activity_contract.go` vì đã 346 dòng) và được đăng ký bằng hai dòng trong `execution_repository_contract.go`.

**Chưa kiểm chứng / ngoài phạm vi.**
- Chú thích của `proto/orca/request/v1/activity.proto` (dòng 15 đến 17) liệt kê các kind nhưng chưa có `analysis.revision_requested`; chỉ là comment, không phải tệp của tôi.
- Frontend (`activity-entry-presentation.ts`, `activity-entry-text.ts`, bản dịch) chưa biết kind mới; hiện rơi vào nhánh kind lạ. Thuộc FE-REQ-SOL-041.
- Chưa đo ở quy mô triệu dòng; số liệu EXPLAIN ở trên ở 10 060 dòng.
