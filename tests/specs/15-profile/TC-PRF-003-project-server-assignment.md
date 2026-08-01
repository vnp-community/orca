# TC-PRF-003 — Project-Dev Server Assignment

**BL Reference:** BL-PRF-03  
**Priority:** P0  
**Type:** Integration  
**Actor:** Admin, Lead

---

## TC-PRF-003-01: Tạo project với devServerId binding

**Priority:** P0

### Steps
1. Admin/Lead: `POST /api/projects` với `{ name, devServerId: 'srv-123', ... }`

### Expected Results
- Project created với `devServerId = 'srv-123'`
- Mọi operations trên project route qua srv-123

---

## TC-PRF-003-02: Auto-routing — Worktree ops đến đúng server

**Priority:** P0

### Steps
1. `worktree.create { projectId: 'proj-A' }` (bound to srv-1)
2. Verify relay dùng srv-1

### Expected Results
- `ProjectServerRouter.getRelayForProject('proj-A')` → relay của srv-1

---

## TC-PRF-003-03: User membership — Non-member bị từ chối

**Priority:** P0

### Steps
1. User không phải member của project-X
2. `worktree.create { projectId: 'project-X' }`

### Expected Results
- Error: `{ code: 'NOT_PROJECT_MEMBER' }`

---

## TC-PRF-003-04: Change devServerId — Migration

**Priority:** P1

### Steps
1. Project bound to srv-1
2. Admin change binding to srv-2
3. Next operation uses srv-2

### Expected Results
- `ProjectServerRouter` updates routing
- Operations route to srv-2

