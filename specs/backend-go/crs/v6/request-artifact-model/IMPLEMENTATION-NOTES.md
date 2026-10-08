# Ghi chú triển khai: request-artifact-model

## Phần task-service của CR-REQ-027 (TASK-REQ-027-02), 2026-10-08

Kiểm chứng: `go build/vet/test ./...` của `task-service` xanh; integration Postgres 16 (`postgres:16-alpine`) và MySQL 8.0 thật cho migration `0020` up/down/up, CAS 8 goroutine, khoá cây, cascade, chia lô trên 200 id, cách ly tenant, `RunInTxWithSpecs` rollback chung task và spec; RLS kiểm bằng role không phải superuser. `buf breaking` so với `main` xanh.

### Đã làm
- Migration `0020_task_specs` (hai dialect), RLS `FORCE` với `NULLIF(current_setting('app.tenant_id', true), '')` và `WITH CHECK`; repository đặt `set_config` trong mỗi giao dịch (`adapter/postgres/tenant_scoped_tx.go`).
- `domain.NewTaskSpec`, `CanonicalJSON` (khoá theo thứ tự, NFC, số nguyên không `.0`, từ chối khoá trùng, sâu tối đa 32), digest SHA-256 hex. Mẫu vàng dùng chung với request-service: `task-service/testdata/artifacts/task/canonical_cases.json` (request-service cần chép đúng tệp này; bước CI `diff -r` chưa làm).
- Use case `SetTaskSpec`, `GetTaskSpecs`, `LockTaskSpecs`; `UpdateTask.WithTaskSpecLock` chặn đổi `title` khi spec đã khoá (status, labels, PR, worktree vẫn đổi được); RPC `SetTaskSpec`, `GetTaskSpecs`, `LockTaskSpecs` ở `task.proto`, handler `server_task_spec.go`, nối ở `main.go`.
- `RunInTxWithSpecs` (cả hai adapter) cạnh `RunInTx` không đổi. Chưa có người gọi trong task-service vì `CreatePlanTree` chưa tồn tại.
- `common/apperrors`: thêm `KindAborted` (cuối danh sách, ánh xạ `codes.Aborted`) cho `TASK_SPEC_VERSION_CONFLICT`. Cộng thêm, không đổi gì sẵn có.

### Quyết định lệch so với task
1. Quyền dùng hành động `write` (OPA không có `edit`). Kiểm quyền chỉ khi ngữ cảnh có `user`; lời gọi giữa service (chỉ có tenant) chỉ bị giới hạn tenant, như `GetTask` và `ReportTaskExecutionResult` (task-service chưa có danh tính service). Ghi nhận là rủi ro.
2. `expected_version = 0` nghĩa là tạo mới. Gửi lại đúng spec đã lưu (cùng digest, cùng `schema_version`) trả về bản hiện có, không tăng `version`: để `CommitPlan` thử lại an toàn.
3. `title` chỉ bị chặn khi giá trị mới khác giá trị hiện tại (giao diện hay gửi lại nguyên title).
4. `GetTaskSpecs` trả `spec_json` đã chuẩn tắc lại (JSONB/JSON làm lại định dạng), nên request-service băm đúng chuỗi nhận được.
5. Spec phải là đối tượng JSON.
6. Khi bảng chưa migrate, `HasSpec` và `IsLocked` trả `false` (log một lần) để rolling deploy không làm hỏng Execute/UpdateTask.

### Chưa kiểm chứng / còn mở
- TiDB; Postgres 14 (chỉ chạy 16).
- Phạm vi khoá (Q3): hiện khoá mọi spec trong cây con của task được truyền vào; `request-service` quyết định gọi cho plan hay phase.
- `CreatePlanTree` với `spec_json` (027-07 bước 6) chưa làm vì RPC chưa có.
- Bước CI so sánh mẫu vàng hai service chưa làm.

---

# Phần request-service (nhánh rf/art, đã hợp nhất vào `feat/request-flow-backend`)

Viết ngày 2026-10-08 cho CR-REQ-027 và CR-REQ-028 ở `request-service`; phần task-service (TASK-REQ-027-02) nằm phía trên. Mục "Sau khi hợp nhất" cuối phần này ghi những gì đã nối với Solution, Approval và thư mục người dùng thật.

## Kết quả

| Task | Trạng thái | Ghi chú ngắn |
|---|---|---|
| 027-01 | DONE | migration `0060`; `solutions.seq` nullable ở `0060`, `NOT NULL` ở `0062` (quyết định 2) |
| 027-03 | DONE | thư viện JSON Schema đổi (xem spike) |
| 027-04 | DONE | bản chiếu; export Plan chưa (cần cây task-service) |
| 027-05 | DONE | |
| 027-06 | DONE | |
| 027-07 | một phần | phía Solution xong; Plan/Phase/khoá spec chưa vì `GeneratePlan`/`CommitPlan` còn là stub |
| 027-08 | một phần | 7 RPC thật, buf sạch, kiểm mẫu trong CI; thiếu kịch bản (d) `CommitPlan` |
| 028-01 đến 028-05 | DONE | |
| 028-06 | một phần | thiếu `AdvanceExecution` (chưa có executor) và metric |
| 028-07 | một phần | Solution xong; thiếu cổng Plan (chưa có handler Plan) |
| 028-08 | DONE | 11 RPC, bảy kịch bản trên hai dialect, buf sạch |

## Kiểm chứng đã chạy (thật)

- `go build ./... && go vet ./... && go test ./... -count=1` trong `backend-go/services/request-service`: PASS toàn bộ gói; `gofmt -l` sạch; `go test -race` trên `domain`, `usecase`, `adapter/grpc` PASS.
- Fuzz: `FuzzParseProjection` 25 giây (1,37 triệu lượt), `FuzzCanonicalJSON` 15 giây: không lỗi.
- Tích hợp (`-tags integration`) với container thật: `postgres:16-alpine`, `mysql:8.0`, `nats:2.10-alpine`. `./internal/adapter/postgres`, `./internal/adapter/mysql`, `./internal/adapter/eventbus`, `./cmd/server` PASS, gồm: migration up/down/up, backfill `seq`, hợp đồng cột, CHECK 12 trạng thái, chỉ mục một-open/một-live, RLS bằng role `NOSUPERUSER NOBYPASSRLS`, `RunArtifactContract` và `RunClarificationContract` (đua `AppendRequestRevision`, hai `RequestClarification`, hai bộ quét, 20 `MintSolutionID`, rollback đa bảng) trên **cả hai** dialect, consumer resume trên NATS thật, và `TestRun_ArtifactAndClarificationFlow_Postgres` (dịch vụ đầy đủ: gRPC, outbox relay, NATS, Postgres với RLS).
- Container chỉ do chính tôi tạo qua testcontainers (có reaper); không xoá container nào khác.

## Spike thư viện JSON Schema (TASK-REQ-027-03)

`github.com/google/jsonschema-go v0.4.3` (đã có ở `api-gateway`): hỗ trợ `$defs`, `$ref` nội bộ, `if/then`, `const/enum`, `pattern`, `uniqueItems`, `unevaluatedProperties` (draft 2020-12 đủ cho 8 schema), nhưng (1) `Validate` dừng ở lỗi đầu tiên, (2) thông điệp lỗi nêu đường dẫn **schema** chứ không phải JSON Pointer của dữ liệu, (3) không áp `format`. Không đáp ứng "trả đủ lỗi kèm JSON Pointer". `github.com/santhosh-tekuri/jsonschema/v6 v6.0.2` (MIT, Go thuần, đã có trong module cache, không thuộc họ `xeipuuv`): trả mọi lỗi với `InstanceLocation`, áp `format`, `unevaluatedProperties` đúng; được chọn. `go.mod` thêm `santhosh-tekuri/jsonschema/v6` và `gopkg.in/yaml.v3` (và `google.golang.org/protobuf` chuyển thành phụ thuộc trực tiếp); `go.sum` thêm các dòng tương ứng. Cập nhật Q1 của SOL-027: đủ draft 2020-12; chưa quyết xuất schema qua RPC cho frontend.

## Quyết định lệch so với task và lý do

1. **Số migration:** `0060_request_artifact_model`, `0061_clarifications_decisions` (dải `0060`..`0069`).
2. **`solutions.seq` nullable ở `0060`, `NOT NULL` ở `0062`:** lúc viết `0060` các `INSERT` Solution của nhánh khác chưa biết cột này. Sau hợp nhất mọi đường ghi đi qua `insertSolutionRow`, nơi cấp `MAX(seq)+1` dưới khoá hàng Request, nên `0062_solutions_seq_not_null` (up/down hai dialect, đã chạy) đặt `NOT NULL` sau khi lấp mọi `NULL` còn sót. MySQL dùng giá trị mặc định dạng biểu thức cho cột JSON (8.0.13+) để `INSERT` cũ không vỡ.
3. **`RequestContentWriter` là cổng riêng** (không thêm `UpdateContent` vào `RequestRepository`), để các fake của cổng hiện có không phải sửa. `Create` của repository ghi luôn 5 cột nội dung nên không cần `SeedContent`.
4. **Chốt chặn ghi nội dung** chỉ cho một file (`append_request_revision.go`), chặt hơn "ba file"; so khớp theo tên trường và tên biến (không có thông tin kiểu).
5. **`Violation`** dùng chung struct sẵn có ở `plan_proposal.go`, thêm `Path`. `ComputeInputDigest` đã tồn tại (`context_pack.go`) nên hàm của provenance tên `ComputeProvenanceInputDigest`. `NormalizeTitle` đã tồn tại nên hàm so khớp xác nhận Decision tên `NormalizeConfirmationText`.
6. **Quan hệ:** `StoredInRelationsTable` trả 4 quan hệ; CHECK của bảng vẫn cho cả `verifies` (xem task 027-06).
7. **Hết hạn và nhắc** dùng `ListDueRefs` + `LockOpenDue` (`SKIP LOCKED`) trong giao dịch của đúng tenant thay cho `ClaimExpired` một câu lệnh, để `ReturnToBacklog` lỗi thì Clarification vẫn `open`. Postgres quét liên tenant bằng policy `app.relay` (chỉ `SELECT` cho `clarifications`).
8. **`ConfirmRequestType` với Request chưa sẵn sàng** vẫn ghi `type_confirmed` và Approval `request_type` rồi mở Clarification; lặp lại lúc đang ở `awaiting_information` là no-op. Waiver ghi trong `meta` của revision (không thêm cột).
9. **Hook thoát khỏi `awaiting_information`** (đổi loại, huỷ Request, trả về backlog) cắm bằng `WithClarifications(...)` vào ba use case sẵn có (`typeChangeStatus` nhận thêm `awaiting_information`; `StageForStatus` có `awaiting_information`); cộng thêm, các test cũ không đổi (trừ số liệu ma trận chuyển trạng thái 12x18 và `TestParseRequestStatus_All`).
10. **README v6 mục 3.3** được sửa thêm `awaiting_information` (hợp đồng sẵn có `TestREADMEListsSameTypesAndStatuses`); `docs/crs/v6/README.md` chỉ đổi một câu. CR không đổi.
11. **Cấu hình** (`REQUEST_CLARIFICATION_*`, `REQUEST_DECISION_HIGH_RISK_SERVICES`) đọc ở `cmd/server/wire_artifact.go` bằng biến môi trường, không sửa `config.Config` dùng chung. `REQUEST_READINESS_AI_DRAFT` chưa làm.
12. **Phiên bản conflict** của Clarification/Decision là `FailedPrecondition` (như `REQUEST_VERSION_CONFLICT`); `common/apperrors` ở nhánh này chưa có `Aborted` (nhánh task-b thêm `KindAborted`; đổi sau khi hợp nhất nếu muốn).
13. **Proto:** không sửa `.proto`. Cần theo dõi: `ChooseSolutionOptionRequest.rationale` và phản hồi `decision_status`/`requires_confirmation` (28-07) thuộc nhánh Solution.

## Thay đổi ở nơi dùng chung / mã của feature khác và người gọi đã kiểm

- `domain.Request` (+5 trường), `domain.Solution` (+5 trường), `domain.Violation` (+`Path`), `domain.RequestStatus` (+`awaiting_information`, `IsHumanWaiting`), `AllTriggers` (+2), `NextStatus` (thêm nguồn cho `return_to_backlog`, `type_change`, `cancel`; không đổi chữ ký), `StageForStatus`. Người gọi `NextStatus`/`AllRequestStatuses`/`AllTriggers` kiểm bằng tìm tên đầy đủ: `transition_request.go`, `get_request_flow.go`, `request_flow_path.go`, test ma trận; không nơi nào giả định đúng 11/16.
- `usecase.TransitionInput` (+`ResumeStatus`), payload `status_changed` (+`resume_status`, tuỳ chọn), `CreateRequestInput` (+AC, `TypeFields`), `ConfirmRequestType` (+`WithReadiness`), `ReturnRequestToBacklog`/`CancelRequest`/`ChangeRequestType` (+`WithClarifications`). Chữ ký các hàm khởi tạo không đổi.
- `repository.Create` hai dialect (ghi cột nội dung), `requestColumns`/`scanRequest` (đọc cột nội dung), `contracttest.ExpectedColumns` (qua `withArtifactColumns`), `mysql/schema_contract_test.go` (bỏ cột sinh khỏi so sánh), `cmd/server/main.go` (một lời gọi `wireArtifact` + `artifact.attach`), `store_wiring.go` (+`artifact artifactStores`), `adapter/grpc/server.go` (+2 trường), `request_mapper.go`, `server_intake.go`.
- Không sửa module dùng chung (`common/*`, `proto/*`, `task-service`, `notification-service`) ở nhánh này.

## Điều chưa kiểm chứng

- Postgres 14 và MySQL đúng 8.0.16 (chỉ chạy Postgres 16 và `mysql:8.0` mới nhất); TiDB.
- Hiệu năng: `Validate` ở Plan 256 KB (mẫu nhỏ 16 µs), `GetArtifactGraph` với cây lớn, `ListPendingForUser` với nhiều team.
- `PlanTreeClient` với task-service thật (hướng cạnh `depends_on` là giả định theo tên trường); `GetArtifactGraph` chỉ chạy với `PlanTreeReader` giả.
- Hạn mặc định, 3 vòng, ngưỡng rủi ro cao, giới hạn kích thước: số đề xuất, chưa đo.
- Bản chiếu Markdown với đầu ra agent thật; kích hoạt lại thật (`GenerateSolution`, `AdvanceExecution` chưa tồn tại nên `AnalysisStarter`/`ExecutionAdvancer` mặc định ghi log và bỏ qua).
- Team và `role:admin` làm người nhận/trả lời: `PrincipalRecipients` và `AnswerClarification.WithTeams` có chỗ gắn nhưng chưa có triển khai thật (tenant-service/auth-service).

## Câu hỏi mở / việc cho người điều phối

1. Khi hợp nhất `rf/task-b`: chạy `scripts/check-artifact-samples.sh` (đã so tay: `canonical_cases.json` giống từng byte) và thêm vào workflow CI.
2. Nhánh Solution/Plan (rf/sol, rf/appr, rf/int-task): dùng `MintArtifactIDs`, `ValidateArtifactSemantics`, `ReplaceRequestCoverage`, `ApprovalGates`, `RecordDecision`, `AnswerClarification.WithSolutions`, `ResumeAfterClarification.WithAnalysis/WithExecution` (xem task 027-07, 028-06, 028-07). Nếu viết lại `solution_repository.go`, `INSERT` Solution không bắt buộc `seq` nhưng nên gọi `MintSolutionID` trong cùng giao dịch.
3. Hợp nhất nhiều nhánh sẽ xung đột nhỏ ở: `store_wiring.go`, `main.go`, `adapter/grpc/server.go`, `request_scan.go`, `expected_columns.go`, `plan_proposal.go` (struct `Violation`), `README.md` của service và các `tasks/README.md`, `solutions/README.md`.
4. Q3 SOL-027 (phạm vi khoá spec), Q1 SOL-028 (quyền hỏi tay và huỷ thủ công), Q3/Q4 SOL-028 vẫn mở; đề xuất đã cài: tự sinh Solution sau khi trả lời (`REQUEST_CLARIFICATION_AUTO_REGENERATE=true`), không tự sinh Plan.
5. `/dev/shm` (tmpfs, nơi `~/.cache/go-build` trỏ tới) đã đầy 16 GB trong lúc làm, làm mọi `go build` báo "no space left on device"; tôi dùng `GOCACHE` riêng trong scratchpad. Cần dọn cache chung nếu các agent khác gặp lỗi này.

## Sau khi hợp nhất (2026-10-08, `feat/request-flow-backend`)

**Xung đột đã giải:** `canonical_json_digest.go` giữ một bộ chuẩn tắc (`CanonicalJSON` ở `canonical_json.go`, trả `[]byte`; `parseStrictJSON` và `ErrInvalidJSONDocument` còn lại là lớp mỏng bọc lên nó cho `solution_options.go` và `analysis_evidence.go`; `DigestOptions` giữ nguyên giá trị băm). Các test `DigestOptions_*` của nhánh Solution (thứ tự khoá, cách viết số, khoá trùng, độ sâu, Unicode) vẫn xanh trên bộ chuẩn tắc mới. `domain.Solution` có cả trường của nhánh Solution lẫn `Seq`, `SchemaVersion`, `ProvenanceJSON`, `InputRequestRevision`, `ContentDigest`. Bản sao `canonical_cases.json` giống từng byte với task-service (`scripts/check-artifact-samples.sh`, đã thêm vào `backend-go-request-service.yml`). Hàm ánh xạ proto của Clarification đổi tên (`questionKindToProto`, `clarificationStatusToProto`) vì trùng tên với của Solution; hàm kiểm thử `contains` đổi thành `containsString`.

**Nối vào Solution (027-07 phía Solution, 028-07):**
- Repository Solution hai dialect đọc/ghi `seq`, `schema_version`, `provenance`, `input_request_revision`, `content_digest`; `Update` giữ giá trị cũ khi người gọi không đặt.
- `SolutionArtifactRecorder`: kiểm bao phủ AC lúc sinh (thiếu thì model được thử lại một lần), đóng dấu provenance, ghi chỉ mục `SOL-n.s` / `/opt-k`, cạnh `derived_from` và `supersedes`; `supersedes` cũng được ghi khi tạo lại (cạnh tới Solution cùng loại gần nhất đã bị thay, vì `GenerateSolution` đã đóng bản cũ trước khi worker chạy).
- `ChooseSolutionOption.WithDecisions` gọi `RecordDecision` trong cùng giao dịch (nhận `rationale`; trả `decision_status`, `requires_confirmation`). Chính sách tự chọn lấy `self_approval_allowed` của Approval đang chờ; không có Approval thì cho phép.
- Handler `solution` chặn `Approve` bằng `ApprovalGates.CheckSolution` (Decision `effective` đúng digest, câu hỏi `blocking` đã trả lời) và `CheckChosen` (phương án chọn trả lời mọi AC `active`). Tạo lại Solution hoặc `type_changed` đóng Decision của Solution bị thay.
- **Câu hỏi chặn không tự mở Clarification:** `open_questions[].blocking` chặn duyệt đến khi có Clarification `solution_open_question` (khoá `open_question:Q-n`, `source_ref` = id Solution) được trả lời; hiện người dùng tạo nó bằng RPC `RequestClarification`. Chưa tự mở vì sẽ chuyển Request sang `awaiting_information` giữa giai đoạn chờ duyệt; cần quyết định sản phẩm (Q1 của SOL-028).

**Nối vào 028-06:** `SolutionRegenerator` làm `AnalysisStarter`: `GenerateSolution.PrepareForSystem` tạo run trong giao dịch đánh dấu `processed_events`, worker chỉ chạy sau commit (nếu chạy trước, worker không thấy dòng run). Thiếu dev server thì bỏ qua có ghi log. Người nhận `team:`/`role:admin` và quyền trả lời của team dùng đúng `TeamMembershipResolver` và `AdminDirectoryResolver` của approval (không có địa chỉ dịch vụ thì thất bại đóng, người nhận đó bị bỏ qua). `ProposedSolutionSuperseder` thay chỗ giữ chỗ cho việc đóng Solution khi trả lời Clarification.

**`schemas/v1/solution.schema.json` chặt hơn `SolutionOptions.Validate`** (đòi `risk` và `hours_estimate` nguyên). Hai bên khác nhau nên schema không dùng để kiểm đầu ra model; muốn dùng thì phải sửa schema theo `Option` của nhánh Solution hoặc ngược lại (câu hỏi mở).

**Việc còn lại và không làm được ở nhánh này:** toàn bộ phía Plan/Phase của 027-07 (bước 4 đến 9) và `CheckPlan`; `ExecutionAdvancer`; metric; `model` trong provenance (cần `ProjectAICompleter` trả `CompleteResult`, đổi cổng của SOL-008 và SOL-026); tiêu chí "Solution `approved` không sửa `options`" (repository `Update` không chặn theo trạng thái, chỉ use case).

**Kiểm chứng sau hợp nhất:** xem báo cáo cuối của người thực hiện; lệnh chính: `go vet ./... && go vet -tags integration ./...`, `go test -race ./...`, `go test -tags integration ./internal/adapter/postgres ./internal/adapter/mysql ./internal/adapter/eventbus ./cmd/server`, `buf lint`/`buf breaking` cho `orca/request` và `orca/task`.
