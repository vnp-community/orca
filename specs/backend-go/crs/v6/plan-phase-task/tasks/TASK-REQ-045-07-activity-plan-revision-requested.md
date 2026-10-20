# TASK-REQ-045-07: Activity `plan.revision_requested` và sửa nhánh `analysis.revision_requested` cho khỏi bắt nhầm lượt Plan

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/request_activity.go` (sửa: hằng, `AllActivityKinds`, mô tả theo kind nếu có), `internal/adapter/{postgres,mysql}/request_activity.go` (sửa), `internal/adapter/contracttest/{activity_revision_contract,activity_contract}.go` (sửa), `internal/adapter/contracttest/activity_revision_sql_test.go` (sửa), test domain của danh sách kind
**Depends on:** TASK-REQ-045-02
**Status:** [x] DONE (2026-10-20)

---

## Context

- CR-REQ-041 đã thêm nhánh UNION `analysis.revision_requested` đọc `request.analysis_runs` có `feedback` khác rỗng và không bắt đầu bằng `clarification:` (Postgres `adapter/postgres/request_activity.go:39-40`; MySQL dòng 38), tác nhân `user`, `subject_type='solution'`, **không chọn cột `feedback`** (C9).
- Nhánh đó không lọc `kind`. Lượt Plan cũng có `feedback` (cột chung), nên ngay khi Plan có phản hồi, dòng Activity sẽ ghi "Requested changes to the analysis" cho lượt Plan (G12).
- `domain/request_activity.go`: `ActivityAnalysisRevisionRequested`, `AllActivityKinds` là tập đóng cho bộ lọc.

## Việc cần làm

1. Test hợp đồng đỏ trước: Request có một lượt Solution có phản hồi và một lượt Plan có phản hồi → feed phải có đúng một `analysis.revision_requested` và đúng một `plan.revision_requested` (hiện là hai dòng `analysis.*`).
2. `request_activity.go` (domain): `const ActivityPlanRevisionRequested = "plan.revision_requested"` với chú thích "ghi việc xảy ra, không bao giờ ghi nội dung"; thêm vào `AllActivityKinds`.
3. Hai dialect: nhánh `analysis.revision_requested` thêm `AND kind <> 'plan'`; nhánh mới `plan.revision_requested`: `FROM analysis_runs WHERE tenant_id AND request_id AND kind = 'plan' AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'`, cột `id:revision`, `started_at`, `'user'`, `actor_id`, `subject_type = 'plan'`, `subject_id = ''`, `summary` cố định "Requested changes to the plan", **không chọn `feedback`**. Giữ cùng thứ tự cột A..D, N với các nhánh khác (`ActivityRow`).
4. `BuildActivityEntry`/`activity_presentation` (backend nếu có hàm tóm tắt theo kind): thêm kind mới với tóm tắt cố định, mức `info`.
5. Bộ lọc `kinds` của `ListRequestActivity` chấp nhận kind mới; `ErrActivityFilterInvalid` vẫn từ chối kind lạ.
6. Chỉ mục `analysis_runs_by_request` (`0100`) đã phục vụ truy vấn; kiểm EXPLAIN nhánh mới không quét toàn bảng.

## Kiểm thử

- Hợp đồng hai dialect: ca ở việc 1; lượt Plan có phản hồi `clarification:` không sinh dòng; nội dung phản hồi không xuất hiện ở bất kỳ cột nào của dòng (chuỗi cắm sẵn không nằm trong JSON feed).
- `activity_revision_sql_test.go`: kiểm chuỗi SQL không chứa `feedback` ở danh sách cột chọn của nhánh mới.
- Domain: `AllActivityKinds` chứa kind mới; bộ lọc theo kind.
- `go test ./services/request-service/internal/... -run 'Activity' -race -count=1`; testcontainers hai dialect.

## Tiêu chí hoàn thành

- [x] Lượt Plan có phản hồi hiện đúng một dòng `plan.revision_requested`, không còn dòng `analysis.revision_requested` cho lượt Plan. (`Activity_PlanRevisionRequested_NotMistakenForAnAnalysis` (Postgres và MySQL): 1 dòng `plan.*`, 1 dòng `analysis.*`)
- [x] Dòng không chứa nội dung phản hồi (C9). (cùng kịch bản (chuỗi đánh dấu không có trong hàng hay entry) và `TestActivityUnionFeedbackProblem` (phép kiểm văn bản câu lệnh áp cho cả hai nhánh))
- [x] Bộ lọc theo kind nhận kind mới; Request cũ không đổi. (`TestAllActivityKinds_NamesThePlanRevisionKindOnce`; kịch bản lọc `kinds` trộn hai loại; Request không có lượt Plan có phản hồi không đổi)
- [x] EXPLAIN dùng chỉ mục. (`TestPostgres_PlanRevisionActivityBranchUsesAnIndex` (Index Scan); MySQL: `possible_keys` có `analysis_runs_idem` (khoá duy nhất không từng phần), bảng thử quá nhỏ nên optimizer chọn quét)

## Rủi ro và lưu ý

- Gateway/frontend phải chịu được kind lạ (đã có từ CR-REQ-037); frontend hiển thị ở FE-REQ-TASK-045-07.
- Chạy `gitnexus_impact` cho `AllActivityKinds` và hàm dựng UNION.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai

**Blast radius (grep).** `AllActivityKinds`: chỉ `cleanActivityKinds` (`list_request_activity.go`) nên bộ lọc `kinds` của gateway/MCP nhận kind mới mà không đổi gì; `BuildActivityEntry` chỉ được gọi ở `list_request_activity.go`; `activityUnion`/`activityBranches` chỉ dùng trong hai adapter. Gateway không liệt kê kind. Frontend rơi vào nhánh kind lạ cho tới FE-REQ-TASK-045-07.

**Tệp đã đổi.** `internal/domain/request_activity.go` (hằng `ActivityPlanRevisionRequested = "plan.revision_requested"`, vào `AllActivityKinds`, nhánh `BuildActivityEntry`: tóm tắt cố định `Requested changes to the plan`, severity info, không `Details`) + `request_activity_test.go`; `internal/adapter/{postgres,mysql}/request_activity.go` (nhánh `analysis.revision_requested` thêm `AND kind <> 'plan'`; nhánh mới thứ 12 `kind = 'plan'`, `id || ':plan-revision'`, `started_at`, `'user'`, `actor_id`, `subject_type = 'plan'`, `subject_id = ''`, **không** chọn `feedback`; MySQL `activityBranches` 11 -> 12); `contracttest/activity_revision_contract.go` (phép kiểm văn bản cho cả hai nhánh, kịch bản mới `activityPlanRevisionRequested`), `activity_revision_sql_test.go` (4 ca sai mới), `execution_repository_contract.go` (đăng ký một dòng); `mysql/request_activity_sql_test.go` (test số nhánh khớp số tham số, vì quên tăng `activityBranches` sẽ lệch đối số âm thầm); `{postgres,mysql}/plan_documents_migration_integration_test.go` (EXPLAIN).

**Lệnh đã chạy.** `go test ./internal/domain/ ./internal/adapter/... -run 'Activity|Union' -count=1` xanh; `go test ./... -race -count=1` toàn module xanh; integration `go test -tags=integration -p 1 ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'Activity|Execution'` xanh trên cả hai dialect.

**Lệch so với đề bài.** (1) Hậu tố id là `:plan-revision` (đề bài ghi `id:revision`) để không trùng với hậu tố `:revision` của nhánh analysis nếu một ngày hai nhánh chung một id. (2) Không có migration/chỉ mục mới: Postgres dùng `analysis_runs_by_request` (0100); đo EXPLAIN trên bảng nhỏ (30 lượt) với `enable_seqscan=off` chỉ chứng minh chỉ mục *có thể* phục vụ, chưa đo trên bảng lớn như bản của 041-03.

**Chưa kiểm chứng / ngoài phạm vi.** `proto/orca/request/v1/activity.proto` còn comment liệt kê kind chưa có `plan.revision_requested` (chỉ là comment, không phải file của tôi). Frontend chưa hiển thị kind mới.
