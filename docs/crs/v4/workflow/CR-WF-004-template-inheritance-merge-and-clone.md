# CR-WF-004 — Template Inheritance Deep-Merge, Clone Mode, Conditional Version Bump

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WF-004 |
| **Tên** | Thay "nearest-ancestor-wins toàn bộ DAG" bằng field-level `overrides`/`inject_steps`/`remove_steps`; thêm Clone; version bump có điều kiện |
| **Loại** | Feature |
| **Priority** | P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | Không — độc lập kỹ thuật với CR-WF-002/003, có thể chạy song song |
| **Áp dụng thiết kế** | `specs/backend-go/bugs/logic-v1/BUG-WF-01-workflow-template-sharing-fields-missing.md`, `specs/backend-go/bugs/task-v1/BUG-TASKV1-007-workflow-sharing-not-implemented.md` |
| **Tác động** | `backend-go/services/workflow-service/internal/usecase/resolve_template.go`, `create_template.go`, `update_template.go`, `internal/domain/template.go` |

---

## 1. Vấn đề

`resolveEffectiveTemplate` (`internal/usecase/resolve_template.go:88-107`)
đi từ ancestor cụ thể nhất tới root, **trả nguyên toàn bộ DAG** của tổ tiên
đầu tiên có ≥1 step — không có khái niệm `overrides` (field-level), không
`inject_steps` (chèn step trước/sau 1 anchor), không `remove_steps`. Spec F36
(dòng 112-147) mô tả rõ 3-tier inheritance Company→Team→Personal PHẢI merge
ở mức field, không phải swap toàn bộ.

`create_template.go` chỉ có 1 con đường tạo template — theo chuỗi
`parent_template_id` (thừa kế) — không có endpoint tạo **bản sao độc lập**
(Clone) để người dùng "Use as base, không còn liên kết với gốc".

`update_template.go` bump `templates.version` **vô điều kiện mỗi lần ghi**
(`template.go:73-75`'s comment tự xác nhận) — kể cả sửa lỗi chính tả cũng
tăng version, gây version-churn không cần thiết cho template đang được nhiều
người dùng tham chiếu.

## 2. Giải pháp đề xuất

### 2.1 Schema — `overrides`/`inject_steps`/`remove_steps`

```go
// internal/domain/template.go
type WorkflowTemplate struct {
    // ...field hiện có...
    Overrides    map[string]any `json:"overrides,omitempty"`    // field-path → giá trị mới, áp lên DAG kế thừa
    InjectSteps  []StepInjection `json:"injectSteps,omitempty"`  // {anchorStepId, position: "before"|"after", step}
    RemoveSteps  []string        `json:"removeSteps,omitempty"`  // stepId bị loại khỏi DAG kế thừa
}
```

### 2.2 `resolveEffectiveTemplate` — deep-merge thay vì swap

```go
// resolve_template.go
func (r *TemplateResolver) resolveEffectiveTemplate(ctx context.Context, tpl WorkflowTemplate) (DAG, error) {
    baseDAG, err := r.resolveAncestorDAG(ctx, tpl.ParentTemplateID) // đệ quy lên tới root có DAG
    if err != nil { return DAG{}, err }
    merged := deepMerge(baseDAG, tpl.Overrides)      // áp override field-level lên DAG cha
    merged = applyInjections(merged, tpl.InjectSteps) // chèn step tại đúng anchor
    merged = applyRemovals(merged, tpl.RemoveSteps)   // loại step theo id
    return merged, nil
}
```

`deepMerge` chỉ override field có mặt trong `Overrides` (path dạng
`"steps.step1.prompt"`), giữ nguyên mọi field khác từ DAG cha — khác hoàn
toàn hành vi "lấy nguyên DAG của 1 ancestor" hiện tại.

### 2.3 Clone mode

```protobuf
rpc CloneTemplate(CloneTemplateRequest) returns (CloneTemplateResponse);
message CloneTemplateRequest {
  string source_template_id = 1;
  string new_name = 2;
  string scope = 3; // scope của bản clone, độc lập với scope gốc
}
```

`CloneTemplate` resolve DAG hiệu lực của source (dùng `resolveEffectiveTemplate`
ở trên) rồi ghi thành 1 template **mới, không có `parent_template_id`** —
đứt liên kết hoàn toàn với gốc (khác Inherit — sửa gốc sau này không ảnh
hưởng bản clone).

### 2.4 Version bump có điều kiện

```go
// update_template.go
func (u *UpdateTemplate) Execute(ctx context.Context, id string, changes TemplateChanges) error {
    isBreaking := changes.RemovesStep() || changes.ChangesStepType() || changes.ChangesDependency()
    hasActiveUsage, _ := u.repo.HasActiveExecutionsUsingTemplate(ctx, id)
    if isBreaking && hasActiveUsage {
        u.repo.BumpVersion(ctx, id) // chỉ bump khi thật sự breaking VÀ đang được dùng
    }
    return u.repo.Update(ctx, id, changes)
}
```

## 3. Rủi ro / Không thuộc phạm vi

- `deepMerge`'s cú pháp field-path (`"steps.step1.prompt"`) cần 1 spec rõ
  ràng cho các trường hợp biên (override 1 field lồng trong array step) —
  chốt cú pháp trong review trước khi code, tránh 2 lần đổi format.
- Định nghĩa chính xác "breaking change" cho version bump (§2.4) là quyết
  định sản phẩm — CR này đề xuất 1 tiêu chí cụ thể (remove step/đổi step
  type/đổi dependency) nhưng cần team xác nhận trước khi chốt, không tự
  quyết định 1 chiều.
- Không thuộc phạm vi: migrate dữ liệu template cũ đang dùng
  "nearest-ancestor-wins" sang format `overrides` mới — additive, template
  cũ không có `overrides`/`injectSteps`/`removeSteps` tiếp tục hoạt động
  đúng như hành vi hiện tại (empty override = giữ nguyên DAG cha, tương
  đương hành vi cũ).

## Acceptance Criteria

- [ ] `resolveEffectiveTemplate` merge đúng field-level — test: template con
      chỉ override `prompt` của 1 step, các step khác + field khác của step
      đó giữ nguyên từ cha.
- [ ] `inject_steps` chèn đúng vị trí trước/sau anchor step đã chỉ định.
- [ ] `remove_steps` loại đúng step khỏi DAG hiệu lực, không phá cạnh DAG còn
      lại (test: xoá step giữa chuỗi phụ thuộc, xác nhận dependency downstream
      được xử lý hợp lý — báo lỗi rõ ràng nếu xoá tạo dependency treo).
- [ ] `CloneTemplate` tạo bản độc lập, sửa gốc sau đó KHÔNG ảnh hưởng bản
      clone (test trực tiếp).
- [ ] Version chỉ bump khi thay đổi breaking VÀ có execution đang active dùng
      template đó — sửa lỗi chính tả (đổi field không breaking) không bump
      version (có test case cụ thể cho từng nhánh điều kiện).
- [ ] Template cũ (chưa có `overrides`) hoạt động không đổi sau migration.
