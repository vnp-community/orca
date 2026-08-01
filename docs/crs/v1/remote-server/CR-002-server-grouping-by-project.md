# CR-002 — Server Grouping by Project/Team

**CR-ID:** CR-002  
**Ngày:** 2026-07-22  
**Priority:** 🟠 High  
**Effort:** Small (1–2 ngày, phụ thuộc CR-001)  
**Depends on:** CR-001 (Fleet Inventory Config)  
**Status:** Implemented  

---

## 1. Vấn đề

Orca hiện hiển thị SSH targets như một flat list. Khi có 10–20 dev servers, developer không biết:
- Server nào thuộc project nào
- Server nào của team mình
- Môi trường là dev/staging/prod

**Hiện tại trong `SshTarget`:**
```typescript
type SshTarget = {
  id: string
  label: string       // Chỉ có text label tự do, không có structured metadata
  host: string
  port: number
  // ... KHÔNG có: project, team, environment
}
```

**UI hiện tại:** Flat list, không group, không filter theo project.

---

## 2. Phân tích codebase

### 2.1 SSH Target storage

`ssh-connection-store.ts` → `listTargets()`:
```typescript
listTargets(): SshTarget[] {
  return this.store.getSshTargets()
    .filter(target => !isRuntimeOwnedSshTarget(target))
  // Returns flat array — không có group/sort theo project/team
}
```

### 2.2 Renderer (UI)

Không tìm thấy grouping logic trong renderer. Danh sách SSH targets hiển thị flat trong sidebar.

### 2.3 SQLite schema

`persistence.ts` — table `sshTargets` không có cột `project`, `team`, `environment`.

---

## 3. Giải pháp đề xuất

### 3.1 Extend `SshTarget` schema

```typescript
// src/shared/ssh-types.ts
export type SshTarget = {
  // ... existing fields ...

  // NEW: Grouping metadata
  /** Project this server belongs to. e.g. "vnp-blc", "vnp-ai-ops" */
  project?: string
  /** Team owning this server. e.g. "backend", "frontend" */
  team?: string
  /** Deployment environment */
  environment?: 'development' | 'staging' | 'production'
  /** Tags for flexible grouping */
  tags?: string[]
}
```

### 3.2 Group view trong UI

```
SSH Hosts Sidebar (Proposed):
───────────────────────────────
  PROJECT: vnp-blc
    ● Dev Alpha [backend] ✅ Connected
    ● Dev Alpha2 [backend] ⊙ Connecting...

  PROJECT: vnp-ai-ops
    ● Dev Beta [ai-platform] ✅ Connected

  PROJECT: vnp-claw
    ● Dev Gamma [frontend] ✅ Connected

  UNASSIGNED
    ● My Local Machine
───────────────────────────────
```

### 3.3 Filter/Search

- Filter by `project`: "Chỉ hiện server của project đang làm việc"
- Filter by `team`: "Chỉ hiện server của team mình"
- Filter by `environment`: "Chỉ hiện dev servers"
- Search by label/host

---

## 4. Changes Required

### 4.1 Orca codebase

| File | Thay đổi |
|------|---------|
| `src/shared/ssh-types.ts` | Thêm `project?`, `team?`, `environment?`, `tags?` |
| `src/main/persistence.ts` | Thêm columns mới vào SQLite schema |
| `src/main/ssh/ssh-connection-store.ts` | Thêm `listTargetsByProject()`, `listTargetsByTeam()` |
| `src/renderer/src/` | Group/filter UI cho SSH targets |
| `src/main/ipc/` | Expose new filter methods qua IPC |

### 4.2 Migration

```sql
-- SQLite migration: thêm columns mới
ALTER TABLE sshTargets ADD COLUMN project TEXT;
ALTER TABLE sshTargets ADD COLUMN team TEXT;
ALTER TABLE sshTargets ADD COLUMN environment TEXT;
ALTER TABLE sshTargets ADD COLUMN tags TEXT; -- JSON array
```

---

## 5. Workaround hiện tại

Dùng naming convention trong `label` và `configHost`:

```
# ~/.ssh/config trên Orca Server
Host vnp-blc-dev-alpha
  HostName dev-alpha.vnpblc.internal
  User dev

Host vnp-ai-ops-dev-beta
  HostName dev-beta.vnpblc.internal
  User dev
```

Prefix `{project}-` trong host name giúp sort theo project trong flat list.

---

## 6. Acceptance Criteria

- [x] `SshTarget` có fields `project`, `team`, `environment`
- [x] UI sidebar group servers theo `project` (collapsible sections)
- [x] Filter panel: lọc theo project/team/environment
- [x] Group headers rõ ràng, collapsible
- [x] Import từ `orca-fleet.yaml` tự điền các fields này
- [x] Backward compatible: server cũ không có metadata vẫn hiển thị ở "Unassigned"

---

## 7. Implementation Notes

> **Implemented:** 2026-07-23

| File | Status |
|------|--------|
| `src/shared/ssh-types.ts` | ✅ [MODIFY] `project?`, `team?`, `environment?`, `tags?`, `fleetId?`, `repos?`, `SshTargetGroup`, `groupSshTargetsByProject()` |
| `src/main/persistence.ts` | ✅ [MODIFY] SQLite columns: `project`, `team`, `environment`, `tags` (JSON) |
| `src/main/ssh/ssh-connection-store.ts` | ✅ [MODIFY] `listTargetsByProject()`, `listTargetsByTeam()`, `filterTargets()`, `getProjectNames()` |
| `src/renderer/src/components/settings/ssh/SshTargetGroupedList.tsx` | ✅ [NEW] Filter bar + grouped collapsible list |
| `src/renderer/src/components/settings/ssh/SshTargetGroup.tsx` | ✅ [NEW] Collapsible group header component |
| `src/renderer/src/components/settings/ssh/SshTargetGroupRow.tsx` | ✅ [NEW] Individual target row in grouped view |
| `src/main/ipc/ssh.ts` | ✅ [MODIFY] `ssh.getProjectNames`, `ssh.filterTargets` IPC handlers |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23 | 6/6 AC done**

Server grouping by project implemented in fleet config and runtime service.

| File | Status |
|------|--------|
| `src/main/ssh/fleet-bootstrap-service.ts` | ✅ Project-based grouping |
| `src/shared/fleet-types.ts` | ✅ `FleetProject`, `FleetServer` types |
