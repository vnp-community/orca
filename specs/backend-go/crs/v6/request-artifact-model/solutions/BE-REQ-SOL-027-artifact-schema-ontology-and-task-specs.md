# BE-REQ-SOL-027: Lược đồ có phiên bản, ontology, bảng phủ, `request_revisions` và `task.task_specs`

> **🚧 Đang triển khai: 5/8 task xong** (027-01, 03, 04, 05, 06 kiểm chứng 2026-10-08; 027-02 do nhánh task-b làm, chưa hợp nhất; 027-07 chưa làm, 027-08 một phần). Xem [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md). Phụ thuộc [BE-REQ-SOL-002](../../request-service-foundation/solutions/BE-REQ-SOL-002-request-data-model-and-repositories.md), [BE-REQ-SOL-007](../../solution-analysis/solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md), [BE-REQ-SOL-011](../../plan-phase-task/solutions/BE-REQ-SOL-011-task-service-plan-phase-task-types.md), [BE-REQ-SOL-012](../../plan-phase-task/solutions/BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md).

**CR:** [CR-REQ-027](../../../../../../docs/crs/v6/request-artifact-model/CR-REQ-027-artifact-schema-and-ontology.md)
**Service:** `request-service` (`schemas/v1`, `internal/domain`, `internal/usecase`, `internal/adapter/{postgres,mysql,grpc}`, `migrations/*`) · `task-service` (`internal/domain`, `usecase`, `adapter/{postgres,mysql,grpc}`, `migrations/*`) · `proto`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, khoá lạc quan, tenant), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (RPC giữa service, sự kiện), [`services/task-service.md`](../../../../tdd/services/task-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): CR-REQ-027, `docs/crs/v6/README.md` mục 3.5 và 8, các solution v6 đã có (`SOL-002/003/007/008/009/011/012/013`), `task-service/internal/domain/task.go` (`Description` dòng 91, `AIContext` 100, `AIPlanJSON string` 101, `TaskNumber` 129, `Labels` 157), `task-service/internal/usecase/{update_task.go,ports.go (TxRunner dòng 431)}`, `migrations/postgres/0001_init.up.sql` (FK `REFERENCES task.tasks(id) ON DELETE CASCADE`, RLS `tenant_isolation`), `0013_execution_leases.up.sql`, `migrations/mysql/0012_task_sources.up.sql` (FK `fk_task_sources_task ... ON DELETE CASCADE`), `adapter/postgres/task_sources.go` (mẫu `LinkSource`, SQLSTATE `23505`), `proto/orca/task/v1/task.proto` (không có RPC spec nào). `request-service` chưa có mã. Migration cao nhất của `task-service` hiện là `0014_task_sources_site`; SOL-011 dự kiến `0015`, `0016`.

### Correction relative to CR-REQ-027

| # | CR nói | Mã thật hoặc solution khác | Xử lý |
|---|--------|----------------------------|-------|
| C1 | "Thư viện JSON Schema chưa có trong `go.mod`, chỉ thấy `yaml.v3` ở `api-gateway`" | `services/api-gateway/go.mod` có **trực tiếp** `github.com/google/jsonschema-go v0.4.3` (dùng ở `internal/adapter/mcpserver/tools/{spec,fields,output_schema}.go`, cùng thư viện với `modelcontextprotocol/go-sdk`) | Đề xuất dùng lại thư viện này (không thêm họ `xeipuuv`). Chưa kiểm chứng mức hỗ trợ `$defs`, `if/then`, `unevaluatedProperties` của draft 2020-12; task 027-03 bắt đầu bằng spike 0,5 ngày với 8 schema thật, nếu thiếu thì chọn thư viện khác (Q1) |
| C2 | `task_specs` RLS `tenant_isolation` "như `0001_init`" | RLS của `task-service` là policy không có hiệu lực thật (BE-REQ-SOL-002 C2) | Mọi truy vấn `task_specs` phải có `tenant_id` trong `WHERE`; policy chỉ thêm cho đồng bộ, không dựa vào nó |
| C3 | Cột `solutions.seq NOT NULL` thêm bằng migration | BE-REQ-SOL-002 tạo `solutions` ở `0002_request_core`; bảng chưa có dữ liệu thật nếu service chưa phát hành | Nếu `0002` chưa merge: gộp cột vào `0002`. Nếu đã merge: migration mới `ADD COLUMN seq INT NULL`, backfill bằng `ROW_NUMBER() OVER (PARTITION BY tenant_id, request_id ORDER BY created_at, id)` (MySQL 8.0 có), rồi `SET NOT NULL` |
| C4 | `open_questions`, `assumptions` đổi từ `string[]` sang đối tượng | BE-REQ-SOL-007 mục B định nghĩa `SolutionOptions{... Assumptions, OpenQuestions []string}` và TASK-REQ-007-02 viết `Validate` theo đó | Sửa hợp đồng **trước** khi làm TASK-REQ-007-02; task 027-07 liệt kê chính xác chỗ sửa. Không sửa solution đã có trong phạm vi này |
| C5 | Số migration `request-service` do CR-REQ-002 cấp | CR-REQ-001/002/004/006 chồng số (README `request-artifact-model`) | Ghi `NNNN`; task 027-01 đọc thư mục thật lúc làm |
| C6 | `Task.AIPlanJSON` "chuỗi phản hồi AI thô, cột `ai_plan_json JSONB`" | Domain là `string` (dòng 101), cột `JSONB` ở migration `0003` | Giữ nguyên: `task_specs.spec` là nguồn cho nội dung có cấu trúc, `ai_plan_json` chỉ là bản thô |
| C7 | Sự kiện `orca.request.request.revised` | Quy ước subject `orca.request.<entity>.<event>` (README mục 8 điểm 4) | Đúng; thêm vào danh sách ở README `request-service-foundation` khi triển khai |

## 2. Giải pháp

### A. Cây thư mục

```
request-service/
  schemas/v1/{request,solution,diagnosis,findings,answer,plan,phase,task}.schema.json   (go:embed)
  internal/domain/artifact_kind.go            # ArtifactKind, DisplayID: Format*/Parse*
  internal/domain/artifact_schema.go          # ValidateArtifact, Upgrade<Kind>V1ToV2, Violation
  internal/domain/canonical_json_digest.go    # CanonicalJSON, NFC, Digest (SOL-007 có DigestOptions: dùng lại hàm lõi)
  internal/domain/acceptance_criteria.go      # AcceptanceCriterion, AC id không tái dùng
  internal/domain/request_content.go          # RequestContent, ApplyContent
  internal/domain/request_content_validation.go  # required_fields_by_type, ValidateRequestContent(draft|ready)
  internal/domain/relation_rules.go           # 8 quan hệ, bộ ba hợp lệ
  internal/domain/artifact_semantic_validation.go # 10 mã REQUEST_ARTIFACT_*
  internal/domain/provenance.go               # Provenance, InputDigest
  internal/domain/artifact_projection.go      # RenderArtifact, ParseProjection
  internal/domain/request_revision.go         # RequestRevision, RevisionCause
  internal/usecase/append_request_revision.go
  internal/usecase/edit_request_content.go
  internal/usecase/mint_artifact_ids.go       # artifact_index
  internal/usecase/replace_request_coverage.go
  internal/usecase/get_artifact_graph.go, resolve_artifact_ref.go, export_artifact_projection.go
  internal/adapter/{postgres,mysql}/{request_revision,artifact_index,artifact_relation,request_coverage}_repository.go
  internal/adapter/grpc/artifact_server.go
  migrations/{postgres,mysql}/NNNN_request_artifact_model.{up,down}.sql
  testdata/artifacts/{valid,invalid}/*.json
task-service/
  migrations/{postgres,mysql}/00NN_task_specs.{up,down}.sql
  internal/domain/task_spec.go
  internal/usecase/{set_task_spec,get_task_specs,lock_task_specs}.go
  internal/adapter/{postgres,mysql}/task_spec_repository.go
  internal/adapter/grpc/server_task_spec.go
proto/orca/request/v1/artifact.proto   proto/orca/task/v1/task.proto (sửa)
```

### B. Schema và kiểm ngữ nghĩa

Tám tệp schema nhúng bằng `//go:embed schemas/v1/*.json`; `ValidateArtifact(kind ArtifactKind, version int, raw []byte) []Violation` với `Violation{Path (JSON Pointer), Code, Message string; Line int}` không dừng ở lỗi đầu, giới hạn 50 lỗi. Hàm `Upgrade<Kind>V<N>ToV<N+1>(raw) ([]byte, error)` thuần; người đọc nâng khi đọc, người ghi luôn ghi bản mới nhất. Chuẩn hoá: mọi chuỗi qua NFC (`golang.org/x/text/unicode/norm`) **trước** khi băm hoặc so sánh. `CanonicalJSON(v any) ([]byte, error)`: khoá theo thứ tự chữ cái, không khoảng trắng; SOL-007 `DigestOptions` gọi lại hàm này để không có hai thuật toán digest.

Kiểm ngữ nghĩa (`ValidateArtifactSemantics(ctx ArtifactContext) []Violation`) trả mười mã `REQUEST_ARTIFACT_*` ở CR mục 2.8 (`SCHEMA_INVALID`, `SCHEMA_VERSION_UNSUPPORTED`, `DEPENDENCY_CYCLE`, `UNKNOWN_AC`, `AC_NOT_COVERED`, `TASK_NO_CHECK`, `TASK_NO_AC`, `AC_UNCOVERED_BY_OPTION`, `LIMIT_EXCEEDED`, `RELATION_NOT_ALLOWED`). `ArtifactContext{Request RequestContent; Solution *SolutionOptions; Plan *PlanSpecs}` là dữ liệu thuần; không gọi DB.

### C. ID hiển thị và `artifact_index`

`FormatRequestID(n) = "REQ-<n>"`, `FormatAC(reqNum, n) = "REQ-<n>#AC-<k>"` (AC lưu `AC-<k>` trong tài liệu), `FormatSolutionID(reqNum, seq) = "SOL-<n>.<seq>"`, `FormatOptionID = "SOL-<n>.<seq>/opt-<k>"`, `FormatPlanID = "PLN-<n>.<seq>"`, `FormatPhaseID = "PH-<n>.<planseq>.<k>"`, `FormatTaskID = "TSK-<n>.<planseq>.<k>"`; mỗi hàm có `Parse*` đối ngẫu. `MintArtifactIDs` ghi `artifact_index(tenant_id, display_id, kind, request_id, artifact_id, created_at)` trong **cùng transaction** với việc tạo thực thể (Solution: `seq = MAX(seq)+1` theo Request, chốt bằng UNIQUE `(tenant_id, request_id, seq)` và retry khi va chạm). Chỉ thêm, không sửa. `TSK-` không thay `TaskNumber` của `task-service`.

### D. Migration

`request-service` `NNNN_request_artifact_model` (hai dialect; Postgres `JSONB`/`TIMESTAMPTZ`/schema `request.`, MySQL `JSON`/`TIMESTAMP(6)`/`CHAR(64)` cho digest):

```sql
ALTER TABLE request.requests
  ADD COLUMN content_schema_version INT NOT NULL DEFAULT 1,
  ADD COLUMN acceptance_criteria JSONB NOT NULL DEFAULT '[]',   -- MySQL: JSON, ứng dụng luôn ghi giá trị (không DEFAULT literal)
  ADD COLUMN type_fields JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN content_revision INT NOT NULL DEFAULT 1,
  ADD COLUMN content_digest TEXT NOT NULL DEFAULT '';           -- MySQL: CHAR(64)
CREATE TABLE request.request_revisions (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, request_id UUID NOT NULL,
  revision INT NOT NULL,
  cause TEXT NOT NULL CHECK (cause IN ('created','clarification_answered','edited','type_changed')),
  snapshot JSONB NOT NULL, digest TEXT NOT NULL,
  actor_id TEXT NULL, actor_kind TEXT NOT NULL CHECK (actor_kind IN ('ai','user','system')),
  clarification_id UUID NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, request_id, revision));
CREATE TABLE request.artifact_index (...);       -- PK (tenant_id, display_id); INDEX (tenant_id, artifact_id)
CREATE TABLE request.artifact_relations (...);   -- UNIQUE (tenant_id, rel, from_kind, from_id, to_kind, to_id)
CREATE TABLE request.request_coverage (...);     -- INDEX (tenant_id, request_id, plan_task_id)
ALTER TABLE request.solutions ADD COLUMN seq INT NULL, ADD COLUMN schema_version INT NOT NULL DEFAULT 1,
  ADD COLUMN provenance JSONB NOT NULL DEFAULT '{}', ADD COLUMN input_request_revision INT NOT NULL DEFAULT 1,
  ADD COLUMN content_digest TEXT NOT NULL DEFAULT '';   -- rồi backfill seq và SET NOT NULL, UNIQUE (tenant_id, request_id, seq)
```

`task-service` `00NN_task_specs` (số sau `0016`, hai dialect): `task.task_specs(task_id UUID PK REFERENCES task.tasks(id) ON DELETE CASCADE, tenant_id, schema_version INT, spec JSONB, digest TEXT, locked_at TIMESTAMPTZ NULL, created_at, updated_at, version BIGINT)`; MySQL `CHAR(36)`, `JSON`, `CHAR(64)`, `fk_task_specs_task`. Down: `DROP TABLE`. `request_coverage` là dữ liệu dẫn xuất, thay nguyên khối.

### E. Use case

- `AppendRequestRevision.Execute(ctx, in{RequestID, Content RequestContent, Cause, ActorID, ActorKind, ClarificationID, ExpectedVersion})`: `RequireTenantID`; `repo.Get`; `ValidateRequestContent(type, content, draft)`; `Digest`; một transaction: `request_revisions` revision `content_revision+1`, `UPDATE requests SET title, body, type?, acceptance_criteria, type_fields, content_revision, content_digest, version=version+1 WHERE ... AND version=?` (CAS, `REQUEST_VERSION_CONFLICT`), outbox `orca.request.request.revised {request_id, revision, cause, digest}`. **Chỉ** use case này được gán các cột nội dung (test kiểu `status_write_guard_test.go` của SOL-003: `go/parser` cấm gán `.Title`, `.Body`, `.AcceptanceCriteria`, `.TypeFields`, `.ContentRevision` ngoài `append_request_revision.go`).
- `EditRequestContent` (RPC người dùng): chỉ khi Request chưa vào `awaiting_*` duyệt và chưa qua `analyzing`; `expected_revision`; gọi `AppendRequestRevision(cause=edited)`.
- `ReplaceRequestCoverage(ctx, tx, requestID, planTaskID, rows)`: `DELETE ... WHERE request_id AND plan_task_id` rồi `INSERT` nhiều hàng trong transaction của `CommitPlan` (SOL-012 bước CAS `plan_task_id`).
- `GetArtifactGraph(requestID)`: hợp nhất `artifact_relations`, `request_links` và `GetSubtree` của task-service (`contains`, `depends_on`) thành danh sách cạnh `{rel, from, to}` có thứ tự xác định.

### F. `task-service`

`SetTaskSpec(task_id, schema_version, spec_json, expected_version)`: kiểm `schema_version` đã biết, kích thước ≤ 32 KB, `locked_at != NULL` thì `TASK_SPEC_LOCKED` (`FailedPrecondition`); `Upsert` CAS theo `version`. `GetTaskSpecs(task_ids[])` (tối đa 200, thiếu thì bỏ qua). `LockTaskSpecs(plan_task_id)`: `GetSubtree`, `UPDATE task_specs SET locked_at = now WHERE task_id IN (...) AND locked_at IS NULL` (idempotent). `UpdateTask` (Title, Labels...) từ chối khi `locked_at` khác NULL **chỉ với** trường nội dung (`title`; `description` chưa có trong `UpdateTaskInput`, xem Q3), trạng thái và tiến độ vẫn đổi được. `CreatePlanTreeRequest` thêm `string spec_json` cho Plan, `PlanTreePhase`, `PlanTreeTask`: ghi trong cùng `RunInTx` (`TxRunner.RunInTx` đã cung cấp `TaskRepository`/`EdgeRepository` theo giao dịch; thêm `TaskSpecRepository` vào chữ ký `fn` là thay đổi chữ ký có ảnh hưởng tới SOL-011/012 (task 027-02 xử lý)).

### G. Điểm nối với solution khác (không sửa tài liệu đó)

| Solution | Điểm nối | Task |
|---|---|---|
| SOL-002 | Thêm cột và bảng ở D; `domain.Request` thêm 5 trường | 027-01, 027-05 |
| SOL-004 | `CreateWithinTx` ghi `request_revisions` revision 1 (`cause=created`) và nhận `acceptance_criteria`, `type_fields_json` | 027-05 |
| SOL-007 | `SolutionOptions` có `requirement_coverage`, `constraints`, `non_functional`, `test_strategy`, `evidence_refs`, `OpenQuestion{id,text,blocking}`, `Assumption{id,text,needs_confirmation}`; ghi `provenance`, `seq`, `input_request_revision`, `content_digest`; `Validate` của handler gọi `AC_UNCOVERED_BY_OPTION` | 027-07 |
| SOL-012 | `TaskProposal.Satisfies/Acceptance/Checks`; `ValidateProposal` gọi 027-06; `CommitPlan` thay `request_coverage`, ghi `artifact_relations`, tạo `PLN/PH/TSK` | 027-07 |
| SOL-013 | `StartPhase` gọi `LockTaskSpecs` khi duyệt Phase | 027-07 |
| SOL-009 | Consumer `orca.request.approval.decided` (plan/task_list approved) gọi `LockTaskSpecs` | 027-07 |

### H. Proto, lỗi, sự kiện, quyền

`artifact.proto`: `GetRequestCoverage`, `GetArtifactGraph`, `ExportArtifactProjection`, `ResolveArtifactRef`, `ListRequestRevisions`, `EditRequestContent`; `AppendRequestRevision` chỉ là use case nội bộ (không RPC công khai). Thêm vào `Request` và `Solution` các trường ở CR 2.10. `task.proto`: `SetTaskSpec`, `GetTaskSpecs`, `LockTaskSpecs`. Mã lỗi: mười mã `REQUEST_ARTIFACT_*`, `REQUEST_REVISION_CONFLICT` (hoặc dùng `REQUEST_VERSION_CONFLICT`), `REQUEST_CONTENT_NOT_EDITABLE`, `TASK_SPEC_LOCKED`, `TASK_SPEC_INVALID`, `TASK_SPEC_VERSION_CONFLICT`. Sự kiện `orca.request.request.revised` (payload không chứa nội dung). Quyền: đọc theo quyền đọc Request; `EditRequestContent` theo quyền ghi mức Request (chưa chốt, README mục 8); `SetTaskSpec`, `LockTaskSpecs` chỉ gọi từ service (`request-service`), kiểm danh tính nội bộ như `ReportTaskExecutionResult` (chưa có, ghi rủi ro).

### I. Mã lỗi tóm tắt

| Mã | Kind | Khi nào |
|---|---|---|
| `REQUEST_ARTIFACT_SCHEMA_INVALID` | InvalidArgument | vi phạm JSON Schema, kèm danh sách `Violation` |
| `REQUEST_ARTIFACT_SCHEMA_VERSION_UNSUPPORTED` | FailedPrecondition | `schema_version` lớn hơn bản đã biết |
| `REQUEST_ARTIFACT_AC_NOT_COVERED` | FailedPrecondition | AC `active` không có task nào phủ |
| `REQUEST_ARTIFACT_RELATION_NOT_ALLOWED` | FailedPrecondition | bộ ba quan hệ ngoài bảng CR 2.3 |
| `REQUEST_CONTENT_NOT_EDITABLE` | FailedPrecondition | sửa Request đã qua `analyzing` hoặc đang chờ duyệt |
| `TASK_SPEC_LOCKED` | FailedPrecondition | ghi spec của task thuộc Plan đã duyệt |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| 1 | Mô hình chuẩn là JSON trong DB, Markdown/YAML chỉ là bản chiếu | Một nguồn sự thật |
| 2 | `task_specs` là bảng riêng ở `task-service` | `tasks` là đường đọc nóng; MySQL JSON không lập chỉ mục |
| 3 | `contains`/`depends_on` không sao chép sang `artifact_relations` | `task-service` đã là nguồn |
| 4 | `request_coverage` thay nguyên khối trong transaction commit Plan | Dữ liệu dẫn xuất, dựng lại được |
| 5 | Chỉ một use case ghi cột nội dung Request, có test parser chặn | Cùng tinh thần chốt chặn `status` của SOL-003 |
| 6 | Digest dùng một hàm `CanonicalJSON` chung | Hai DB lưu JSON khác nhau; không hai thuật toán |
| 7 | Dùng `google/jsonschema-go` nếu spike đạt | Đã có trong repo, không thêm phụ thuộc |

## 4. Phụ thuộc và thứ tự

Cần SOL-002 (bảng `requests`, `solutions`), SOL-011 (`request_id` trên `tasks`, `TxRunner`), SOL-012 (`CreatePlanTree`, `CommitPlan`) ở mức cổng; làm phần domain, migration, schema trước. Mở khoá: BE-REQ-SOL-026 (bản chiếu, `RenderArtifact`), BE-REQ-SOL-028 (`ValidateRequestContent`, `AppendRequestRevision`), CR-REQ-029 (`TaskSpec` v2). Thứ tự task: 01 migration request-service; 02 `task_specs` và ba RPC ở task-service; 03 schema registry; 04 bản chiếu; 05 nội dung Request và revision; 06 ID, quan hệ, ngữ nghĩa; 07 nối SOL-007/012/013/009; 08 proto, gRPC và tích hợp. Khuyến nghị: làm 03 đến 05 **trước** TASK-REQ-007-02 và TASK-REQ-012-03.

## 5. Kiểm thử

- **Unit:** mỗi schema có mẫu hợp lệ và mỗi luật một mẫu sai trong `testdata/artifacts/`; `ValidateRequestContent` 11 loại hai mức; mười mã ngữ nghĩa; `relation_rules`; `CanonicalJSON` ổn định theo thứ tự khoá và NFC; `Format*`/`Parse*` vòng đi về.
- **Property và fuzz:** `Parse(Render(x)) == x`; fuzz parser phần mở đầu YAML và khối `orca-json` (không panic, giới hạn kích thước).
- **Integration hai dialect (`-tags=integration`):** migration lên/xuống; CAS `AppendRequestRevision` (hai goroutine, một thắng); UNIQUE `(tenant_id, request_id, seq)`; `ReplaceRequestCoverage` rollback khi lỗi giữa chừng; `task_specs` FK cascade, `locked_at`; tiếng Việt có dấu qua cột JSON; RLS bằng role không bypass.
- **Hợp đồng:** `buf breaking`; mọi mẫu schema chạy ở cả `request-service` và `task-service`.
- **Chưa kiểm chứng:** năng lực thư viện JSON Schema (spike); chi phí kiểm ở tải thật; độ dài Plan do AI sinh.

## 6. Rủi ro và điểm chưa kiểm chứng

- Đổi hợp đồng SOL-007 (C4) sau khi TASK-REQ-007-02 đã viết thì thành migration dữ liệu.
- AI gán `satisfies` bừa vẫn qua kiểm cấu trúc; chỉ người duyệt phát hiện (UI bảng phủ ở CR-REQ-021).
- `TASK_SPEC_LOCKED` có thể gây phiền khi người dùng muốn sửa nhỏ; ranh giới "khóa gì" chưa chốt (Q3).
- Thêm tham số vào `TxRunner.RunInTx` của task-service chạm mọi nơi gọi (`AIApply`, `CreatePlanTree`); đề xuất cổng riêng `TaskSpecWriter` nhận ctx giao dịch thay vì đổi chữ ký.
- Số migration của hai service chưa cấp; RLS `task-service` không có hiệu lực.

## 7. Câu hỏi mở

1. Thư viện JSON Schema: `google/jsonschema-go` có đủ không; có xuất schema qua RPC cho frontend không.
2. Hiển thị `TaskNumber` cạnh `TSK-<...>` hay tách hẳn.
3. Phạm vi khoá sau duyệt Plan: chỉ `task_specs` hay cả `title`, `description`, cạnh `depends_on`.
4. Hợp nhất `subject_digest` của Approval với `content_digest` hay giữ riêng.
5. `EditRequestContent` có cần Approval riêng khi đã qua `analyzing` không.
6. `evidenced_by` hoãn tới khi CR-REQ-030 (Evidence) có.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/request-artifact-model/CR-REQ-027-artifact-schema-and-ontology.md`; `/opt/repos/orca/docs/research/receive-request/artifact-formats-ontology-and-execution-readiness.md` (phần A)
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go`, `internal/usecase/{update_task.go, ports.go}`, `internal/adapter/postgres/task_sources.go`, `migrations/{postgres,mysql}/0001_init.up.sql`, `0012_task_sources.up.sql`, `0013_execution_leases.up.sql`
- `/opt/repos/orca/backend-go/services/api-gateway/go.mod` (`github.com/google/jsonschema-go v0.4.3`), `internal/adapter/mcpserver/tools/spec.go`
- `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto`
- `/opt/repos/orca/agent/src/relay/ai-complete-handler.ts` (trả `{content, model}`)
- Solution liên quan: `../../request-service-foundation/solutions/BE-REQ-SOL-002-*.md`, `../../request-lifecycle/solutions/BE-REQ-SOL-003-*.md`, `../../solution-analysis/solutions/BE-REQ-SOL-007-*.md`, `../../plan-phase-task/solutions/BE-REQ-SOL-011-*.md`, `BE-REQ-SOL-012-*.md`, `BE-REQ-SOL-013-*.md`
