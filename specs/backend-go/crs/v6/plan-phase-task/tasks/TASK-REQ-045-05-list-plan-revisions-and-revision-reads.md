# TASK-REQ-045-05: `ListPlanRevisions` và đọc theo phiên bản (`GetPlanProposal.revision`, `GetPlanBundleFile.revision`)

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`, `proto`
**File:** `proto/orca/request/v1/{plan,request}.proto` (sửa) + mã sinh, `internal/usecase/{list_plan_revisions,get_plan_proposal,get_plan_bundle_file}.go` (mới/sửa), `internal/domain/{plan_errors,rpc_catalog}.go` (+ test; sửa), `internal/adapter/grpc/{server_plan,flow_gate}.go` (sửa), `cmd/server/wire_plan.go` (sửa), `internal/adapter/contracttest/plan_revision_read_contract.go` (mới)
**Depends on:** TASK-REQ-045-04
**Status:** [ ] TODO

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

- [ ] Người đọc được Request liệt kê được mọi phiên bản Plan (native và openspec) kèm `revisionFeedback`.
- [ ] Đọc được từng file của bản cũ còn chữ; bản đã lược báo `REQUEST_PLAN_REVISION_TEXT_PRUNED`.
- [ ] Đề xuất native của bản cũ đọc được khi lượt mới nhất đang chạy hoặc đã thất bại.
- [ ] Không có N+1; chỉ mục được dùng (EXPLAIN).
- [ ] Danh mục RPC và `flow_gate` khớp (`TestCatalog*` xanh).

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
