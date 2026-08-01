# BL-WF-01 — Workflow Template Management (Create / Inherit / Share)

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-WF-01 |
| **Tên** | Workflow Template Management |
| **Domain** | Workflow Orchestration |
| **Actor** | Admin (company templates), Lead (team templates), User (personal) |
| **Priority** | P1 |

---

## Mô tả

Quản lý vòng đời của Workflow Templates: tạo mới, kế thừa từ template cha, publish lên library, share, và version control.

---

## Template Scopes

| Scope | Tạo bởi | Thấy bởi | Override by |
|-------|---------|----------|-------------|
| `company` | Admin | Tất cả users | Lead (team templates) |
| `team` | Admin/Lead | Team members | User (personal workflows) |
| `personal` | Any user | Owner only (mặc định) | — |

---

## Luồng: Tạo Template mới (from scratch)

```
User → Workflows → Library → Create New
    │
    ├── Input:
    │   - name, description, tags
    │   - scope: 'personal' | 'team' | 'company'
    │   - visibility: 'private' | 'team' | 'company' | 'public'
    │
    ├── Workflow Editor:
    │   - Visual DAG editor HOẶC YAML editor
    │   - Add steps: drag-drop từ step palette
    │   - Configure per step: server, provider, prompt/command
    │
    ├── Validate YAML schema (Zod):
    │   - All step IDs unique
    │   - depends_on references valid step IDs (no cycles)
    │   - server refs format valid
    │   - provider refs format valid
    │
    ├── INSERT orca_workflow_templates (id, scope, owner_id, name,
    │     template_yaml, visibility, version='1.0', tags, ...)
    │
    └── Success: template xuất hiện trong Library
```

---

## Luồng: Kế thừa từ Template (Override/Extend)

```
User → Library → [template] → "Use as Base" / "Clone & Customize"
    │
    ├── Clone (copy): tạo personal copy — không linked với parent
    │
    └── Inherit (extend): tạo child template với reference đến parent
        ├── template_id: "team:backend-team:standard-feature-dev"
        │
        ├── Cấu trúc inheritance:
        │   overrides:          # thay đổi field của step cha
        │     steps.implement.provider.model: "gpt-4o"
        │     steps.implement.prompt: "{{base_prompt}} Also add OpenAPI docs."
        │   inject_steps:       # thêm steps mới
        │     - position: after
        │       after_step: implement
        │       step: { id: my-step, type: shell, command: "make fmt" }
        │   remove_steps:       # bỏ step không cần
        │     - "lint-check"
        │
        └── resolveTemplate(childId):
              parent = loadTemplate(child.templateId)
              resolved = deepMerge(parent, child.overrides)
              resolved.steps = applyInjectionsAndRemovals(resolved.steps, child)
              return resolved
```

---

## Luồng: Publish & Share

```
Owner → Library → [workflow] → Share
    │
    ├── Change visibility:
    │   private → team    (team members thấy)
    │   team → company    (all company users thấy — admin approval optional)
    │   → public          (tạo share link: orca.company.com/workflows/share/<token>)
    │
    ├── Nếu publish lên company scope:
    │   Admin review → approve/reject
    │   Sau khi approve: xuất hiện trong "Company Standards"
    │
    └── Rating & Usage tracking:
        - Mỗi lần user chạy workflow → increment usage_count
        - User có thể rate (1–5 stars) sau khi chạy xong
```

---

## Version Control

```typescript
// Khi update template có người đang dùng:
if (template.usageCount > 0 && hasBreakingChanges(oldYaml, newYaml)) {
  // Bump minor version 1.0 → 1.1
  // Existing executions vẫn chạy với version cũ
  // New executions dùng version mới
  // Show changelog diff trong UI
}
```

---

## Tiêu chí chấp nhận

- [ ] CRUD templates với scope: company/team/personal
- [ ] YAML schema validation (step IDs unique, no cycles, valid refs)
- [ ] Clone mode (copy, no parent link)
- [ ] Inherit mode (parent ref, overrides + inject + remove)
- [ ] `resolveTemplate()` merge parent + child đúng
- [ ] Visibility change: private → team → company → public
- [ ] Share link generation (public visibility)
- [ ] Company scope: admin approval flow
- [ ] Rating + usage_count tracking
- [ ] Version bump khi breaking change + có active usage
