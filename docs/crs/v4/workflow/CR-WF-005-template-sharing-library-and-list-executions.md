# CR-WF-005 — Workflow Sharing &amp; Library Schema/RPC + `ListExecutions` Fix

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WF-005 |
| **Tên** | Schema/RPC mới cho visibility/share-link/rating/search — nền tảng backend cho Template Library UI; cộng fix `workflow.listExecutions` (RPC không tồn tại, frontend đang gọi vào chỗ trống) |
| **Loại** | Feature (thiết kế mới — kể cả TDD cũng chưa từng sketch phần sharing/library) |
| **Priority** | P1 (P0 riêng cho phần `ListExecutions` — đang phá `WorkflowMonitor.tsx`, component workflow DUY NHẤT thực sự mounted trong app) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | [CR-WF-004](./CR-WF-004-template-inheritance-merge-and-clone.md) (Clone mode là nền tảng cho "Import to My Workflows") |
| **Áp dụng thiết kế** | `specs/backend-go/bugs/logic-v1/BUG-WF-01-workflow-template-sharing-fields-missing.md`, `BUG-WF-03-workflow-sharing-not-implemented.md`, `specs/backend-go/bugs/task-v1/BUG-TASKV1-007-workflow-sharing-not-implemented.md`, `specs/backend-go/bugs/missing-v1/BUG-030-workflow-channels-not-implemented.md` |
| **Tác động** | `backend-go/services/workflow-service/internal/domain/template.go`, migration mới, `proto/orca/workflow/v1/workflow.proto`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go` |

---

## 1. Vấn đề

### 1.1 Sharing/Library — chưa có gì, kể cả trong TDD

`domain.WorkflowTemplate` (`template.go:63-77`) và proto message
(`workflow.proto:63-71`) chỉ có `id, tenant_id, name, dag_json, scope,
parent_template_id, version`. Không `visibility`, `owner_id`, `description`,
`tags`, `share_token`, `rating`, `usage_count`. `Scope` vẫn là enum đóng 3
giá trị (`company|team|personal`) — không có `public`.

**Quan trọng:** ngay cả `specs/backend-go/tdd/services/workflow-service.md`
§5 (thiết kế dự định) **cũng không có** các cột này — chỉ thêm
`description`/`owner_id` ngoài field đã build. Nghĩa là toàn bộ BL-WF-03
(sharing/library theo spec F36 gốc) **chưa từng được đưa vào TDD backend-go**
— đây là thiết kế mới thật sự, không phải "bắt kịp TDD".

`list_templates.go` chỉ filter theo `scope` — không search theo text/tag,
không sort trending/recent.

### 1.2 `workflow.listExecutions` — RPC không tồn tại, frontend đang gọi vào chỗ trống

```tsx
// frontend/src/renderer/src/components/workflow/WorkflowMonitor.tsx:33
const result = await callRuntimeRpc<WorkflowExecution[]>(target, 'workflow.listExecutions', { projectId })
```

Xác nhận trực tiếp: `workflow.proto`'s `WorkflowService` không có RPC
`ListExecutions` (chỉ có `GetExecution` số ít); `channels_workflow.go` đăng
ký đúng 11 channel (`execute, cancel, template.create, template.update,
getExecution, pause, resume, template.list, template.resolve,
hasActiveExecutions, executeAdHocStep`) — **`listExecutions` không nằm
trong số đó**. `WorkflowMonitor.tsx` là component workflow duy nhất thực sự
mounted trong `WorkspaceLayout.tsx` (`activeTab === 'workflows'`) — nghĩa là
tab Workflow của bất kỳ ai dùng backend-go **luôn fail load** ngay khi mở.
Đáng chú ý: backend Node legacy **có** `listExecutions`
(`BUG-BE-HLD-009` dòng 17) — đây là 1 regression thật của backend-go so với
legacy, không phải tính năng chưa từng tồn tại.

## 2. Giải pháp đề xuất

### 2.1 `ListExecutions` — fix nhanh, tách PR riêng nếu cần gấp

```protobuf
rpc ListExecutions(ListExecutionsRequest) returns (ListExecutionsResponse);
message ListExecutionsRequest { string project_id = 1; int32 limit = 2; string cursor = 3; }
```

Wire vào `channels_workflow.go` như 11 channel còn lại. Vì `WorkflowMonitor`
đang crash ngay khi mở, khuyến nghị **tách phần này thành 1 PR/patch riêng,
ship trước phần sharing/library** nếu team cần fix gấp trải nghiệm hiện tại.

### 2.2 Schema sharing/library

```sql
ALTER TABLE workflow.templates
  ADD COLUMN owner_id      TEXT,
  ADD COLUMN description   TEXT,
  ADD COLUMN tags          TEXT[] DEFAULT '{}',
  ADD COLUMN visibility    TEXT DEFAULT 'private', -- private|team|company|public
  ADD COLUMN share_token   TEXT,
  ADD COLUMN rating_sum    INT DEFAULT 0,
  ADD COLUMN rating_count  INT DEFAULT 0,
  ADD COLUMN usage_count   INT DEFAULT 0;
```

### 2.3 RPC mới

```protobuf
rpc UpdateVisibility(UpdateVisibilityRequest) returns (google.protobuf.Empty);
rpc GenerateShareLink(GenerateShareLinkRequest) returns (GenerateShareLinkResponse);
rpc GetTemplateByShareToken(GetTemplateByShareTokenRequest) returns (GetTemplateByShareTokenResponse); // view/import, không cần login
rpc RateTemplate(RateTemplateRequest) returns (google.protobuf.Empty);
rpc SearchTemplates(SearchTemplatesRequest) returns (SearchTemplatesResponse); // text + tag filter + sort trending/recent
```

`usage_count` tăng ở `Execute` usecase mỗi lần 1 execution dùng template đó
(không phải ở `ListTemplates`/view — chỉ tính lần dùng thật).

### 2.4 "Import to My Workflows" — dùng lại `CloneTemplate` (CR-WF-004)

`GetTemplateByShareToken` trả metadata + cho phép gọi `CloneTemplate`
(CR-WF-004) với `source_template_id` = template được share — không tự viết
1 cơ chế import riêng, tái sử dụng Clone.

## 3. Rủi ro / Không thuộc phạm vi

- **Đây là thiết kế mới, cần review kiến trúc riêng** trước khi implement
  (khác các CR áp dụng SOL doc có sẵn) — đặc biệt cơ chế `share_token` phải
  đi qua security review (giống lưu ý ở [`CR-TG-003`](../task-graph/CR-TG-003-task-access-control-team-scope-and-sharing.md)'s
  share-link cho Task) trước khi coi là an toàn cho production.
- `visibility=company` — cần xác nhận có yêu cầu admin-approval hay không
  (spec ngụ ý "company scope admin approval" nhưng không có chi tiết) — chốt
  trong review, không tự giả định.
- Không thuộc phạm vi: UI Library/browse — đó là CR-WF-006, CR này chỉ cung
  cấp RPC.
- `CR-TRACE-017`'s `workflowShareFlow` tracer (đã document sẵn ở kiến trúc
  legacy) cần 1 follow-up nhỏ re-map sang file backend-go thật sau khi CR
  này merge — ghi chú, không tự làm trong CR này.

## Acceptance Criteria

- [ ] `ListExecutions` hoạt động, `WorkflowMonitor.tsx` load được danh sách
      execution không còn lỗi (verify bằng cách chạy app thật, không chỉ
      unit test RPC).
- [ ] `UpdateVisibility`/`GenerateShareLink`/`GetTemplateByShareToken` hoạt
      động; `GetTemplateByShareToken` không yêu cầu auth nhưng chỉ trả field
      an toàn (không leak field nội bộ như `owner_id` thật của tenant khác).
- [ ] `SearchTemplates` search đúng theo text (name/description) + tag, sort
      đúng theo `usage_count` (trending) hoặc `created_at` (recent).
- [ ] `usage_count` tăng đúng 1 lần mỗi execution thật (không tăng khi chỉ
      xem/preview).
- [ ] `RateTemplate` cập nhật đúng `rating_sum`/`rating_count`, giá trị trung
      bình tính đúng ở response.
- [ ] "Import to My Workflows" từ share-link tạo đúng 1 bản Clone độc lập
      (dùng chung logic CR-WF-004, có test xác nhận).
