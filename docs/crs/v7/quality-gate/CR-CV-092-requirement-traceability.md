# CR-CV-092 — Truy vết yêu cầu: Request/Task/Plan ↔ thay đổi ↔ test

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-092 |
| **Tên** | `RequirementTrace`: nối thay đổi của worktree với yêu cầu (Task hiện có, và Request/Plan/Phase của v6 khi có), ghép bằng nhãn có mức tin cậy, hiển thị "tiêu chí nào chưa thấy bằng chứng"; RPC `GetRequirementTrace` |
| **Loại** | Feature (tích hợp liên service; chạy độc lập được ở mức Task, mở rộng khi có v6) |
| **Priority** | ⚪ P2 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-085 (cổng, quyền, cờ), CR-CV-036 (overlay: tệp/symbol đổi, `tested`), CR-CV-082 (`QualityRun` cho bằng chứng kiểm tra), CR-CV-012 (binding worktree), CR-CV-013. **Không** phụ thuộc cứng v6; nhánh Request chỉ bật khi có `request-service` (CR-REQ-027) |
| **Mở khoá** | CR-CV-087 (hiển thị), CR-CV-090 (một phần báo cáo), CR-CV-093 (ngữ cảnh cho tóm tắt) |
| **Tác động** | `backend-go/services/code-intel-service` (usecase, port `RequirementProvider`, adapter gọi `project-service`/`task-service`/`request-service`, bảng `requirement_trace_links`, migration hai dialect), `backend-go/proto/orca/codeintel/v1/requirement_trace.proto`, `api-gateway` (kênh `codeIntel.quality.trace`), frontend (lens/ngăn "Yêu cầu" do CR-CV-087 vẽ). Nguồn: [research 11](../../../research/view-code/11-additions-for-quality-control.md) C3 |

---

## 1. Bối cảnh và vấn đề

Research 11 C3 muốn trả lời "yêu cầu gì → phần code nào → test nào" và chỉ ra tiêu chí chưa có bằng chứng. Research ghi "cần dữ liệu tiêu chí chấp nhận có cấu trúc; hiện v6 chỉ là đề xuất". Đã đọc code ngày 2026-10-06:

### 1.1 Dữ liệu yêu cầu **đã có thật**

| Dữ liệu | Nơi | Ghi chú |
|---|---|---|
| `Task` | `backend-go/services/task-service/internal/domain/task.go` | `Title`, `Description`, `Type` (`task\|bug\|feature\|epic`), `AIContext` (chuỗi), `AIPlanJSON` (chuỗi), `WorktreeID`, `AgentSessionID`, `TaskNumber` (số tuần tự theo project, "#TG-42"), `PRURL`, `ParentID` (cây). **Không có trường tiêu chí chấp nhận có cấu trúc** (grep `acceptance` trong `task-service`: chỉ chú thích trong `execute_task.go`) |
| Nguồn ngoài | `task.task_sources` (`task_id`, `provider ∈ jira\|linear\|github\|gitlab`, `ref`, `url`, `site`; migration `0012`, `0014`); RPC `GetTaskSource(task_id)` (`task.proto:21`) | **Chỉ tra thuận theo `task_id`**; không có RPC tra ngược từ `(provider, ref)` (`CreateTaskFromSource` là idempotent theo khoá đó nhưng là RPC ghi) |
| Liên kết worktree | `project.proto` `Worktree`: `task_id` (`:543`), `linked_issue_provider/ref/site` (`:531`, `:556`), `orchestration_run_id`; `RecordWorktreeCreatedRequest` mang `linked_issue_*` | Đây là cầu **worktree → task/issue** đáng tin nhất hiện có (CR-TG-008) |
| Tìm theo số | `FindTaskByNumber(project, "#TG-N")` (`task.proto:47`, `usecase/find_task_by_number.go`) | Chú thích `TaskNumber`: "để commit message có thể tham chiếu `#TG-42`"; **không thấy ai phân tích commit để dùng nó** (grep `#TG-` ngoài `task-service` và proto sinh: không có). `Task.PRURL` do "saga ghi ngược khi tạo PR" nêu trong chú thích; chưa kiểm chứng saga có tồn tại |
| Trailer commit | `git-gateway-service/internal/usecase/commit_message_prompt.go`: khi có `issueRef`, nhắc AI thêm dòng `Refs: <issueRef>` cuối thông điệp | Chỉ khi người dùng dùng "sinh commit message bằng AI"; không bắt buộc |
| Thực thi | `task.execution_links` (engine, `external_ref_id`, `status_mirror`), `orchestration.coordinator_runs`, `dispatch_contexts` | Cho biết task nào đã chạy; **không** cho biết tiêu chí nào được đáp ứng |
| Lưu ý quyền | `Server.GetTask` (`task-service/internal/adapter/grpc/server.go:149`) gọi thẳng `getTask.Execute(ctx, id)`; chưa thấy kiểm grant ở lớp này (có thể ở use case — chưa đọc) | Phải kiểm `ResolvePermission` (`task.proto:30`) trước khi trả nội dung task cho người dùng khác |

### 1.2 Dữ liệu v6 (đề xuất, **chưa có**)

CR-REQ-027 (`docs/crs/v6/request-artifact-model/`) định nghĩa: `requests.acceptance_criteria[] = {id:"AC-1", text, status:"active|retired", verify_hint:"test|metric|manual|review"}`; Task v1 `{objective, satisfies:["AC-1"], acceptance:[…], checks:[{id, kind:"command|test|lint|typecheck|diff_rule|manual", …}], exempt_from_coverage}` lưu ở `task.task_specs`; `Solution.requirement_coverage[]`; RPC `GetRequestCoverage`, `GetArtifactGraph`, `ResolveArtifactRef` (`artifact.proto`). Nghĩa là v6 *đã thiết kế* bảng phủ yêu cầu ↔ Task nhưng dừng ở mức Task; **không biết code hay test thật**. CR này bổ sung nửa còn lại: Task/AC ↔ thay đổi ↔ test/kiểm tra.

### 1.3 Vấn đề

- Chưa có cách biết thay đổi trong worktree thuộc yêu cầu nào, và yêu cầu nào còn chưa có dấu vết nào trong thay đổi/test.
- Với Task hiện có, "tiêu chí" chỉ là văn bản tự do trong `Description`; mọi suy luận từ đó là ước đoán và phải gắn nhãn như vậy.
- Không được biến "có dấu vết" thành "đã đáp ứng": một test chạm tên tiêu chí không chứng minh tiêu chí đạt.

## 2. Giải pháp đề xuất

### 2.1 Hai nguồn yêu cầu sau một cổng `RequirementProvider`

Port ở `code-intel-service/internal/usecase/ports.go`: `Resolve(ctx, binding) (RequirementSet, error)`. Hai adapter, cái nào có thì dùng, cả hai có thì **ưu tiên có cấu trúc** (v6) và hạ nguồn kia xuống nguồn phụ:

| Adapter | Khi có | Yêu cầu | Độ tin cậy gốc |
|---|---|---|---|
| `task_requirement_provider` (mới, **độc lập v6**) | Worktree có `task_id` hoặc `linked_issue_*` → task (mục 2.2) | Tiêu chí rút từ `Task.Description` (mục 2.3), hoặc một yêu cầu duy nhất = tiêu đề task | `inferred`/`derived` |
| `request_requirement_provider` (mới, **chỉ khi có `request-service`**) | Task thuộc Request v6 (`artifact_relations`/`ResolveArtifactRef`) | AC có `id` từ `requests.acceptance_criteria`, `satisfies` của Task/Phase, `checks[]` | `explicit` |

Nhánh v6 bật bằng cờ cấu hình `CODEINTEL_REQUIREMENT_SOURCE_REQUEST_ENABLED` (mặc định `false`) **và** kiểm tra ở lúc chạy rằng `request-service` trả được RPC; thiếu thì rơi về nhánh Task kèm `warnings: ["request_service_unavailable"]`. Không import mã v6 trực tiếp; chỉ gọi gRPC qua adapter (không FK chéo service, README v7 mục 6). Chi tiết ID CR-REQ chịu trách nhiệm: CR-REQ-027 (schema AC, `GetRequestCoverage`), CR-REQ-029 (hợp đồng thực thi, `checks`), CR-REQ-012/013 (Plan/Phase/Task).

### 2.2 Tìm yêu cầu từ worktree (độ tin cậy `linkConfidence`)

Thứ tự thử, dừng ở kết quả đầu có giá trị:

1. **`explicit`**: `project-service.GetWorktree` → `task_id` có giá trị → `task-service` (đã kiểm `ResolvePermission(read)`) lấy task; hoặc `Request` qua v6.
2. **`derived`**: `linked_issue_provider/ref/site` có nhưng không `task_id` → tìm task có `task_sources` khớp. Vì không có RPC tra ngược (1.1), cần **một trong hai**: (a) thêm RPC hẹp `FindTaskBySource(project_id, provider, ref, site)` vào `task-service` (đề nghị, CR riêng của task-service; ghi "Điều chỉnh hợp đồng"), hoặc (b) duyệt `ListTasks(project_id)` theo trang và gọi `GetTaskSource` từng task (**không dùng**: tốn N lời gọi). Cho tới khi có (a), trường hợp này trả `requirement: null` với `reason=no_reverse_lookup`.
3. **`inferred`**: trailer trong commit của `base..head` (`Refs: <ref>`, `#TG-<n>`, `Fixes:`) → `FindTaskByNumber`/khớp `task_sources`; hoặc tên nhánh chứa `<ref>` (ví dụ `ENG-123-...`). Nhãn "suy luận"; người dùng xác nhận được (2.6).
4. Không tìm được: hiển thị trạng thái `no_requirement_linked` (không bịa), kèm hành động "Liên kết với task…" (mở chọn task; ghi `requirement_trace_links` kiểu `worktree_task`, 2.6).

Mọi lần gọi `project-service`/`task-service` đi qua kiểm quyền của CR-CV-013 (chuỗi 0–9; quyền `quality_read`) và **bị cắt theo tenant**; nội dung task chỉ trả khi người gọi đọc được task đó.

### 2.3 Rút tiêu chí từ `Task.Description` (nhánh độc lập, suy luận)

Hàm thuần `domain/requirement_criteria_extractor.go`:

1. Tìm tiêu đề Markdown khớp (không phân biệt hoa thường/dấu) `acceptance criteria`, `tiêu chí (chấp nhận|hoàn thành)`, `definition of done`, `yêu cầu`; lấy các mục danh sách dưới tiêu đề đó đến tiêu đề cùng cấp kế tiếp.
2. Nếu không có tiêu đề: lấy mọi mục danh sách việc `- [ ]`/`- [x]` trong mô tả.
3. Nếu vẫn không có: **một** yêu cầu có `origin="title_only"` (văn bản = tiêu đề task) và cờ "chưa có tiêu chí có cấu trúc".
4. Mỗi mục: `key = "task:<task_id>#<hash 8 hex của văn bản chuẩn hoá>"` (ổn định khi đổi thứ tự, đổi khi sửa chữ — chấp nhận, ghi ở mục 6), `text` ≤ 300 ký tự, tối đa 20 mục; `origin ∈ structured|checklist_heuristic|title_only`. Mục đã tick `[x]` **không** được coi là bằng chứng.

Nguồn `ai_context`/`AIPlanJSON` **không** dùng để rút tiêu chí (định dạng tự do, có thể chứa dữ liệu nhạy cảm).

### 2.4 Mô hình `RequirementTrace` (proto `requirement_trace.proto`, mới)

```jsonc
RequirementTrace {
  "subject": { "source": "task|request", "taskId": "…", "requestId": null, "taskNumber": 42,
               "externalRef": {"provider":"jira","ref":"ENG-123"}, "worktreeRef": "…",
               "headCommit": "…", "baseCommit": "…", "indexCommit": "…", "indexStale": false },
  "linkConfidence": "explicit|derived|inferred|none",
  "requirements": [ Requirement ],              // ≤ 50
  "unlinkedChanges": [ { "file": "…", "symbols": ["…"], "reason": "no_match" } ],   // ≤ 100
  "summary": { "total": 6, "hasEvidence": 2, "partial": 2, "noEvidence": 1, "unknown": 1 },
  "warnings": [ "no_structured_criteria", "index_stale", "request_service_unavailable" ] }

Requirement { "key": "AC-1|task:…#a1b2c3d4", "text": "…", "origin": "structured|checklist_heuristic|title_only",
  "verifyHint": "test|metric|manual|review|null", "retired": false,
  "state": "has_evidence|partial|no_evidence|manual_pending|unknown",
  "evidence": [ Evidence ] }                    // ≤ 20 mỗi yêu cầu

Evidence { "kind": "change|test|check_run|manual_confirmation",
  "ref": "<SymbolRef.key|đường dẫn|runId|linkId>", "label": "…ngắn…",
  "confidence": "explicit|derived|inferred",
  "matchedBy": "satisfies|commit_trailer|path_hint|name_overlap|test_edge|check_profile|user_confirmed" }
```

Trạng thái **không bao giờ** là "đã đáp ứng/hoàn thành". Quy tắc `state`:

| `state` | Điều kiện |
|---|---|
| `has_evidence` | ≥ 1 bằng chứng `change` **và** ≥ 1 bằng chứng `test`/`check_run` đều `explicit|derived|user_confirmed` (khi `verifyHint ∈ {test,null}`); với `manual/review` thì cần `manual_confirmation` |
| `partial` | chỉ có change **hoặc** chỉ có test/kiểm tra, mức `explicit|derived` |
| `manual_pending` | `verifyHint ∈ {manual, review}` và chưa có xác nhận của người |
| `no_evidence` | chỉ có bằng chứng `inferred` (liệt kê như "gợi ý", không tính) hoặc không có gì |
| `unknown` | index cũ/thiếu overlay/kiểm tra bắt buộc chưa chạy (lý do trong `warnings`) — không suy diễn sang `no_evidence` |

Nhãn "suy luận": mọi `Evidence` hoặc `linkConfidence` mang `inferred` hiển thị nhãn **"Suy luận"** (i18n) và icon khác; UI không dùng màu "đạt" cho chúng (CR-CV-087/088).

### 2.5 Cách ghép bằng chứng

Chạy trong `usecase/get_requirement_trace.go`, đầu vào là `ChangeOverlay` (CR-CV-036: `changedFiles`, `changedSymbols` có `tested`, `readingOrder[].tests`), `QualityRun`/`QualityFinding` (CR-CV-082), commit trong `base..head` (qua collector; lệnh git chỉ đọc, giới hạn 500 commit), và danh sách yêu cầu:

1. **`satisfies` (explicit)**: chỉ có ở nhánh v6 (`Task.satisfies ↔ AC id`; Phase/Plan qua cây). Tệp/symbol đổi của worktree gắn vào Task tương ứng của worktree (`Worktree.task_id`) → mỗi AC mà task đó `satisfies` nhận bằng chứng `change` mức `derived` (**không** `explicit`, vì "task thoả AC" không chứng minh *dòng đó* thoả AC).
2. **Trailer commit (`commit_trailer`, derived/inferred)**: trailer `Refs:`/`Fixes:`/`Satisfies:` (đề xuất quy ước `Satisfies: AC-1` cho v6) trong thông điệp commit gắn các tệp của commit đó vào yêu cầu. Chỉ dùng phần trailer ở cuối thông điệp (không quét thân commit tự do).
3. **Cạnh test (`test_edge`, derived)**: test trong `readingOrder[].tests`/`changedSymbols[].tested=yes` của symbol đã gắn change → bằng chứng `test`. Mỗi bằng chứng ghi `ref` = `SymbolRef.key` của test.
4. **Kiểm tra đã chạy (`check_profile`, derived)**: `checks[].kind` v6 ∈ `test|lint|typecheck` ánh xạ sang profile chạy cùng category (CR-CV-081); có `QualityRun` `succeeded` cùng HEAD → bằng chứng `check_run`; **không** ánh xạ `command` tuỳ ý (không chạy lệnh, D5/O11).
5. **Trùng tên (`name_overlap`, inferred)**: tách từ văn bản tiêu chí và tên tệp/symbol/test (camelCase, snake_case, kebab-case, bỏ dấu, từ dừng, tối thiểu 2 từ khớp có nghĩa); điểm Jaccard ≥ 0,5 trên token → đề xuất (≤ 5 mỗi yêu cầu). Luôn `inferred`; **không** làm đổi `state` ngoài `no_evidence` + gợi ý.
6. **Xác nhận của người (`user_confirmed`)**: bảng `requirement_trace_links` (2.6) — xác nhận một gợi ý thành `explicit`/`derived`, hoặc loại bỏ (`rejected`) để không hiện lại.

Thay đổi không gắn yêu cầu nào đi vào `unlinkedChanges` (ví dụ file chạm nhưng không thuộc tiêu chí nào: có thể là phạm vi thừa — gợi ý review, không phải lỗi). Mọi truy vấn đồ thị (nếu cần ngoài overlay) theo giới hạn của README (`truncated`, `totalCount`).

### 2.6 Lưu xác nhận: `requirement_trace_links` (bảng mới, hai dialect)

| Cột | Ghi chú |
|-----|---------|
| `id`, `tenant_id` | uuid |
| `repo_id` | uuid (xác nhận là tri thức của repo như `finding_dismissals`; xem Q3) |
| `scope_key` | `worktree:<repo_binding_id>` hoặc `repo` |
| `requirement_key` | varchar(160): `AC-…` hoặc khoá `task:…#hash` |
| `link_kind` | `evidence_confirm\|evidence_reject\|worktree_task` |
| `evidence_kind`, `evidence_ref` | varchar; `evidence_ref` ≤ 255 (đường dẫn hoặc `SymbolRef.key` đã băm nếu dài) |
| `task_id` | uuid NULL, dùng cho `worktree_task` |
| `by_user`, `at` | uuid, timestamptz |
| `version` | bigint |

Duy nhất `(tenant_id, repo_id, scope_key, requirement_key, link_kind, evidence_kind, evidence_ref)`. Quyền ghi: `review_write` (CR-CV-013); đọc: `quality_read`. Audit: `codeintel.trace.confirm` (`target: requirement:<repo>:<key cắt 120>`). Không hết hạn ở MVP (Q3).

### 2.7 RPC và kênh

`GetRequirementTrace(GetRequirementTraceRequest{repo_binding_id, base_ref?, include_inferred=true, task_id? /*liên kết tay*/, turn_key?}) → {trace, evaluatedAt}` trong `orca.codeintel.v1.QualityGateService` (README v7 3.10); kênh `codeIntel.quality.trace`; quyền `quality_read` (thêm `read` theo CR-CV-085 2.9); cờ `quality_gate_enabled` (CR-CV-085 2.2) — nghĩa là nhóm chất lượng, theo Q4. RPC ghi `ConfirmRequirementEvidence`/`LinkWorktreeTask` thuộc cùng service (message do CR này sở hữu) — ghi vào "Điều chỉnh hợp đồng". Không gọi agent trực tiếp (dùng overlay/snapshot đã có và `git.*` qua collector); hạn mức CR-CV-013 2.6 áp dụng; kết quả đệm trong `graph_snapshots(view="requirementTrace", commit, params_hash)` (CR-CV-022) nhưng **xác nhận của người ghép lúc đọc**, không nằm trong cache.

### 2.8 Hiển thị (dữ liệu cho CR-CV-087)

CR này chỉ cung cấp dữ liệu; gợi ý bố cục: ngăn "Yêu cầu" trong tab Review, mỗi tiêu chí một hàng với nhãn trạng thái bằng chữ + hình dạng (không chỉ màu), danh sách bằng chứng (nút nhảy tới tệp/symbol qua `pendingDiffReveal` của CR-CV-053), mục "Chưa thấy bằng chứng" đứng đầu, "Thay đổi chưa gắn yêu cầu" cuối; chữ: "Chưa thấy bằng chứng", "Có dấu vết (chưa xác nhận)", "Suy luận" — **không** dùng "đã đáp ứng", "đã hoàn thành", "đạt yêu cầu".

### 2.9 Cấu trúc file (mới)

`code-intel-service/internal/`: `domain/requirement_trace.go`, `requirement_criteria_extractor.go`, `requirement_evidence_matcher.go`, `requirement_name_tokenizer.go`; `usecase/get_requirement_trace.go`, `confirm_requirement_evidence.go`; `adapter/grpcclient/task_requirement_provider.go`, `request_requirement_provider.go`; `adapter/{postgres,mysql}/requirement_trace_link_repository.go`; `adapter/grpc/requirement_trace_server.go`. Tên theo khái niệm, không `helpers/utils`.

## 3. Quyết định thiết kế

1. **Chạy độc lập với v6**: nhánh Task dùng dữ liệu có thật; nhánh Request là adapter bật sau. Không trì hoãn giá trị theo việc v6 triển khai.
2. **Trạng thái không bao giờ là "đã đáp ứng"**: chỉ "có/chưa có dấu vết", đúng tinh thần không overclaim.
3. **`inferred` không tính vào `state`** (chỉ gợi ý): ngăn trùng tên ngẫu nhiên tạo cảm giác phủ.
4. **`unknown` ≠ `no_evidence`**: index cũ/thiếu dữ liệu không được đổi thành "chưa có bằng chứng".
5. **Chỉ trailer cuối commit**: tránh khớp nhầm tham chiếu trong thân commit/đoạn trích.
6. **Xác nhận của người là dữ liệu hạng nhất** nhưng tách khỏi cache và có audit.
7. **Không dùng `ai_context`/`AIPlanJSON`** làm nguồn tiêu chí.

## 4. Tiêu chí chấp nhận

- [ ] Worktree có `Worktree.task_id` → trace `linkConfidence=explicit`, yêu cầu rút từ `Description`; worktree không có liên kết → `none` + hành động liên kết, không bịa task.
- [ ] Mô tả task có mục "Acceptance criteria" 3 dòng → đúng 3 yêu cầu `checklist_heuristic`; không có tiêu chí → một yêu cầu `title_only` + cảnh báo `no_structured_criteria`; mục `[x]` không tạo bằng chứng.
- [ ] Yêu cầu chỉ có bằng chứng `inferred` → `no_evidence` và nằm trong danh sách "gợi ý", không tăng `summary.hasEvidence`.
- [ ] Index cũ hoặc thiếu overlay → các yêu cầu bị ảnh hưởng là `unknown` kèm `warnings`, không `no_evidence`.
- [ ] Yêu cầu có change `derived` nhưng không test/check → `partial`; có cả hai → `has_evidence`; `verifyHint=manual` → `manual_pending` đến khi có `manual_confirmation`.
- [ ] `confirm`/`reject` bằng chứng đổi trace tức thì mà không làm vô hiệu cache; `reject` không hiện lại ở lần tính sau; xoá worktree không xoá xác nhận mức `repo`.
- [ ] Nhánh v6 tắt hoặc `request-service` lỗi → rơi về nhánh Task kèm `request_service_unavailable`; bật → AC có `id` ổn định (`AC-1`) và `Task.satisfies` được dùng, test dùng fixture giả cho `GetRequestCoverage` (không phụ thuộc v6 thật).
- [ ] Không trả nội dung task nếu người gọi không có quyền đọc task (test với hai người dùng); `ai_context` không xuất hiện trong bất kỳ đầu ra nào.
- [ ] `unlinkedChanges` liệt kê đúng tệp không khớp; mọi danh sách có `truncated/totalCount`; không gọi git quá 500 commit.
- [ ] Chạy trên hai dialect (bảng `requirement_trace_links`), mọi truy vấn lọc `tenant_id`.

## 5. Kiểm thử

- **Unit**: trích tiêu chí (bảng: Markdown tiếng Việt/Anh, danh sách lồng, tick, tiêu đề lạ, mô tả rỗng, 100 mục); tokenizer (camelCase, kebab, dấu); khớp trailer (chỉ phần cuối; `Refs:`, `Fixes:`, `#TG-12`, chuỗi giả); máy trạng thái `state` (bảng ca đủ tổ hợp, đặc biệt `unknown`).
- **Use case**: các adapter giả cho `project-service`, `task-service`, `request-service`; kiểm quyền; fallback.
- **Repository (hai dialect, integration)**: duy nhất, CAS, lọc tenant.
- **Hợp đồng**: fixture JSON `RequirementTrace` cho CR-CV-087.
- **Thử với dữ liệu thật trước khi chốt**: chọn 5 task đã hoàn tất của Orca có mô tả dài, chạy bộ rút tiêu chí và đếm tỉ lệ rút đúng bằng mắt; chỉ khi đạt ngưỡng do nhóm đặt mới coi nhánh Task là hữu ích.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chất lượng nguồn**: `Task.Description` thường không có tiêu chí; nhánh độc lập có thể hầu như chỉ cho `title_only`. Giá trị thật của CR này phụ thuộc v6 (AC có cấu trúc). Chưa đo tỉ lệ mô tả có checklist.
- Không có RPC tra ngược `(provider, ref) → task` (2.2); không có thì trường hợp `linked_issue_*` không `task_id` bị bỏ.
- Quy ước commit (`Refs:`, `#TG-N`) không được thi hành; nhiều commit không có trailer. Có thể đề xuất thêm vào công cụ sinh commit message (ngoài phạm vi).
- Khoá `task:…#hash` đổi khi sửa chữ tiêu chí → mất xác nhận cũ (chấp nhận, như `finding_key` của CR-CV-037).
- `name_overlap` dễ báo nhầm với tên chung (`service`, `handler`); ngưỡng 0,5 là phỏng đoán, chưa đo.
- Phụ thuộc CR đề xuất chưa có (036, 082, 085, v6 CR-REQ-027/029): hình dạng thật có thể khác.
- `Server.GetTask` chưa thấy kiểm grant ở lớp gRPC (1.1); phải kiểm lại trong use case trước khi dựa vào.
- Dữ liệu yêu cầu có thể nhạy cảm (nội dung Jira); trace chỉ xuất ≤ 300 ký tự mỗi tiêu chí và đi qua bộ che bí mật của CR-CV-013.

## 7. Câu hỏi mở

- **Q1.** Thêm RPC `FindTaskBySource` vào `task-service` (CR riêng) hay chấp nhận bỏ nhánh `derived` ở MVP?
- **Q2.** Có chuẩn hoá trailer `Satisfies: AC-n` cho v6 (và bộ sinh commit message hỗ trợ) không? Cần chủ sở hữu CR-REQ-012/013.
- **Q3.** Xác nhận bằng chứng theo `repo` (sống qua worktree) hay theo worktree? Đề xuất: theo worktree mặc định, `repo` khi người dùng chọn.
- **Q4.** `GetRequirementTrace` nằm dưới cờ `quality_gate_enabled` hay chỉ `code_intel_enabled`? Đề xuất: `quality_gate_enabled` (cùng nhóm).
- **Q5.** Có lấy tiêu chí từ Jira (issue-tracking-service `GetIssue`) khi task chỉ chép tiêu đề? Chưa đọc cấu trúc Jira issue; cần quyết định riêng.
- **Q6.** Có đưa `summary` của trace vào cổng chất lượng (CR-CV-085) như một kiểm tra `warn` không? Mặc định **không** (cổng chỉ dựa kiểm tra máy).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.10 `GetRequirementTrace`; mục 8)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (C3)
- `/opt/repos/orca/docs/crs/v6/README.md`, `/opt/repos/orca/docs/crs/v6/request-artifact-model/CR-REQ-027-artifact-schema-and-ontology.md` (2.4, 2.5, `GetRequestCoverage`), `/opt/repos/orca/docs/crs/v6/execution-contract/CR-REQ-029-execution-contract-and-readiness-gate.md`, `/opt/repos/orca/docs/crs/v6/plan-phase-task/README.md`
- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md` (2.3), `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-012-project-worktree-to-repo-binding.md`, `CR-CV-013-authorization-audit-and-quotas.md`
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go`, `domain/task_source.go`, `migrations/postgres/0012_task_sources.up.sql`, `0010_execution_links.up.sql`, `internal/usecase/find_task_by_number.go`, `internal/adapter/grpc/server.go` (`GetTask` `:149`)
- `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto` (`GetTaskSource`, `FindTaskByNumber`, `ResolvePermission`), `/opt/repos/orca/backend-go/proto/orca/project/v1/project.proto` (`Worktree.task_id` `:543`, `linked_issue_*` `:531`)
- `/opt/repos/orca/backend-go/services/git-gateway-service/internal/usecase/commit_message_prompt.go` (trailer `Refs:`)
- CR cùng nhóm (chỉ ID): CR-CV-082, 085, 087, 089, 093
- `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`
