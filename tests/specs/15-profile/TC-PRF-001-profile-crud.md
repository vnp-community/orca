# TC-PRF-001 — Profile CRUD (Company/Dept/User)

**BL Reference:** BL-PRF-01  
**Flow Reference:** docs/flows/logic/profile.md  
**Priority:** P0  
**Type:** Integration  
**Actor:** Admin, Lead, User

---

## TC-PRF-001-01: Admin tạo Company Profile

**Priority:** P0

### Steps
1. Login admin
2. `POST /admin/api/profiles/company` với:
   ```json
   {
     "name": "Acme Corp Profile",
     "agent": { "defaultModel": "claude-opus-4", "maxTokens": 8000 },
     "security": { "lockedModels": ["gpt-3.5"] },
     "envVars": { "COMPANY_ENV": "production" }
   }
   ```

### Expected Results
- HTTP 201 Created
- Profile stored với type='company'
- `security.lockedModels` set

---

## TC-PRF-001-02: Lead tạo Department Profile

**Priority:** P0

### Steps
1. Login Tech Lead
2. `POST /api/profiles/department` với parent=companyProfileId

### Expected Results
- Dept profile tạo với parentId=companyProfileId
- type='department'

---

## TC-PRF-001-03: User tạo User Profile

**Priority:** P0

### Steps
1. Login user
2. `POST /api/profiles/user` với overrides:
   ```json
   { "agent": { "defaultModel": "gpt-4" }, "envVars": { "USER_PREF": "dark" } }
   ```

### Expected Results
- User profile stored với type='user', userId attached

---

## TC-PRF-001-04: Update profile — Partial update

**Priority:** P1

### Steps
1. `PATCH /api/profiles/{id}` với chỉ một field thay đổi

### Expected Results
- Chỉ field đó thay đổi, các fields khác giữ nguyên

---

## TC-PRF-001-05: Delete profile với children — Cascade hoặc reject

**Priority:** P1

### Steps
1. Delete Company profile khi có Dept profiles

### Expected Results
- Error: `{ code: 'HAS_CHILDREN' }` hoặc cascade delete

