# BL-PRF-01 — Tạo và Cập nhật Profile (Company / Department / User)

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-PRF-01 |
| **Tên** | Tạo và Cập nhật Profile |
| **Domain** | Profile Management |
| **Actor** | Admin (Company/Dept), Lead (Dept), User (Personal) |
| **Priority** | P0 |

---

## Mô tả

Quản lý 3 tầng profile trong hệ thống: Company (root), Department (intermediate), User (leaf). Mỗi tầng có thể được tạo, đọc, và cập nhật bởi người có quyền tương ứng.

---

## Tiền điều kiện

- Admin đã đăng nhập với role = 'admin'
- Company profile đã được tạo (prerequisite cho Department)
- Department đã tồn tại (prerequisite cho User assignment)

---

## Luồng chính

### 1. Tạo/Cập nhật Company Profile

```
Admin → Settings → Company → Edit Profile
    │
    ├── Validate: role === 'admin'
    ├── Load existing company profile (or empty {})
    ├── Render form: AI Policy, Security, Integrations
    │
    ├── On save:
    │   ├── Validate schema (Zod): OrcaProfile.company fields
    │   ├── Validate: approved_models ⊆ SUPPORTED_MODELS
    │   ├── Validate: session_timeout_hours ∈ [1, 168]
    │   ├── upsert orca_company SET profile_json = ?
    │   ├── ProfileCache.invalidate('company')  ← all users affected
    │   └── audit_log('company.profile.updated', adminId)
    │
    └── Success: "Company profile saved. Changes apply to all users."
```

### 2. Tạo Department

```
Admin → Settings → Departments → New Department
    │
    ├── Input: name, description, team_lead_id (optional)
    ├── Validate: name unique within company
    ├── INSERT orca_departments (id, company_id, name, profile_json='{}')
    └── audit_log('department.created', adminId, deptId)
```

### 3. Cập nhật Department Profile

```
Admin/Lead → Settings → Departments → [dept] → Edit Profile
    │
    ├── Validate: role === 'admin' OR (role === 'lead' AND user.dept_id === dept.id)
    ├── Load dept profile
    ├── Render form: AI model, fleet access, env vars (KHÔNG có Security section)
    │
    ├── On save:
    │   ├── Validate: cannot set security.* fields (dept cannot override company security)
    │   ├── UPDATE orca_departments SET profile_json = ? WHERE id = ?
    │   ├── ProfileCache.invalidate('department', deptId)
    │   └── audit_log('department.profile.updated', userId, deptId)
    │
    └── Affected users: all members of this department
```

### 4. Cập nhật User Personal Profile

```
User → Settings → My Profile → Edit
    │
    ├── Load resolved effective profile (for display)
    ├── Load user's own profile_json (for editing)
    │
    ├── Render form với 2 columns:
    │   [Inherited value] → [Override (editable)]
    │   e.g. "Theme: dark (Dept default)" → User can set "light"
    │
    ├── On save:
    │   ├── Validate: cannot set security.* or integrations.githubOrg
    │   ├── UPDATE orca_users SET profile_json = ? WHERE id = ?
    │   ├── ProfileCache.invalidate('user', userId)
    │   └── (no audit log for personal prefs — privacy)
    │
    └── Changes apply on next RPC call (cache TTL = 60s)
```

---

## Validation Rules

| Field | Company | Dept | User |
|-------|---------|------|------|
| `agent.approvedModels` | ✅ set list | ❌ read-only | ❌ read-only |
| `agent.preferredModel` | ✅ set default | ✅ override | ✅ override |
| `agent.trustPreset` | ✅ set default | ✅ override | ✅ override |
| `security.*` | ✅ all fields | ❌ forbidden | ❌ forbidden |
| `editor.*` | ✅ set default | ✅ override | ✅ override |
| `shell.envVars` | ✅ global vars | ✅ team vars | ✅ personal vars (merged) |
| `fleet.allowedServerTags` | ✅ allowed set | ✅ subset only | ❌ read-only |

---

## Error Cases

| Lỗi | HTTP | Thông báo |
|-----|------|-----------|
| Not admin (company edit) | 403 | "Company profile requires admin role" |
| Not lead (dept edit) | 403 | "Department profile requires lead or admin role" |
| Invalid model name | 400 | "Model 'xyz' not in supported models list" |
| Invalid session timeout | 400 | "Session timeout must be between 1 and 168 hours" |
| Dept setting security field | 400 | "Security settings can only be set at company level" |
