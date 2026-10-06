# CR-REQ-027: Lược đồ có phiên bản và ontology cho Request, Solution, Plan, Phase, Task

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-027 |
| **Tên** | JSON Schema có `schema_version`, ID ổn định, quan hệ chuẩn, tiêu chí chấp nhận `AC-n`, bảng phủ yêu cầu, provenance, bản chiếu Markdown/YAML, kiểm tra ngữ nghĩa, bất biến sau duyệt |
| **Loại** | Feature (nền hợp đồng dữ liệu) |
| **Priority** | 🔴 P0 (CR-REQ-026 và CR-REQ-028 đứng trên CR này) |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-002 (bảng `requests`, `solutions`), CR-REQ-007 (schema `options`), CR-REQ-008 (tài liệu diagnosis, findings, answer), CR-REQ-011 (task `plan`/`phase`, `request_id`), CR-REQ-012 (`PlanProposal`, `CreatePlanTree`) |
| **Mở khoá** | CR-REQ-026 (adapter OpenSpec dùng bản chiếu), CR-REQ-028 (Definition of Ready dùng `ValidateRequestContent`, `request_revisions`), CR-REQ-029 (`TaskSpec` mở rộng Task schema v1) |
| **Tác động** | `request-service` (domain, usecase, adapter postgres/mysql, migration, proto); `task-service` (bảng `task_specs`, 3 RPC, migration); sửa nhỏ CR-REQ-002, 004, 007, 008, 011, 012 (mục 9) |

## 1. Bối cảnh và vấn đề

1. Mỗi CR hiện tự định nghĩa dạng JSON của mình: `options` ở CR-REQ-007 (có `schema_version: 1` nhưng chỉ cho `kind=solution`), ba tài liệu của CR-REQ-008, `PlanProposal` dạng proto ở CR-REQ-012. Không có registry chung, không có quy tắc nâng phiên bản, không có ID chéo giữa các thực thể ngoài `REQ-<number>`.
2. `Task.AIPlanJSON` (`task-service/internal/domain/task.go`) là chuỗi phản hồi AI thô, cột `ai_plan_json JSONB` (migration `0003`), không có schema. `Task` chưa có chỗ cho tiêu chí hoàn thành hay Check có cấu trúc (`Description`, `AIContext`, `PromptTemplate`, `Labels`).
3. README v6 mục 3.5 chưa có tiêu chí chấp nhận cho Request; vì vậy không ai trả lời được "task này phục vụ yêu cầu nào" và "yêu cầu nào chưa có task hay chưa có kiểm chứng".
4. Solution và Plan không ghi nguồn gốc: công cụ sinh, model, phiên bản prompt, đầu vào. `ai.complete` trả `{content, model}` (`agent/src/relay/ai-complete-handler.ts`), nhưng CR-REQ-007 bỏ `model` đi. Không điều tra được khi chất lượng giảm sau khi đổi prompt.
5. CR-REQ-026 (OpenSpec) và các công cụ khác cần một định dạng chiếu ra và đọc vào mà không để văn bản tự do thành nguồn sự thật.
6. Request đã duyệt rồi bị sửa tại chỗ sẽ làm Approval (gắn `subject_digest`) và mọi thứ sinh ra từ nó mất căn cứ.

CR này sở hữu: schema, ID, quan hệ, AC, bảng phủ, provenance, bản chiếu, kiểm tra ngữ nghĩa, quy tắc bất biến. Không sở hữu: nội dung phương án (CR-REQ-007), nội dung Plan (CR-REQ-012), `TaskSpec` đầy đủ gồm phạm vi, ràng buộc, Check chạy được (CR-REQ-029), hỏi lại (CR-REQ-028).

## 2. Giải pháp đề xuất

### 2.1 Registry schema (`request-service`)

- Tệp schema đặt ở `backend-go/services/request-service/schemas/v1/{request,solution,diagnosis,findings,answer,plan,phase,task}.schema.json` (mới), nhúng bằng `go:embed`, JSON Schema draft 2020-12.
- `schema_version` là số nguyên chính (major), mỗi `kind` một số riêng. Quy tắc: thêm trường tùy chọn không đổi số; đổi tên, đổi kiểu, thêm trường bắt buộc, siết enum thì tăng số và có hàm `Upgrade<Kind>V1ToV2` thuần. Người ghi luôn ghi phiên bản mới nhất; người đọc nhận mọi phiên bản đã phát hành, nâng khi đọc, không ghi lại bản cũ.
- `internal/domain/artifact_schema.go` (mới): `ValidateArtifact(kind, version, raw []byte) []Violation` trả danh sách lỗi có `path` (JSON Pointer) và `code`, không dừng ở lỗi đầu. Thư viện JSON Schema chưa có trong `go.mod` của repo (chỉ thấy `gopkg.in/yaml.v3` ở `api-gateway`; `xeipuuv/*` chỉ là phụ thuộc gián tiếp); chọn thư viện khi triển khai, ưu tiên loại hỗ trợ draft 2020-12 và không cần mạng.
- Kích thước tối đa mỗi tài liệu: Request 128 KB, Solution 64 KB (theo CR-REQ-007), Plan 256 KB, Task spec 32 KB. Số đề xuất, chưa đo.
- Chuẩn hóa chuỗi: mọi chuỗi tiếng Việt chuẩn hóa NFC trước khi băm hoặc so sánh (dấu tổ hợp khác dạng dựng sẵn cho cùng một chữ, làm digest lệch).

### 2.2 ID ổn định và `artifact_index`

| Thực thể | Dạng ID hiển thị | Cấp ở đâu | Ghi chú |
|---|---|---|---|
| Request | `REQ-<number>` | `requests.number` (CR-REQ-002) | giữ nguyên; thay đổi nội dung là `revision`, không đổi ID |
| Tiêu chí chấp nhận | `AC-<n>` | trong nội dung Request | duy nhất trong một Request, đánh số tăng dần, **không tái dùng** số của AC đã bị gỡ (đánh dấu `retired`); đầy đủ: `REQ-142#AC-2` |
| Solution | `SOL-<reqnum>.<seq>` | cột mới `solutions.seq` | `seq` tăng theo Request cho mọi `kind`; Option là `SOL-142.2/opt-1` (`opt-N` đã có ở CR-REQ-007) |
| Plan | `PLN-<reqnum>.<seq>` | `artifact_index` | mỗi lần replan là Plan mới, `seq` tăng |
| Phase | `PH-<reqnum>.<planseq>.<n>` | `artifact_index` | |
| Task làm việc | `TSK-<reqnum>.<planseq>.<n>` | `artifact_index` | `n` là thứ tự trong cây Plan lúc commit; không thay `TaskNumber` của `task-service` (README O2 chỉ bỏ số cho Plan, Phase) |

ID hiển thị chỉ là bí danh có thể tra ngược; khóa thật vẫn là UUID. Bảng `artifact_index` (mới, `request-service`): `tenant_id`, `display_id`, `kind` (`request|solution|option|plan|phase|task`), `request_id`, `artifact_id` (UUID, không FK với `task-service`), `created_at`; khóa chính `(tenant_id, display_id)`, chỉ mục `(tenant_id, artifact_id)`. Chỉ thêm, không sửa. RPC `ResolveArtifactRef(display_id)` trả `{kind, artifact_id, request_id}`.

### 2.3 Quan hệ chuẩn (`artifact_relations`)

Tám quan hệ, mỗi quan hệ có bộ ba (`from_kind`, `to_kind`) hợp lệ, kiểm ở `internal/domain/relation_rules.go` (mới):

| Quan hệ | Từ → Đến | Nơi lưu |
|---|---|---|
| `derived_from` | Solution → Request (kèm `revision`); Plan → Solution | `artifact_relations` |
| `implements` | Plan → Option (`SOL-142.2/opt-1`) | `artifact_relations` |
| `contains` | Plan → Phase, Plan hoặc Phase → Task | **không sao chép**: cạnh `parent_child` của `task-service` là nguồn, đọc qua `GetSubtree` |
| `depends_on` | Task → Task, Phase → Phase | **không sao chép**: cạnh `depends_on` của `task-service` |
| `verifies` | Check → AC; Check → Task | `request_coverage` (2.6) |
| `supersedes` | Solution → Solution; Plan → Plan | `artifact_relations` |
| `spawned_by` | Request → Request | `request_links` đã có (CR-REQ-002), chỉ đọc |
| `evidenced_by` | Solution → Evidence | `artifact_relations`; `to_id` là chuỗi tham chiếu, bảng Evidence thuộc CR sau (CR-REQ-030 dự kiến) |

`artifact_relations`: `id`, `tenant_id`, `request_id`, `rel` (CHECK 5 giá trị lưu ở đây), `from_kind`, `from_id`, `to_kind`, `to_id`, `created_by_run_id` NULL, `created_at`; UNIQUE `(tenant_id, rel, from_kind, from_id, to_kind, to_id)`; chỉ mục `(tenant_id, request_id)`. Chỉ thêm, không sửa. RPC `GetArtifactGraph(request_id)` hợp nhất bảng này, `request_links` và cây của `task-service` thành một danh sách cạnh duy nhất cho UI và MCP.

### 2.4 Request: tiêu chí chấp nhận và trường bắt buộc theo loại

Thêm vào `requests` (migration cho cả hai dialect; số thứ tự do CR-REQ-002 cấp): `content_schema_version INT NOT NULL DEFAULT 1`, `acceptance_criteria` (Postgres `JSONB`, MySQL `JSON`, ứng dụng luôn ghi giá trị, như `options` ở CR-REQ-002), `type_fields` (cùng kiểu), `content_revision INT NOT NULL DEFAULT 1`, `content_digest` (Postgres `TEXT`, MySQL `CHAR(64)`). `body` hiện có đóng vai "phát biểu vấn đề".

`acceptance_criteria` là mảng `{id:"AC-1", text (1..500), status:"active|retired", verify_hint:"test|metric|manual|review"}`; kèm bộ đếm `ac_next` trong cùng đối tượng gốc của `type_fields`.

Trường bắt buộc theo loại (`required_fields_by_type`, mã Go tĩnh, một nguồn cho CR-REQ-028):

| Loại | Khóa trong `type_fields` |
|---|---|
| `bug` | `repro_steps[]`, `actual`, `expected`, `environment`, `severity` (`low|medium|high|critical`) |
| `change_request`, `refactor` | `goal`, `value`, `scope_in[]`, `scope_out[]` |
| `security` | `affected_components[]`, `exploitability` (`low|medium|high`), `data_exposed` |
| `performance` | `metric`, `current_value`, `target_value`, `unit` |
| `ops_request` | `target_environment`, `window`, `rollback_plan` |
| `hotfix` | `production_impact`, `started_at` |
| `spike` | `question`, `time_box_hours` |
| `question` | `question` |
| `docs` | `audience`, `scope` |
| `task` | không có khóa riêng |

Hàm `ValidateRequestContent(type, content, level)` với `level` là `draft` hoặc `ready`. `draft` chỉ kiểm kiểu và kích thước (Request từ Jira thường thiếu AC, không được từ chối lúc tạo, CR-REQ-004). `ready` yêu cầu: `body` có ít nhất 20 ký tự không phải khoảng trắng, ít nhất một AC `active`, mọi khóa của loại có giá trị không rỗng. CR-REQ-028 là nơi gọi mức `ready`.

`request_revisions` (mới, bất biến, chỉ thêm): `id`, `tenant_id`, `request_id`, `revision INT`, `cause` (CHECK `created|clarification_answered|edited|type_changed`), `snapshot` JSON (`title`, `body`, `type`, `acceptance_criteria`, `type_fields`), `digest`, `actor_id` NULL, `actor_kind` (`ai|user|system`), `clarification_id` NULL, `created_at`; UNIQUE `(tenant_id, request_id, revision)`. Chỉ use case `AppendRequestRevision` được ghi các cột nội dung của `requests` (cùng tinh thần chốt chặn `TransitionRequest` của CR-REQ-003), trong một transaction với CAS `version`, tăng `content_revision` và phát `orca.request.request.revised` `{request_id, revision, cause, digest}`.

### 2.5 Solution, Plan, Phase, Task

**`solutions`** (migration): thêm `seq INT NOT NULL`, `schema_version INT NOT NULL DEFAULT 1`, `provenance` JSON NOT NULL, `input_request_revision INT NOT NULL`, `content_digest` (`TEXT` / `CHAR(64)`), UNIQUE `(tenant_id, request_id, seq)`. Schema `options` (CR-REQ-007 mục 2.3) bổ sung, tất cả tùy chọn ở v1 trừ khi ghi khác:
- `requirement_coverage[]`: `{ac_id, option_ids[], status:"covered|partial|out_of_scope", note}`. Bắt buộc có đủ mọi AC `active` cho phương án được chọn trước khi `Approve` (kiểm ở `SubjectHandler`, mã `REQUEST_ARTIFACT_AC_UNCOVERED_BY_OPTION`); `out_of_scope` bắt buộc có `note`.
- `constraints[]`, `non_functional[]` (`{kind: performance|security|availability|compat|other, text}`), `test_strategy`, `evidence_refs[]`.
- `open_questions` đổi từ `string[]` sang `{id:"Q-1", text, blocking:bool}`; `assumptions` từ `string[]` sang `{id:"A-1", text, needs_confirmation:bool}` (CR-REQ-007 chưa triển khai nên là sửa hợp đồng, không phải nâng phiên bản; CR-REQ-028 dùng hai cờ này).

**Plan và Phase** là task `type=plan|phase` ở `task-service`; nội dung có cấu trúc nằm trong `task_specs` (dưới) cùng `ai_plan_json` giữ phản hồi thô như CR-REQ-012. Plan v1: `{schema_version, goal, scope, risks[], rollback, verification_strategy, implements:{solution_id, option_id}, assumptions[], satisfies_all:bool}`. Phase v1: `{schema_version, goal, entry_criteria[], exit_criteria[], depends_on_phases[]}`. Điều kiện vào/ra là văn bản có thể kiểm chứng, CR-REQ-029 mới biến thành Check chạy được.

**Task v1** (tối thiểu để dựng bảng phủ; CR-REQ-029 mở rộng cộng thêm thành v2 mà không đổi tên): `{schema_version, objective, satisfies:["AC-1"], acceptance:["..."], checks:[{id:"c1", kind:"command|test|lint|typecheck|diff_rule|manual", description, command?, expect?}], exempt_from_coverage:false}`. `exempt_from_coverage=true` chỉ hợp lệ khi task mang nhãn `rollback` hoặc `check:*` (CR-REQ-012 mục 2.1).

**Chỗ lưu Task: bảng riêng `task.task_specs` ở `task-service`**, không thêm cột JSON vào `tasks`. Lý do: `tasks` nằm trên đường đọc nóng (`ListTasks`, Board, cây); thêm tới 32 KB JSON mỗi dòng làm phình kết quả `SELECT *`; MySQL JSON không lập chỉ mục được; `tasks` đã có hai cột văn bản lớn (`ai_context`, `ai_plan_json`).

| Cột | Postgres (schema `task`) | MySQL |
|---|---|---|
| `task_id` | `UUID PRIMARY KEY REFERENCES task.tasks(id) ON DELETE CASCADE` (như `0001_init`) | `CHAR(36) PRIMARY KEY` + FK cùng kiểu |
| `tenant_id` | `UUID NOT NULL` (RLS `tenant_isolation`) | `CHAR(36) NOT NULL` |
| `schema_version` | `INT NOT NULL` | `INT NOT NULL` |
| `spec` | `JSONB NOT NULL` | `JSON NOT NULL` |
| `digest` | `TEXT NOT NULL` | `CHAR(64) NOT NULL` |
| `locked_at` | `TIMESTAMPTZ NULL` | `TIMESTAMP(6) NULL` |
| `created_at`, `updated_at`, `version` | như quy ước | |

Migration kế tiếp sau `0016` của CR-REQ-011 (số chốt khi triển khai, hai dialect, có `down`). RPC mới của `task-service`: `SetTaskSpec(task_id, schema_version, spec_json, expected_version)` (từ chối khi `locked_at` khác NULL: `TASK_SPEC_LOCKED`), `GetTaskSpecs(task_ids[])`, `LockTaskSpecs(plan_task_id)`. `CreatePlanTreeRequest` (CR-REQ-012) thêm trường `spec_json` ở `PlanTreeTask`, `PlanTreePhase` và Plan để ghi spec trong cùng transaction.

**`request_coverage`** (mới, `request-service`): `id`, `tenant_id`, `request_id`, `plan_task_id`, `ac_id`, `task_id`, `check_id` NULL, `created_at`; chỉ mục `(tenant_id, request_id, plan_task_id)`. Được tính lại và thay thế nguyên khối trong transaction commit Plan (cùng transaction CAS `plan_task_id` của CR-REQ-012 bước 3). Bảng phủ là dữ liệu dẫn xuất: có thể dựng lại từ `task_specs` và nội dung Request. Kết quả chạy Check không nằm ở đây (CR-REQ-014 có `request_checks`, CR-REQ-029 có kết quả theo task).

### 2.6 Provenance

`solutions.provenance` và `provenance` trong Plan v1 cùng một cấu trúc:

```json
{"generator": {"kind": "native|openspec|human|mcp", "tool": "ai.complete", "model": "string", "model_source": "agent_response|param|unknown"},
 "prompt": {"template": "solution_prompt", "version": "v1", "digest": "sha256:..."},
 "run_id": "uuid", "attempt": 1,
 "input_digest": "sha256:...", "input_refs": [{"kind": "request", "id": "REQ-142", "revision": 2}],
 "actor": {"id": "string", "kind": "ai|user|system"}, "generated_at": "RFC3339"}
```

`model` lấy từ `content.model` của phản hồi `ai.complete`; với `agent.execPrompt` agent chỉ trả `{stdout, stderr, exitCode, timedOut}` nên `model` rỗng và `model_source=unknown`. `input_digest` là SHA-256 của JSON chuẩn tắc (khóa theo thứ tự chữ cái, không khoảng trắng, chuỗi NFC) gồm: ảnh chụp Request ở `revision`, digest các `prior_artifacts`, phiên bản mẫu prompt, và digest ngữ cảnh dự án. Mẫu prompt là hằng có phiên bản trong mã (`PromptVersion`), đổi mẫu thì đổi phiên bản. Provenance không có `credential_ref`, khóa hay `env`. Các CR 007, 008, 012 điền cấu trúc này khi lưu.

### 2.7 Bản chiếu Markdown/YAML và quy tắc phân tích

Bản chiếu là tệp văn bản sinh từ mô hình chuẩn, dùng để người và công cụ ngoài đọc. Mô hình chuẩn trong DB vẫn là nguồn sự thật.

```
---
orca_schema: 1
kind: solution
id: SOL-142.2
request: REQ-142@r2
status: proposed
supersedes: SOL-142.1
digest: sha256:...
generated_by: {kind: native, tool: ai.complete, model: "...", run: "..."}
---
# <tiêu đề>
<!-- orca:begin options digest=sha256:... -->
## Options
...
<!-- orca:end options -->
```

Quy tắc:
1. Phần mở đầu YAML ở dòng đầu tiên, bắt buộc có `orca_schema`, `kind`, `id`. Tập con YAML an toàn: không anchor, alias, tag tùy biến; tối đa 64 KB; số và chuỗi theo `yaml.v3` ở chế độ nghiêm ngặt (`KnownFields`).
2. Đề mục cố định theo `kind` (Solution: `Summary`, `Options`, `Requirement coverage`, `Assumptions`, `Open questions`; Plan: `Goal`, `Phases`, `Tasks`, `Risks`, `Rollback`). Vùng do Orca sở hữu nằm giữa cặp chú thích `<!-- orca:begin <section> -->` và `<!-- orca:end <section> -->`.
3. Dữ liệu nhập vào chỉ lấy từ: phần mở đầu YAML, khối ```` ```orca-json ```` trong vùng Orca, và các dòng khớp văn phạm của vùng đó (văn phạm `tasks.md` do CR-REQ-026 định nghĩa). Mọi văn bản khác là văn bản tự do, không suy ra dữ liệu từ đó.
4. Vùng Orca viết đè được từ mô hình chuẩn (`RenderArtifact`) bất kỳ lúc nào; vùng ngoài không bao giờ bị ghi đè.
5. Vòng đi về: `Parse(Render(x))` bằng `x` về nội dung chuẩn tắc; `Render` xác định (thứ tự khóa cố định, `\n`, NFC).
6. Đầu ra công cụ sai văn phạm: trả danh sách `Violation` có số dòng, không đoán sửa. Người gọi (use case sinh) quyết định thử lại, như CR-REQ-007 mục 2.4.
7. Nội dung đọc từ tệp trong repo của dự án là dữ liệu không tin cậy: giới hạn kích thước, loại ký tự điều khiển, không bao giờ thành lệnh hay đường dẫn.

RPC `ExportArtifactProjection(request_id, artifact_id, format)` (`format=markdown`) trả tệp; adapter công cụ ngoài dùng cùng hàm `RenderArtifact`.

### 2.8 Kiểm tra ngữ nghĩa (`artifact_semantic_validation.go`, mới)

Chạy sau kiểm cấu trúc, ở server, mỗi lần sinh và mỗi lần commit; CR-REQ-012 `ValidateProposal` gọi thêm hàm này.

| Mã (FailedPrecondition trừ khi ghi khác) | Điều kiện |
|---|---|
| `REQUEST_ARTIFACT_SCHEMA_INVALID` (InvalidArgument) | vi phạm JSON Schema; kèm danh sách `Violation` |
| `REQUEST_ARTIFACT_SCHEMA_VERSION_UNSUPPORTED` | `schema_version` lớn hơn bản đã biết |
| `REQUEST_ARTIFACT_DEPENDENCY_CYCLE` | vòng `depends_on` giữa Task hoặc Phase |
| `REQUEST_ARTIFACT_UNKNOWN_AC` | `satisfies` hay `ac_id` trỏ AC không tồn tại hoặc đã `retired` |
| `REQUEST_ARTIFACT_AC_NOT_COVERED` | AC `active` không có Task nào phủ (loại trừ Task `exempt_from_coverage`) |
| `REQUEST_ARTIFACT_TASK_NO_CHECK` | Task không miễn trừ mà `checks` rỗng |
| `REQUEST_ARTIFACT_TASK_NO_AC` | Task không miễn trừ mà `satisfies` rỗng |
| `REQUEST_ARTIFACT_AC_UNCOVERED_BY_OPTION` | phương án được chọn chưa trả lời đủ AC (2.5) |
| `REQUEST_ARTIFACT_LIMIT_EXCEEDED` | vượt giới hạn kích thước hoặc số phần tử |
| `REQUEST_ARTIFACT_RELATION_NOT_ALLOWED` | bộ ba (quan hệ, từ, đến) ngoài bảng 2.3 |

Mã cho các luật số phương án, hồi quy `bug`/`security`, giới hạn số Phase vẫn thuộc CR-REQ-007 và 012. Hai CR đó gọi lại hàm của CR này, không nhân đôi.

### 2.9 Bất biến sau duyệt

| Đối tượng | Quy tắc | Thi hành |
|---|---|---|
| Request | Nội dung đổi chỉ bằng `AppendRequestRevision` (revision mới, bản cũ giữ nguyên). Solution, Plan ghi `input_request_revision`; lệch revision hiện tại thì UI hiển thị "đã cũ" | use case duy nhất ghi cột nội dung; test kiến trúc kiểu CR-REQ-003 |
| Solution | `options` chỉ ghi khi `status=draft`; `chosen_option` chỉ đổi khi `proposed` (đúng CR-REQ-007). Sau `approved` chỉ còn đổi `status` thành `superseded`. Sửa nội dung là Solution mới (`seq` mới) kèm quan hệ `supersedes` | repository: `UPDATE ... WHERE status='draft'`; test |
| Plan, Phase, Task | Từ khi Approval `plan`/`task_list` được `approved`, `task_specs` của cây bị khóa (`LockTaskSpecs`, do consumer của `orca.request.approval.decided`), đổi thì replan (Plan mới, Plan cũ `superseded`, đúng CR-REQ-012). Task của Phase chỉ khóa khi Approval `phase` của nó được duyệt (CR-REQ-013) | `task-service`: `SetTaskSpec`, `UpdateTask` (title, description) từ chối `TASK_SPEC_LOCKED` khi có `locked_at`; trạng thái và tiến độ vẫn đổi được |

`subject_digest` của Approval (CR-REQ-009) giữ nguyên công thức riêng của từng handler; `content_digest` của CR này là digest nội dung tài liệu, không thay thế nó (Q4).

### 2.10 Proto, lỗi, sự kiện

- `proto/orca/request/v1/artifact.proto` (mới): `GetRequestCoverage`, `GetArtifactGraph`, `ExportArtifactProjection`, `ResolveArtifactRef`, `ListRequestRevisions`, `AppendRequestRevision` (nội bộ, CR-REQ-028 gọi; người dùng sửa Request dùng `EditRequestContent` với `expected_revision`, chỉ khi Request chưa `awaiting_*` duyệt và chưa qua `analyzing`). Thêm trường vào `Request` (`acceptance_criteria_json`, `type_fields_json`, `content_revision`, `content_schema_version`) và `Solution` (`display_id`, `seq`, `provenance_json`, `input_request_revision`, `content_digest`). `CreateRequestRequest` (CR-REQ-004) nhận `acceptance_criteria` và `type_fields_json` tùy chọn.
- `task.proto`: `SetTaskSpec`, `GetTaskSpecs`, `LockTaskSpecs`.
- Sự kiện: `orca.request.request.revised`. Không đổi sự kiện hiện có; payload không chứa nội dung Request.
- Quyền: đọc theo quyền đọc Request; `EditRequestContent` theo quyền ghi mức Request (câu hỏi còn mở của README v6 mục 8, CR-REQ-010).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| 1 | Mô hình chuẩn là JSON trong DB; Markdown/YAML chỉ là bản chiếu | Một nguồn sự thật; công cụ ngoài đổi không làm vỡ mô hình |
| 2 | `schema_version` là số nguyên chính, thêm trường tùy chọn không tăng số | Đơn giản; nâng cấp khi đọc bằng hàm thuần, dễ test |
| 3 | `task_specs` là bảng riêng, không phải cột JSON trên `tasks` | Đường đọc nóng của `tasks`; MySQL không lập chỉ mục JSON; khóa theo `locked_at` rõ ràng |
| 4 | `contains` và `depends_on` không sao chép sang `artifact_relations` | `task-service` đã là nguồn; sao chép sẽ lệch |
| 5 | ID hiển thị là bí danh bất biến, không thay `TaskNumber` | Không đụng README O2; Plan, Phase vẫn không có số trong `task-service` |
| 6 | Request đổi nội dung bằng revision, không thêm Request mới | Giữ `REQ-<number>` làm điểm tựa; Approval và Solution tham chiếu được revision |
| 7 | `draft` và `ready` là hai mức kiểm | Jira và GitHub không có AC; từ chối lúc tạo làm mất Request |
| 8 | Bản chiếu chỉ tin vùng Orca và khối `orca-json` | Văn bản tự do từ agent và từ Request là không tin cậy; tránh suy diễn từ prose |
| 9 | Khóa spec khi duyệt, không khóa trạng thái | Agent vẫn chạy và báo tiến độ trên cây đã duyệt |

## 4. Tiêu chí chấp nhận

- [ ] Schema của 8 `kind` có trong `schemas/v1`, mỗi `kind` có ít nhất một mẫu hợp lệ và mỗi luật có một mẫu sai trong `testdata/artifacts/`; `ValidateArtifact` trả đủ lỗi kèm JSON Pointer.
- [ ] `ValidateRequestContent` mức `draft` chấp nhận Request chỉ có `title` và `body`; mức `ready` trả đúng danh sách khóa thiếu cho cả 11 loại.
- [ ] Migration `requests`, `solutions`, `request_revisions`, `artifact_index`, `artifact_relations`, `request_coverage` và `task_specs` chạy lên và xuống trên Postgres 14+ và MySQL 8.0.1+.
- [ ] `AppendRequestRevision` tăng `content_revision` đúng một, ghi `request_revisions`, phát một sự kiện `revised`, và hai lần gọi đồng thời cho đúng một người thắng (CAS `version`).
- [ ] `AC-n` không bị tái dùng sau khi AC bị gỡ; Solution tham chiếu `REQ-142@r1` vẫn đọc được AC cũ.
- [ ] Commit Plan có Task thiếu `satisfies` hoặc thiếu `checks` bị `REQUEST_ARTIFACT_TASK_NO_AC` / `REQUEST_ARTIFACT_TASK_NO_CHECK`; AC không task nào phủ bị `REQUEST_ARTIFACT_AC_NOT_COVERED`; `request_coverage` được thay nguyên khối cùng transaction commit.
- [ ] `Approve` Solution khi phương án được chọn chưa trả lời mọi AC bị `REQUEST_ARTIFACT_AC_UNCOVERED_BY_OPTION`.
- [ ] Solution `approved` không sửa `options` được (repository trả lỗi); sửa nội dung bằng Solution mới có quan hệ `supersedes`.
- [ ] Sau `approval.decided` của Plan, `SetTaskSpec` trên cây trả `TASK_SPEC_LOCKED`; `UpdateTask` đổi `status` vẫn thành công.
- [ ] `Render` rồi `Parse` trả nội dung chuẩn tắc đồng nhất cho mọi mẫu; tệp có `orca-json` sai schema trả `Violation` có số dòng; văn bản ngoài vùng Orca không đổi dữ liệu (test với prose chứa JSON giả).
- [ ] Chuỗi tiếng Việt khác dạng chuẩn hóa cho cùng `input_digest`; `provenance` không chứa `credential_ref`, khóa hay `env`.
- [ ] `GetArtifactGraph` gộp đúng `contains` và `depends_on` từ `task-service` với `derived_from`, `implements`, `supersedes`.

## 5. Kiểm thử

- **Unit:** từng schema (golden hợp lệ và sai), `ValidateRequestContent` theo bảng 11 loại, bộ luật 2.8, `relation_rules`, canonical JSON và digest ổn định theo thứ tự khóa, nâng phiên bản giả (`V0ToV1` mẫu).
- **Property và fuzz (Go native):** `Parse(Render(x)) == x`; fuzz parser phần mở đầu YAML và khối `orca-json` (không panic, giới hạn kích thước).
- **Integration hai dialect:** migration, CAS revision, UNIQUE `(tenant_id, request_id, seq)`, thay thế nguyên khối `request_coverage` trong transaction, `task_specs` với FK cascade, khóa `locked_at`, Unicode tiếng Việt qua cột JSON.
- **Hợp đồng:** `buf breaking`; test mọi mẫu schema chạy trên cả `request-service` và `task-service` (hai bên dùng chung tệp mẫu).
- **Chưa kiểm chứng:** thư viện JSON Schema và chi phí kiểm ở tải thật; chiều dài thực tế của tài liệu Plan do AI sinh.

## 6. Rủi ro và điểm chưa kiểm chứng

- Thay đổi hợp đồng CR-REQ-007 (`open_questions`, `assumptions` thành đối tượng) ảnh hưởng prompt và bộ mẫu vàng; phải làm trước khi triển khai CR-REQ-007, không thì thành migration dữ liệu.
- AI sinh `satisfies` và `checks` sai (gán AC bừa) vẫn qua kiểm cấu trúc; chỉ người duyệt Plan phát hiện. Giảm thiểu: UI hiển thị bảng phủ (CR-REQ-021); đo tỉ lệ bị từ chối (CR-REQ-024).
- `TASK_SPEC_LOCKED` chặn sửa tay trên task của Plan đã duyệt có thể gây phiền; ranh giới "khóa gì" cần xác nhận (Q3).
- Số hiệu migration của ba bảng và của `task-service` chưa được cấp (CR-REQ-001, 002, 004, 006 đang dùng số chồng nhau, xem README `request-artifact-model`).
- Bản chiếu Markdown tăng bề mặt tấn công (nội dung do agent viết đọc lại). Đã hạn chế bằng 2.7 mục 3 và 7; chưa có kiểm thử với đầu ra agent thật.

## 7. Câu hỏi mở

1. Chọn thư viện JSON Schema nào, và có xuất schema qua RPC cho frontend dùng chung không.
2. `TaskNumber` có nên hiển thị cạnh `TSK-<...>` hay tách hẳn?
3. Phạm vi khóa sau duyệt Plan: chỉ `task_specs`, hay cả `title`, `description`, cạnh `depends_on` (cần sửa CR-REQ-011)?
4. Có hợp nhất công thức `subject_digest` của Approval với `content_digest` không.
5. `EditRequestContent` do người dùng gọi có cần Approval riêng khi Request đã qua `analyzing` không (hiện đề xuất: không cho sửa, phải đổi qua Clarification hoặc đổi loại).
6. CR-REQ-030 (Evidence) chưa tồn tại: `evidenced_by` có nên hoãn đến khi CR đó có không.

## 8. Tham chiếu

- `/opt/repos/orca/docs/research/receive-request/artifact-formats-ontology-and-execution-readiness.md` (phần A)
- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.5, 8; CR-REQ-002, 003, 007, 008, 009, 011, 012, 013
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go` (`AIPlanJSON`, `TaskNumber`), `migrations/postgres/0001_init.up.sql` (FK `ON DELETE CASCADE`), `0003_task_fields_and_comments.up.sql` (`ai_plan_json JSONB`)
- `/opt/repos/orca/agent/src/relay/ai-complete-handler.ts` (trả `{content, model}`)
- Mới: `request-service/schemas/v1/*.schema.json`, `internal/domain/artifact_schema.go`, `relation_rules.go`, `request_content_validation.go`, `artifact_semantic_validation.go`, `artifact_projection.go`, `internal/usecase/append_request_revision.go`, `internal/adapter/{postgres,mysql}/artifact_relation_repository.go`, `request_revision_repository.go`, `request_coverage_repository.go`; `task-service/internal/usecase/set_task_spec.go`, `lock_task_specs.go`, `internal/adapter/{postgres,mysql}/task_spec_repository.go`

## 9. Tác động tới CR hiện có (không sửa trong CR này; người duyệt series sửa theo bảng)

| CR | Cần sửa gì |
|---|---|
| README v6 | Mục 3.5: thêm các cột và bảng ở 2.4 đến 2.6; mục 3.6: các RPC ở 2.10; mục 3.7: sự kiện `request.revised` |
| CR-REQ-002 | `requests`: thêm 5 cột nội dung; thêm `request_revisions`, `artifact_index`, `artifact_relations`, `request_coverage`; `solutions`: thêm `seq`, `schema_version`, `provenance`, `input_request_revision`, `content_digest`; `options` mặc định phải là `'{}'` hoặc ứng dụng luôn ghi (đã là câu hỏi mở 4 của CR-REQ-007) |
| CR-REQ-004 | `CreateRequest` nhận `acceptance_criteria`, `type_fields_json`; tạo `request_revisions` revision 1 (`cause=created`) cùng transaction tạo Request |
| CR-REQ-007 | Mục 2.3: đổi `open_questions` và `assumptions` thành đối tượng; thêm `requirement_coverage` và các trường tùy chọn; mục 2.5 bước 5: ghi `provenance`, `input_request_revision`, `seq`, `content_digest`; mục 2.6: `Validate` của handler gọi kiểm AC của 2.5; `model` lấy từ phản hồi `ai.complete` |
| CR-REQ-008 | Ba tài liệu dùng `provenance` (`model_source=unknown`); `open_questions` cùng dạng `{id, text, blocking}` |
| CR-REQ-011 | Quy tắc khóa `TASK_SPEC_LOCKED` cho `UpdateTask`; số migration sau `0016` dành cho `task_specs` |
| CR-REQ-012 | `PlanProposal`, `PlanTreeTask`, `PlanTreePhase` thêm `spec_json` (hoặc trường `satisfies`, `acceptance`, `checks`); `ValidateProposal` gọi 2.8; bước COMMIT thay `request_coverage` và ghi `artifact_relations` (`implements`, `derived_from`, `supersedes`) trong transaction bước 3; `plan_subject_handler` khóa spec khi duyệt |
| CR-REQ-013 | `StartPhase`/duyệt Phase gọi `LockTaskSpecs` cho Task của Phase |
| CR-REQ-021 (frontend) | Hiển thị bảng phủ AC → Task → Check và nhãn "đã cũ" khi `input_request_revision` lệch |
