# TASK-REQ-045-05: `ListPlanRevisions` và đọc theo phiên bản (`GetPlanProposal.revision`, `GetPlanBundleFile.revision`)

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`, `proto`
**File:** `proto/orca/request/v1/{plan,request}.proto` (sửa) + mã sinh, `internal/usecase/{list_plan_revisions,get_plan_proposal,get_plan_bundle_file}.go` (mới/sửa), `internal/domain/{plan_errors,rpc_catalog}.go` (+ test; sửa), `internal/adapter/grpc/{server_plan,flow_gate}.go` (sửa), `cmd/server/wire_plan.go` (sửa), `internal/adapter/contracttest/plan_revision_read_contract.go` (mới)
**Depends on:** TASK-REQ-045-04
**Status:** [x] DONE (2026-10-20)

---

## Context

- `GetPlanProposal` trả bundle/checks/knowledge của **dòng hiện hành** cho bất kỳ `run_id` nào (chỉ một dòng DB) và `find` lấy lượt Plan mới nhất trong `ListRecent` (`get_plan_proposal.go`). Khi lượt chỉnh đang chạy hoặc vừa thất bại, lượt mới nhất không có đề xuất, nên giao diện native không có gì để hiển thị nếu chỉ dựa vào đó.
- `GetPlanBundleFile` đọc `change.Bundle[rel]` (dòng hiện hành).
- Phản hồi, người chỉnh, thời điểm nằm ở `analysis_runs` (`feedback`, `actor_id`, `started_at`, `finished_at`, `run_stats`); native lưu đề xuất ở `raw_output` (đã làm sạch).
- Nhóm RPC: `GetPlanProposal`, `GetPlanBundleFile` là `GroupRead` (`rpc_catalog.go:108-109`); RPC mới cũng `GroupRead`, `AgentAllowed=false`.

## Việc cần làm

1. Proto: `GetPlanProposalRequest.revision = 3` (int32; 0 = hành vi cũ), `GetPlanBundleFileRequest.revision = 3`, `PlanBundleFile.sha256 = 4` và `change = 5` (`added|changed|unchanged|removed`), RPC `ListPlanRevisions(ListPlanRevisionsRequest{request_id, page_size, page_token}) returns (ListPlanRevisionsResponse{revisions, next_page_token})`, message `PlanRevision{revision, run_id, status(current|superseded), engine, revision_feedback, actor_id, generated_at, commit, files_changed, file_count, text_kept, stats}`. Kiểm số trường chưa bị CR khác giữ (`grep -n "= 3;" plan.proto`).
2. `ListPlanRevisions` (usecase mới `list_plan_revisions.go`): `ListSucceeded(plan)` theo Request để có danh sách bản (số = thứ tự), phân trang mới nhất trước (mặc định 20, tối đa 100, `page_token` = số bản cuối của trang trước); với engine openspec ghép `GetByRun`/`List` để lấy `commit`, `files_json`, `text_kept`; `files_changed` = số file `added|changed|removed` so với bản liền trước (bản 1: bằng `file_count`). `revision_feedback` lấy từ `AnalysisRun.Feedback` (rỗng với lượt không phản hồi **và** với phản hồi bắt đầu bằng `clarification:`); nạp lô, không N+1. Request cũ chưa có dòng bản: bản hiện hành tổng hợp từ `openspec_changes`.
3. `GetPlanProposal.Execute(ctx, requestID, runID, revision)`: nếu `revision > 0` thì tìm lượt thứ `revision` trong `ListSucceeded`; không có → `ErrPlanRevisionNotFound` (`REQUEST_PLAN_REVISION_NOT_FOUND`, `NotFound`); trả `proposal`, và với openspec `bundle` dựng từ dòng bản (file với `Size`, `Omitted`, `SHA256`, `Change` so với bản liền trước); `checks` và `knowledge` chỉ khi đó là bản hiện hành. Với `revision == 0` giữ hành vi cũ, thêm `SHA256` và `Change` cho bản hiện hành (so với bản liền trước nếu có). Không đổi `runID` semantics.
4. `GetPlanBundleFile.Execute(ctx, requestID, requested, revision)`: `revision == 0` hoặc bằng bản mới nhất → dòng hiện hành (như cũ, kèm che khi đọc của task 03); bản cũ → `bundle_json` của dòng bản; `NULL` → `ErrPlanRevisionTextPruned` (`REQUEST_PLAN_REVISION_TEXT_PRUNED`, `FailedPrecondition`); đường dẫn kiểm bằng `bundleRelPath` như cũ.
5. `server_plan.go`, `flow_gate.go` (`classRead` cho `ListPlanRevisions`), `rpc_catalog.go` (+ `TestCatalog*`): đăng ký RPC; `wire_plan.go` dựng `ListPlanRevisions`.
6. `PlanRevision.stats`: dùng `runStatsView`/mapper có sẵn của `GetPlanProposalResponse.stats`; không hiện khi lượt không báo.

## Kiểm thử

- `TestListPlanRevisions_*`: không lượt nào; một lượt; nhiều lượt có và không phản hồi; phân trang; native và openspec; bản bị lược chữ `text_kept=false`; Request cũ không có dòng bản; phản hồi `clarification:` bị ẩn; lô một truy vấn.
- `TestGetPlanProposal_Revision_*` (bản cũ, bản hiện hành, bản không tồn tại, native lấy đề xuất đúng lượt), `TestGetPlanBundleFile_Revision_*` (bản cũ, bản bị lược chữ, bản không tồn tại, đường dẫn thoát vẫn bị từ chối).
- Hợp đồng hai dialect `plan_revision_read_contract.go`: ghi ba bản, đọc từng bản, lược chữ, phân trang.
- `go test ./services/request-service/internal/... -run 'PlanRevision|GetPlan|Catalog' -race -count=1`.

## Tiêu chí hoàn thành

- [x] Người đọc được Request liệt kê được mọi phiên bản Plan (native và openspec) kèm `revisionFeedback`. (`TestListPlanRevisions_NativeVersionsNewestFirstWithTheirAsk`, `_OpenSpecVersionsCountTheFilesThatChanged`, `_ARequestThatPredatesTheHistoryListsItsCurrentVersion`; hợp đồng `ThreeVersionsReadBackOneByOne` trên Postgres 16 và MySQL 8)
- [x] Đọc được từng file của bản cũ còn chữ; bản đã lược báo `REQUEST_PLAN_REVISION_TEXT_PRUNED`. (`TestGetPlanBundleFile_Revision_OlderCurrentPrunedAndMissing`; hợp đồng `PrunedTextAndPaging`: 12 bản, bản 1 và 2 bị lược)
- [x] Đề xuất native của bản cũ đọc được khi lượt mới nhất đang chạy hoặc đã thất bại. (`TestGetPlanProposal_Revision_NativeProposalOfTheRightRunWhileTheNewestRunFailed`)
- [x] Không có N+1; chỉ mục được dùng (EXPLAIN). (`TestListPlanRevisions_OpenSpecVersionsCountTheFilesThatChanged`: một lần `List` của kho bản, không `Get`; `TestPostgres_ListSucceededPlanRunsUsesAnIndex` (Index Scan, không Seq Scan; bật `enable_seqscan=off` vì bảng thử nhỏ), `TestMySQL_ListSucceededPlanRunsCanUseAnIndex` (`possible_keys` có `analysis_runs_idem`); chỉ mục của bảng bản đã đo ở task 04)
- [x] Danh mục RPC và `flow_gate` khớp (`TestCatalog*` xanh). (`go test ./internal/domain/ ./internal/adapter/grpc/ ./internal/adapter/opaclient/`, gồm ma trận OPA `TestCatalogPersonMatrixAgainstRego`)

## Rủi ro và lưu ý

- Số bản của Request cũ: lượt Plan thành công trước CR không có dòng bản, nhưng vẫn đếm trong `ListSucceeded`; chỉ bản mới nhất có tài liệu (các bản cũ hơn của chính Request đó báo `text_kept=false`, `files` rỗng). Ghi rõ trong thông điệp giao diện (FE-05).
- `revision_feedback` là nội dung người dùng (C9): không log, không span.
- Chạy `gitnexus_impact` cho `GetPlanProposal.Execute` và `GetPlanBundleFile.Execute` (chữ ký đổi; gọi từ `server_plan.go`, CLI không gọi trực tiếp).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai

**Hoàn thành 2026-10-20.** Đường dẫn dưới `backend-go/services/request-service/` trừ khi ghi khác.

**Blast radius (grep).** `GetPlanProposal.Execute(ctx, requestID, runID, revision)` và `GetPlanBundleFile.Execute(ctx, requestID, requested, revision)` đổi chữ ký: người gọi duy nhất ngoài test là `adapter/grpc/server_plan.go` (CLI và gateway đi qua gRPC). Bốn file test được sửa bằng `gofmt -r 'a.Execute(b, c, d) -> a.Execute(b, c, d, 0)'` (`get_plan_bundle_test`, `get_plan_proposal_test`, `get_plan_proposal_auto_test`, `plan_bundle_redaction_test`; chỉ thêm đối số `0`). `PlanBundleFileView` thêm `SHA256`, `Change`; `addChange` nay dùng `domain.NewRevisionFiles` (cùng kích thước và cờ `Omitted` như trước). `PlanUseCases` thêm `Revisions`. `domain.rpc_catalog` và `flow_gate` thêm `ListPlanRevisions`.

**Tệp đã đổi.** Mới: `internal/usecase/{list_plan_revisions,get_plan_proposal_revision}.go`, `internal/adapter/grpc/server_plan_revisions.go`, `internal/adapter/contracttest/plan_revision_read_contract.go`; test `list_plan_revisions_test.go`, `get_plan_revision_test.go`, `plan_revision_reads_fakes_test.go`, `adapter/grpc/server_plan_revisions_test.go`, `{postgres,mysql}/plan_revision_origin_integration_test.go` (cùng file với task 02: hợp đồng đọc và EXPLAIN). Sửa: `proto/orca/request/v1/{plan,request}.proto` + `plan.pb.go`, `request.pb.go`, `request_grpc.pb.go` (sinh bằng protoc 33.4, protoc-gen-go v1.36.11, protoc-gen-go-grpc 1.6.1; header `protoc (unknown)` của `plan.pb.go` giữ nguyên; `solution.pb.go`, `approval.pb.go` sinh lại không đổi), `internal/usecase/{get_plan_proposal,get_plan_bundle_file}.go`, `internal/domain/{plan_errors,rpc_catalog}.go`, `internal/adapter/grpc/{server_plan,flow_gate}.go`, `cmd/server/wire_plan.go`.

**Hành vi.**
- Số bản là chỉ số lượt Plan thành công (`ListSucceeded`) cộng 1, đúng cho cả engine native. `ListPlanRevisions`: một `ListSucceeded`, một `List` của kho bản (đúng `size+1` dòng để biết bản liền trước của mục cũ nhất), nối theo `run_id`. `files_changed` = số file `added|changed|removed`; bản 1 và bản có bản liền trước chưa có chỉ mục (Request cũ) thì bằng `file_count`. Bản mới nhất của Request cũ (chưa có dòng bản) được tổng hợp từ `openspec_changes` (`text_kept=true`); các bản cũ hơn `text_kept=false`, `file_count=0`. Run native: `text_kept=true`, không `commit`.
- `GetPlanProposal` với `revision > 0` thắng `runId`. Bản cũ: `proposal` từ `raw_output` của đúng lượt, `bundle` từ dòng bản (`commit`, `tasks_md_digest` theo bản; `change_id`, `branch`, trạng thái đồng bộ theo dòng hiện hành), không `checks` và `knowledge`. File `removed` có trong danh sách với kích thước cũ. Bản đầu không có nhãn `change` (không có gì để so). Với `revision == 0` bản hiện hành có thêm `sha256` và `change` (so với bản liền trước nếu kho bản có).
- `GetPlanBundleFile`: bản mới nhất đọc dòng hiện hành (có che khi đọc); bản cũ đọc `bundle_json`, `NULL` hoặc không có dòng bản thì `REQUEST_PLAN_REVISION_TEXT_PRUNED`; số bản vượt hoặc âm thì `REQUEST_PLAN_REVISION_NOT_FOUND`; server chưa gắn kho bản thì chỉ đọc được bản hiện hành (số bản khác: NOT_FOUND). Kiểm đường dẫn vẫn qua `bundleRelPath`.
- Thêm mã `REQUEST_PLAN_REVISION_PAGE_TOKEN_INVALID` (InvalidArgument) cho `page_token` không phải số dương; token vượt cuối thì bắt đầu từ bản mới nhất.
- `revision_feedback` ẩn với phản hồi bắt đầu bằng `clarification:`; không log, không span.

**Lệnh đã chạy.** `go test ./... -race -count=1` toàn module xanh; `go test ./internal/usecase/ -run 'ListPlanRevisions|Revision_|Predates' -count=1`; integration hai dialect `-run 'PlanRevisionRead|ListSucceededPlanRuns'` xanh (Postgres 16, MySQL 8).

**Chưa kiểm chứng.** Đo `ListSucceeded` trên Request có hàng trăm lượt Plan (tải `raw_output` của mọi lượt thành công; Plan native có thể lên 256 KiB mỗi lượt); EXPLAIN chỉ trên bảng nhỏ. Chạy thật trên stack (task 09).
