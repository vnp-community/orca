# BE-CV-SOL-092-requirement-trace: Truy vết yêu cầu (Task hiện có, Request v6 bật sau) ↔ thay đổi ↔ test

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Priority P2; chạy **độc lập v6** ở mức Task. Nhánh Request (v6) chỉ có **cổng + fake** vì `request-service` chưa tồn tại trong repo.

**CR:** [CR-CV-092](../../../../../../docs/crs/v7/quality-gate/CR-CV-092-requirement-traceability.md)
**Service:** `code-intel-service` (mới) · client gRPC tới `project-service`, `task-service`, (tương lai) `request-service` · `codeintel_requirement_trace.proto` (mới)
**Hợp đồng:** [CONTRACT-codeintel-proto-and-data-map.md](../../CONTRACT-codeintel-proto-and-data-map.md) (PQ-01, PQ-03, PQ-04, §2.1 dòng 22, §3.2, §3.3, §4.2 T3/T15, §6.3), [CONTRACT-codeintel-ui-api.md](../../CONTRACT-codeintel-ui-api.md) (§3.2 `quality.trace|trace.confirm|trace.link`, §2.4, §4.7)
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md) (§Dependency graph), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (§Cross-service data consistency, §Multi-tenancy), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (§AuthZ, §Multi-tenancy isolation), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (§gRPC conventions), [`services/task-service.md`](../../../../tdd/services/task-service.md), [`services/project-service.md`](../../../../tdd/services/project-service.md)

---

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

### 0.1 Hợp đồng áp dụng

| PQ / mục | Áp dụng |
|---|---|
| PQ-04 | Request nhận `selector`; `repo_binding_id` chỉ ở bảng |
| PQ-01/PQ-24 | Cờ: `quality_gate_enabled` (Q4 của CR chốt theo "cùng nhóm") → `CODEINTEL_QUALITY_GATE_DISABLED` |
| PQ-03 | Id con không thuộc tenant → `CODEINTEL_NOT_FOUND`; không quyền → `CODEINTEL_NOT_AUTHORIZED` |
| §3.2 | `GetRequirementTrace(base_ref?, include_inferred, task_id?, turn_key?) → {trace, evaluated_at}` (`quality_read`); `ConfirmRequirementEvidence(requirement_key, evidence_kind, evidence_ref, link_kind (CONFIRM|REJECT), scope) → {trace}` và `LinkWorktreeTask(task_id; rỗng = gỡ) → {trace}` (`review_write`) |
| §3.3 | Dùng `ProjectService.ListWorktrees/GetProject/ListMembers` (**không** `GetWorktree` theo id), `TaskService.GetTask/ResolvePermission`; `FindTaskBySource` là **đề nghị**, chưa có |
| §4.2 T15 | Bảng `requirement_trace_links` (`0007_requirement_trace_links`), không hết hạn ở MVP |
| T3 | `view="requirementTrace"` có tên trong danh sách snapshot nhưng solution này **không** ghi snapshot (L3) |
| ui-api §2.4 | `quality.trace.confirm|link` ≤ 8 KiB; `quality.trace` 20 s; ngoài khoảng bị từ chối |
| H8 | Mỗi tiêu chí ≤ 300 ký tự và qua bộ che bí mật; **không** dùng `ai_context`/`AIPlanJSON` |

### 0.2 Lệch giữa CR và hợp đồng / giữa CR và code thật

| # | CR nói | Hợp đồng / code thật | Xử lý |
|---|---|---|---|
| L1 | `project-service.GetWorktree` → `task_id` | Hợp đồng §3.3 cấm `GetWorktree` theo id; **đã đọc**: `usecase/get_worktree.go` và `postgres/worktree_repository.go` chỉ nhận `worktreeID`, **không** lọc tenant | Dùng `ListWorktrees(project_id)` (có tenant qua context) rồi lọc `Worktree.id == binding.worktree_id`; binding không có `worktree_id` (đường dẫn thuần) → `linkConfidence=none` + hành động liên kết tay |
| L2 | "lệnh git chỉ đọc qua collector, ≤ 500 commit" để lấy trailer | `CONTRACT-codeintel-agent-rpc.md` §4–5 **không có** method liệt kê thông điệp commit, và hợp đồng §8.3(5) cấm chạm agent ngoài §4–5 | Trailer đi qua **port `CommitTrailerSource`**; adapter thật chỉ viết khi hợp đồng cho ngoại lệ (dùng `git.history` hiện có, Q1). Thiếu adapter → bỏ bằng chứng `commit_trailer`, `warnings += commit_trailers_unavailable` |
| L3 | Đệm trace trong `graph_snapshots(view="requirementTrace")` | Nội dung tiêu chí lấy từ task có **quyền theo từng người** (`ResolvePermission`); snapshot theo `(binding, commit, params_hash)` không theo người dùng ⇒ người thứ hai không có quyền task có thể đọc chữ của task | **Không đệm** trace chứa chữ tiêu chí ở v1; nguồn đắt (overlay, run) đã có cache riêng; tên view giữ nguyên để dành. Q2 |
| L4 | `Server.GetTask` "chưa thấy kiểm grant" | **Đã đọc**: `usecase/get_task.go` chỉ `tenant.RequireTenantID` + `repo.Get(tenant, id)`; không grant | Gọi `ResolvePermission(task_id, user_id, "read")` **trước** `GetTask`; lỗi `errNoGrant` (không có grant hoặc OPA từ chối, cả hai fail closed) = không đọc được |
| L5 | `CODEINTEL_REQUIREMENT_SOURCE_REQUEST_ENABLED` | Không có trong bảng env của hợp đồng §6.2 | Giữ nguyên tên (tiền tố đúng PQ-23), mặc định `false` |
| L6 | Phát hiện `inferred` cho `linked_issue_*` không `task_id` cần `FindTaskBySource` | §3.3: "đề nghị, CR riêng, chưa có" | Nhánh `derived` trả `requirement: null` với `reason=no_reverse_lookup` (như CR) |
| L7 | Kênh/tham số dùng `repo_binding_id` | PQ-04 | `selector` |

### 0.3 Phụ thuộc chéo khu vực

| Hướng | Solution | Dùng gì |
|---|---|---|
| FE đối ứng | `FE-CV-SOL-092-requirement-trace-view` (ngăn "Yêu cầu", nhãn "Chưa thấy bằng chứng"/"Suy luận", nút liên kết task/xác nhận) · `FE-CV-SOL-087-quality-scorecard-and-state` (lối vào) · `FE-CV-SOL-090-review-report-export` (không bắt buộc) | JSON `RequirementTrace` (ui-api §4.7), 3 kênh |
| AG | Không (§8.2). Chỉ liên quan gián tiếp nếu L2 chọn `git.history` (đã có ở `agent/src/relay/agent-rpc-dispatch-git.ts:49`, `git-handler.ts:515`) | — |
| Dịch vụ khác | `project-service` (`ListWorktrees`), `task-service` (`GetTask`, `ResolvePermission`, `FindTaskByNumber`, `GetTaskSource`, `ListTasks`), `request-service` (v6: `GetRequestCoverage`, `ResolveArtifactRef`; **chưa có**) | Ghi rõ dữ liệu thật/chưa có ở §1 |
| BE lân cận | BE-CV-SOL-036 (overlay: `changedSymbols[].tested`, `readingOrder[].tests`), 082 (`quality_runs`), SOL-085-evaluator (cờ/OPA), 013 (OPA, che bí mật, audit), 012 (binding), 040-quality-channels | |

## 1. Trạng thái hiện tại (re-verify) — dữ liệu thật và chưa có

Đã đọc: CR-092 đủ; `task-service/internal/domain/{task.go,task_source.go}`, `usecase/{get_task.go,resolve_permission.go}`, `adapter/grpc/server.go` (`GetTask`, `ResolvePermission`), `proto/orca/task/v1/task.proto`, `proto/orca/project/v1/project.proto` (`Worktree.task_id=16`, `linked_issue_*`), `project-service/internal/usecase/get_worktree.go`, `git-gateway` đã được CR dẫn (chưa mở lại), `agent/src/relay/agent-rpc-dispatch-git.ts`, `agent/src/shared/git-history-log-parser.ts`; docs v6 `docs/crs/v6/request-artifact-model/`.

| Dữ liệu | Trạng thái | Bằng chứng |
|---|---|---|
| `Task.{Title, Description, Type, AIContext, AIPlanJSON, WorktreeID, TaskNumber}` | **Có** | `task-service/internal/domain/task.go` (`Description` :91, `AIContext` :100, `AIPlanJSON` :101, `WorktreeID` :108, `TaskNumber` :129) |
| Tiêu chí chấp nhận có cấu trúc trong Task | **Chưa có** | không có trường nào (CR; không thấy trong `Task`) |
| `TaskSource{provider ∈ jira|linear|github|gitlab, ref, url, site}` + `GetTaskSource(task_id)` | **Có**, chỉ tra thuận | `domain/task_source.go`; `task.proto:21` |
| Tra ngược `(provider, ref, site)` → task (`FindTaskBySource`) | **Chưa có** | không RPC/use case; `create_task_from_source.go` chỉ ghi |
| `FindTaskByNumber(project_id, task_number)` | **Có** | `task.proto:410` |
| `Worktree.task_id`, `linked_issue_provider/ref/site` | **Có** | `project.proto:531–556` |
| Kiểm grant khi đọc task | `GetTask` **không** kiểm; `ResolvePermission` có (trả lỗi khi không có grant/OPA từ chối) | `get_task.go`; `resolve_permission.go` |
| `GetWorktree` lọc tenant | **Không** (chỉ theo id) | `get_worktree.go`, `worktree_repository.go:114` |
| Trailer `Refs:` trong commit message | Chỉ do AI sinh commit message của git-gateway (theo CR, chưa mở lại) | CR §1.1 |
| Liệt kê thông điệp commit từ agent | `git.history` (`%B`) có ở agent, **không** thuộc hợp đồng codeintel | `git-history-log-parser.ts:5` (`GIT_HISTORY_COMMIT_FORMAT`); ngữ nghĩa khoảng `base..head` của `baseRef` **chưa kiểm chứng** |
| `request-service`, `proto/orca/request`, `GetRequestCoverage`, `acceptance_criteria[]`, `Task.satisfies`, `task.task_specs` (v6) | **Chưa có** (chỉ CR/spec v6) | `ls backend-go/services` và `backend-go/proto/orca` không có `request`; docs v6 là đề xuất |
| `code-intel-service`, `ChangeOverlay`, `QualityRun` | **Chưa có** | CR đề xuất |

### Correction relative to CR-CV-092

Xem bảng 0.2 (L1–L7). Ngoài ra: CR gọi bảng bằng `repo_binding_id`/`scope_key`; hợp đồng T15 cố định `scope_key ∈ (worktree:<repo_binding_id> | repo)`, `repo_id` là khoá nghiệp vụ — theo T15.

## 2. Giải pháp chi tiết

### 2.1 Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_requirement_trace.proto   # RequirementTrace, Requirement, RequirementEvidence + Get/Confirm/Link req-resp
services/code-intel-service/internal/
  domain/requirement_trace.go                  # kiểu miền + hằng enum
  domain/requirement_criteria_extractor.go     # thuần
  domain/requirement_name_tokenizer.go         # thuần
  domain/requirement_evidence_matcher.go       # thuần: bằng chứng + máy trạng thái `state`
  usecase/requirement_trace_ports.go           # RequirementProvider, WorktreeTaskResolver, TaskReader, CommitTrailerSource, OverlayReader, QualityRunReader, TraceLinkRepository
  usecase/get_requirement_trace.go, confirm_requirement_evidence.go, link_worktree_task.go
  adapter/grpcclient/task_requirement_provider.go     # project-service + task-service
  adapter/grpcclient/request_requirement_provider.go  # (tuỳ chọn, sau) khung + fake; thật khi request-service có
  adapter/{postgres,mysql}/requirement_trace_link_repository.go
  adapter/grpc/requirement_trace_server.go            # phương thức của QualityGateServer
  migrations/{postgres,mysql}/0007_requirement_trace_links.{up,down}.sql
```

### 2.2 Tìm yêu cầu từ worktree (`WorktreeTaskResolver`)

Thứ tự (dừng ở kết quả đầu có giá trị), trả `linkConfidence`:

1. **`explicit`**: `requirement_trace_links` kiểu `worktree_task` của `(repo_id, scope=worktree:<binding>)` (người dùng liên kết tay) **hoặc** `ListWorktrees(project_id)` → `Worktree.id == binding.worktree_id` → `task_id`. Cả hai cho `task_id`; ưu tiên liên kết tay.
2. **`derived`**: `Worktree.linked_issue_*` không `task_id` → cần `FindTaskBySource` (L6); chưa có → `requirement: null`, `reason=no_reverse_lookup`.
3. **`inferred`**: trailer cuối thông điệp commit (`Refs:`, `Fixes:`, `Satisfies:`, `#TG-<n>`) qua `CommitTrailerSource` (L2) → `FindTaskByNumber(project_id, n)` hoặc khớp `GetTaskSource`; hoặc tên nhánh chứa ref. Chỉ khi `include_inferred=true`.
4. Không có: `linkConfidence=none` (không bịa task).

Với mọi `task_id` tìm được: `ResolvePermission(task_id, caller_user_id, "read")` trước `GetTask` (L4); lỗi → coi như không có yêu cầu (`reason=task_not_readable`), **không** lộ title. Tenant lấy từ ctx; `task_id` do client cung cấp (`GetRequirementTrace.task_id`, "liên kết tay") cũng phải qua `ResolvePermission` và phải cùng project với `selector`.

### 2.3 Rút tiêu chí từ `Task.Description` (nhánh độc lập, suy luận)

Theo CR §2.3 nguyên văn (tiêu đề Markdown khớp song ngữ → danh sách; nếu không có thì `- [x]/- [x]`; nếu vẫn không có → một yêu cầu `title_only`); `key = "task:<task_id>#<hash 8 hex của văn bản chuẩn hoá>"`; ≤ 20 mục, mỗi mục ≤ 300 ký tự đã qua `TextRedactor`; `[x]` không là bằng chứng; **không** đọc `AIContext`/`AIPlanJSON`. Hash: sha256 của chuỗi chuẩn hoá (NFKC, thường, bỏ dấu, gộp khoảng trắng) lấy 8 hex đầu (quyết định của solution; CR chỉ nói "hash 8 hex").

### 2.4 Ghép bằng chứng và `state`

Hàm thuần (domain) nhận `ChangeOverlay` (đã đọc qua port), `QualityRun`, trailer, `TraceLink` (xác nhận/loại bỏ). Quy tắc nguồn bằng chứng và bảng `state` **theo CR §2.4–2.5** (`has_evidence | partial | no_evidence | manual_pending | unknown`; `inferred` chỉ là gợi ý, không đổi `state` trừ `no_evidence`+gợi ý). Điểm bổ sung:

- Bằng chứng `check_run`: chỉ khi `checks[].kind ∈ test|lint|typecheck` (nhánh v6) hoặc `verifyHint=test` **và** có `quality_runs` `succeeded` cùng `head_commit` với profile cùng category (theo profile hiệu lực SOL-085); **không** ánh xạ `command` tuỳ ý.
- `name_overlap` Jaccard ≥ 0,5 trên token, tối đa 5 gợi ý/yêu cầu, luôn `inferred` — **ngưỡng chưa đo (giá trị khởi điểm chưa hiệu chỉnh)**.
- `unknown` khi `indexStale`, thiếu overlay hoặc check bắt buộc chưa chạy; **không** suy ra `no_evidence`.
- `reject` (`REJECT`) loại bằng chứng khỏi lần tính sau; `confirm` nâng gợi ý thành `explicit/derived` (`matchedBy="user_confirmed"`).
- Giới hạn: `requirements ≤ 50`, `evidence ≤ 20`/yêu cầu, `unlinkedChanges ≤ 100`; vượt thì cắt và thêm `warnings`.

### 2.5 `requirement_trace_links` (T15) và ghi

Migration `0007_requirement_trace_links` (hai dialect, RLS Postgres, `WHERE tenant_id=?` MySQL). Duy nhất `(tenant_id, repo_id, scope_key, requirement_key, link_kind, evidence_kind, evidence_ref)`; ghi bằng upsert (PG `ON CONFLICT … DO UPDATE SET by_user, at, version=version+1`; MySQL `ON DUPLICATE KEY UPDATE`). `evidence_ref` ≤ 255 (chuỗi dài → băm sha256 hex, ghi `evidence_ref="sha256:<hex>"`; nhãn gốc không lưu). `ConfirmRequirementEvidence`: `link_kind` `CONFIRM|REJECT` ánh xạ `evidence_confirm|evidence_reject`; **`CONFIRM` sau `REJECT` cùng khoá** xoá dòng `reject` (và ngược lại) trong cùng transaction để một khoá chỉ có một trạng thái. `LinkWorktreeTask(task_id)`: kiểm `ResolvePermission` + cùng project; rỗng = xoá dòng `worktree_task`. Audit `codeintel.trace.confirm` (`target: requirement:<repo>:<key cắt 120>`), gồm cả `link`.

### 2.6 RPC, cờ, hạn mức

Đường: cờ → OPA (`quality_read`/`review_write`) → `selector → binding` → (không gọi agent trong use case chính; `CommitTrailerSource` nếu có đi qua `AgentCallGate` của SOL-013) → ghép. Mỗi `GetRequirementTrace`: tối đa 1 `ListWorktrees`, 1 `ResolvePermission` + 1 `GetTask`, ≤ 1 `GetTaskSource`; kết quả không cache chữ tiêu chí (L3), nhưng **xác nhận của người luôn ghép lúc đọc** (không có cache để vô hiệu hoá).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Độc lập v6; nhánh Request chỉ khung + fake | `request-service` chưa tồn tại |
| D2 | `ResolvePermission` trước `GetTask` | `GetTask` không kiểm grant (đã đọc) |
| D3 | Không `GetWorktree` theo id | Không lọc tenant (đã đọc); PQ-04, §3.3 |
| D4 | Không cache chữ tiêu chí | Quyền theo từng người (L3) |
| D5 | `inferred` không tính vào `state` | Tránh phủ giả do trùng tên |
| D6 | `unknown` ≠ `no_evidence` | H7 |
| D7 | Trailer qua port; adapter chờ hợp đồng | L2 |
| D8 | Chữ hiển thị do FE; không "đã đáp ứng" | F10 |

## 4. Tiêu chí chấp nhận

- [x] Binding có `worktree_id` khớp worktree có `task_id` → `linkConfidence=explicit`; binding không có `worktree_id` và chưa liên kết tay → `none` + không lộ task nào.
- [x] Người không có grant đọc task → `reason=task_not_readable`, response **không** chứa title/mô tả/tiêu chí; người có grant thấy đủ (test hai người dùng với `ResolvePermission` giả).
- [x] `ai_context`/`AIPlanJSON` không xuất hiện ở bất kỳ đầu ra nào.
- [x] Mô tả có mục "Acceptance criteria" 3 dòng → đúng 3 yêu cầu `checklist_heuristic`; không có → một `title_only` + `warnings=[no_structured_criteria]`; `[x]` không tạo bằng chứng.
- [x] Yêu cầu chỉ có bằng chứng `inferred` → `no_evidence`, nằm trong gợi ý, không tăng `summary.hasEvidence`.
- [x] Index cũ/thiếu overlay → `unknown` + `warnings`, không `no_evidence`.
- [x] change `derived` không test/check → `partial`; có cả hai → `has_evidence`; `verifyHint=manual` → `manual_pending` đến khi có `manual_confirmation`.
- [x] `CONFIRM`/`REJECT`/`LinkWorktreeTask` đổi trace tức thì; `REJECT` không hiện lại; xoá worktree không xoá xác nhận mức `repo`; `CONFIRM` sau `REJECT` chỉ để lại một dòng.
- [x] Nhánh Request tắt hoặc lỗi → rơi về nhánh Task kèm `request_service_unavailable`; fake `GetRequestCoverage` cho AC có `id` ổn định.
- [x] Mọi danh sách có giới hạn; ≤ 8 KiB cho confirm/link; `task_id` khác project → `CODEINTEL_INVALID_PARAMS`.
- [x] Hai dialect; mọi truy vấn lọc `tenant_id`; tenant khác không đọc/ghi link; `selector` tenant khác → `CODEINTEL_NOT_AUTHORIZED`.

## 5. Kiểm thử

- **Unit:** trích tiêu chí (Markdown tiếng Việt/Anh, danh sách lồng, tick, tiêu đề lạ, rỗng, 100 mục), tokenizer (camelCase, kebab, dấu, từ dừng), trailer (`Refs:`, `Fixes:`, `#TG-12`; chỉ khối cuối), máy `state` (bảng đủ tổ hợp, đặc biệt `unknown`).
- **Use case:** fake `project-service`/`task-service`/`request-service`; hai người dùng; fallback; `ResolvePermission` lỗi.
- **Repository (hai dialect, integration):** unique, upsert, thay `reject`↔`confirm`, cách ly tenant.
- **Hợp đồng:** golden `RequirementTrace` cho FE.
- **Thử dữ liệu thật trước khi chốt (CR §5):** 5 task đã hoàn tất của Orca để đo tỉ lệ rút tiêu chí đúng; chưa làm.

## 6. Rủi ro và điểm chưa kiểm chứng

- `Task.Description` thường thiếu tiêu chí → phần lớn `title_only`; giá trị thật phụ thuộc v6 (AC có cấu trúc) chưa có.
- Không có `FindTaskBySource`: nhánh `derived` rỗng.
- Quy ước commit (`Refs:`, `#TG-N`) không được thi hành; trailer có thể thiếu; nguồn thông điệp commit đang chờ quyết định hợp đồng (L2); ngữ nghĩa `git.history` với `baseRef` chưa kiểm chứng.
- Khoá `task:…#hash` đổi khi sửa chữ → mất xác nhận cũ.
- `ResolvePermission` mã lỗi cụ thể khi không có grant (`errNoGrant`) chưa đọc hết; coi mọi lỗi là "không đọc được" (fail closed).
- Dữ liệu yêu cầu có thể nhạy cảm (nội dung Jira) — đã giới hạn 300 ký tự + che.
- Chưa chạy bất kỳ test nào.

## 7. Câu hỏi mở

- **Q1.** Cho phép ngoại lệ `git.history` (hay thêm `commitMessages` vào một method codeintel) để lấy trailer? Nếu không, bỏ `commit_trailer` ở MVP.
- **Q2.** Có cần cache trace theo `(binding, commit, user_id)`? Mặc định không (L3).
- **Q3.** `FindTaskBySource` vào `task-service` (CR riêng) hay bỏ `derived`.
- **Q4.** Chuẩn hoá trailer `Satisfies: AC-n` cho v6 (CR-REQ-012/013).
- **Q5.** Tiêu chí từ Jira (`issue-tracking-service.GetIssue`)? Chưa đọc cấu trúc.
- **Q6.** Đưa `summary` vào cổng như check `warn`? Mặc định **không** (cổng chỉ dựa kiểm tra máy).
- **Q7.** Xác nhận theo `repo` hay theo worktree mặc định (CR Q3)? Hiện `scope` do client chọn, mặc định `worktree`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-092-requirement-traceability.md`; v6: `/opt/repos/orca/docs/crs/v6/request-artifact-model/CR-REQ-027-artifact-schema-and-ontology.md`
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (§3.3, T15), `CONTRACT-codeintel-ui-api.md` (§4.7)
- `/opt/repos/orca/backend-go/services/task-service/internal/{domain/task.go,domain/task_source.go,usecase/get_task.go,usecase/resolve_permission.go,adapter/grpc/server.go}`, `/opt/repos/orca/backend-go/proto/orca/{task,project}/v1/*.proto`, `/opt/repos/orca/backend-go/services/project-service/internal/usecase/get_worktree.go`
- `/opt/repos/orca/agent/src/relay/agent-rpc-dispatch-git.ts`, `/opt/repos/orca/agent/src/shared/git-history-log-parser.ts`
