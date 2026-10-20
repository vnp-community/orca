# TASK-REQ-045-04: Bảng lịch sử phiên bản của OpenSpec change, ghi cùng transaction với lượt sinh

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`
**File:** `migrations/{postgres,mysql}/0104_openspec_change_revisions.{up,down}.sql` (mới), `internal/domain/openspec_change_revision.go` (+ test, mới), `internal/usecase/{ports,run_proposal_generation}.go` (sửa: `OpenSpecChangeRevisionRepository`, `AnalysisRunStore.ListSucceeded`), `internal/adapter/{postgres,mysql}/{openspec_change_revision_repository,analysis_run_repository}.go` (mới/sửa), `cmd/server/{store_wiring,wire_plan}.go` (sửa), `internal/adapter/contracttest/{openspec_change_revision_contract,expected_columns_openspec_revisions}.go` (mới), mọi fake của `AnalysisRunStore`
**Depends on:** TASK-REQ-045-03
**Status:** [ ] TODO

---

## Context

- `request.openspec_changes` có một dòng mỗi Request (`UNIQUE (tenant_id, request_id)`, `0007`); `finish` ghi đè bằng `Upsert` (`run_proposal_generation.go:285`). DB không giữ phiên bản cũ; git trên nhánh proposal là lịch sử duy nhất (`proposal_workspace.go`).
- `PlanArtifacts.Describe` (`plan_subject_artifacts.go`), `TasksMdSyncer` và `ProposalJanitor` đọc dòng hiện hành; không đổi cấu trúc dòng đó (quyết định S3).
- `analysis_runs_one_running` (unique theo `(tenant_id, request_id, kind) WHERE running`) bảo đảm các lượt Plan của một Request tuần tự, nên đánh số "đếm lượt thành công + 1" không đua.
- Mẫu bảng + RLS: `0007_solution_engines.up.sql` (Postgres `ENABLE/FORCE ROW LEVEL SECURITY`, policy `tenant_isolation`); MySQL không RLS, khoá theo `tenant_id`; kiểu: Postgres `uuid`, `jsonb`, `timestamptz`; MySQL `VARCHAR(36)`, `JSON`, `TIMESTAMP(6)`.
- Quyết định: giữ toàn văn `PlanRevisionTextKept = 10` bản mới nhất; chỉ mục và digest giữ mãi (CR mục 2.5).

## Việc cần làm

1. Migration `0104` (số tiếp theo sau `0103` của task 03; `0101` là của BE-REQ-SOL-044, `0102` của task 02; kiểm lại): bảng `openspec_change_revisions (id, tenant_id, request_id FK, revision INT, run_id, commit_sha, tasks_md_digest, bundle_digest, files_json, bundle_json NULL, created_at)`, `UNIQUE (tenant_id, request_id, revision)`, chỉ mục `(tenant_id, request_id, revision DESC)`; RLS Postgres; `.down.sql` cả hai dialect.
2. `domain/openspec_change_revision.go`: `OpenSpecChangeRevision`, `RevisionFile{Path, Size, Omitted, SHA256}`, hằng `PlanRevisionTextKept = 10`, `NewRevisionFiles(bundle map[string]string) []RevisionFile` (sha256 của văn bản đã làm sạch; với file bị lược lấy `sha256` và `size` từ dấu bằng `ParseOmittedMarker`), `DiffFileIndex(prev, cur []RevisionFile) []FileChange` (`added|changed|removed|unchanged`, `removed` mang kích thước bản trước).
3. Cổng `OpenSpecChangeRevisionRepository`: `Append(ctx, rev, keepText int) error` (chèn, rồi `UPDATE ... SET bundle_json = NULL WHERE tenant_id AND request_id AND revision <= $new - $keep AND bundle_json IS NOT NULL`), `List(ctx, requestID, beforeRevision, limit)`, `Get(ctx, requestID, revision)`, `GetByRun(ctx, runID)`. Cài ở Postgres và MySQL; dùng cùng transaction nếu ngữ cảnh đã mở (mẫu `withRelayTx`/`scoped` của repository hiện có).
4. `AnalysisRunStore.ListSucceeded(ctx, requestID, kind)`: `status = 'succeeded'`, `ORDER BY started_at ASC, id`, theo `kind`; cài hai dialect; cập nhật mọi fake. Dùng lại ở task 02 (kế thừa), 05 (đọc), 06 (Plan cũ cho prompt native).
5. `finish` (`run_proposal_generation.go`): trong cùng `InTx` với `Upsert` và `finishPlanRun`, tính `revision = len(ListSucceeded(plan)) + 1`, dựng `OpenSpecChangeRevision` từ `change` vừa lưu (digest tính trên `Bundle` đã làm sạch), `Append(…, domain.PlanRevisionTextKept)`. Lỗi `Append` làm hỏng cả transaction (không để `openspec_changes` mới mà thiếu dòng bản).
6. `wire_plan.go`/`store_wiring.go`: dựng repository cho cả hai dialect, truyền vào `RunProposalGenerationDeps.Revisions`; thiếu cổng thì khởi động lỗi (giống `NewApproverAwareAuthorizer`) khi engine openspec bật.
7. Request cũ chưa có dòng bản: không backfill; đường đọc (task 05) coi dòng `openspec_changes` hiện có là bản hiện hành. Lượt chỉnh đầu tiên sau triển khai ghi bản `ListSucceeded+1` (số này đã tính cả lượt cũ nên liên tục).

## Kiểm thử

- Hợp đồng hai dialect (`openspec_change_revision_contract.go`, testcontainers Postgres 16 và MySQL 8): `Append` ghi cùng transaction với `Upsert` (rollback thì cả hai mất); `UNIQUE` chặn trùng số bản; lược chữ ở bản thứ 11 (bản 1 mất `bundle_json`, bản 2..11 còn; chỉ mục bản 1 còn); `List` phân trang ổn định mới nhất trước; `Get` bản không tồn tại; cách ly tenant; Request cũ không có dòng bản; EXPLAIN truy vấn `List` dùng chỉ mục trên Request có ≥ 50 bản.
- `TestDiffFileIndex_*` (thêm, đổi, xoá, giữ, đổi tên = xoá + thêm), `TestNewRevisionFiles_OmittedMarkerSizeAndHash`.
- `TestFinish_AppendsRevisionInSameTx`, `_FailedAppendFailsRun`, `_RevisionNumberFollowsSucceededPlanRuns`.
- `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'OpenSpecChange|PlanRevision' -count=1 -p 1` (lệnh của máy colima đã dùng ở TASK-REQ-041-04; điều chỉnh theo máy).

## Tiêu chí hoàn thành

- [ ] Mỗi lượt sinh OpenSpec thành công thêm đúng một dòng bản cùng transaction với `Upsert`.
- [ ] Số bản liên tục và bằng thứ tự lượt Plan thành công.
- [ ] Chỉ 10 bản mới nhất giữ toàn văn; chỉ mục và digest của mọi bản còn.
- [ ] Migration xuôi và ngược trên hai dialect; Request cũ đọc được.
- [ ] Không đổi hành vi của `PlanArtifacts.Describe`, `TasksMdSyncer`, `ProposalJanitor`.

## Rủi ro và lưu ý

- `jsonb` Postgres sắp lại khoá và loại khoảng trắng: `files_json` là mảng có thứ tự nên giữ nguyên; `bundle_json` là map nên không dựa vào thứ tự khi tính lại digest (dùng `BundleDigest()` sắp theo đường dẫn).
- Kích thước hàng tối đa ≈ 1 MiB văn bản; đo trên bộ tài liệu pilot (chưa đo).
- Chạy `gitnexus_impact` cho `finish`, `AnalysisRunStore` (nhiều fake).
- Nếu CR-REQ-044 chuyển `finish` hay thêm bước, ghép lại thứ tự giao dịch.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
