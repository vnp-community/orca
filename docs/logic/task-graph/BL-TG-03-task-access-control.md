# BL-TG-03 — Task Access Control & Sharing

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-TG-03 |
| **Tên** | Task Access Control & Sharing |
| **Domain** | Task Graph |
| **Actor** | Owner, Lead, Admin |
| **Priority** | P0 |

---

## Permission Levels (ordered)

```
view < comment < edit < execute < manage
```

Each level includes all lower levels.

| Permission | Actions allowed |
|-----------|----------------|
| `view` | Read task, subtasks, comments, activity |
| `comment` | + Add/edit own comments |
| `edit` | + Edit title/desc/status/labels/estimate/assignee |
| `execute` | + Run Agent, Create Worktree linked to task |
| `manage` | + Grant/revoke others, Delete task, Share tree |

---

## Grant Resolution Algorithm

```typescript
function hasTaskAccess(
  userId: string,
  task: OrcaTask,
  required: Permission
): boolean {
  // 1. Owner always has full manage
  if (task.ownerId === userId) return true

  // 2. Admin has full access to all tasks in their org
  const user = getUser(userId)
  if (user.role === 'admin') return true

  // 3. Check direct grants on this task
  const directGrants = task.grants.filter(g =>
    (g.scope === 'user' && g.userId === userId) ||
    (g.scope === 'team' && user.departmentId === g.teamId) ||
    (g.scope === 'company')
  )

  // 4. Check inherited grants (from ancestor tasks with apply_tree=true)
  const ancestorGrants = getAncestorGrants(task.id, userId)  // walk up parent chain

  const allGrants = [...directGrants, ...ancestorGrants]
  const maxPermission = maxOf(allGrants.map(g => g.permission))

  // 5. Check expiry
  const activeGrants = allGrants.filter(g => !g.expiresAt || g.expiresAt > now())

  return permissionLevel(maxPermission) >= permissionLevel(required)
}
```

---

## Luồng: Grant Access to Task Tree

```
Owner → Task (Epic) → Share → Grant Access
    │
    ├── Select scope:
    │   ○ Company (all users in org)
    │   ○ Team: [Backend Team ▼]
    │   ● User: [@nguyen.van.b search...]
    │
    ├── Select permission: [Execute ▼]
    │
    ├── Apply to:
    │   ○ This task only
    │   ● This task + all subtasks (apply_tree = true)
    │
    ├── Optional: expires at [2026-09-01]
    │
    ├── On confirm:
    │   INSERT orca_task_grants (task_id, scope, user_id?, team_id?,
    │     permission, apply_tree, granted_by, granted_at, expires_at)
    │
    └── Grantee receives WebSocket push:
        { type: 'task.grant_received', taskId, permission, grantedBy }
```

---

## Luồng: Share Task Tree (public link)

```
Owner → Task (Epic) → Share → Public Link
    │
    ├── Generate share token: randomBytes(16).hex()
    ├── INSERT orca_task_grants (scope='public_link', token=..., permission='view')
    ├── share_url = https://orca.company.com/tasks/share/<token>
    └── Anyone with link → view task tree (read-only, no login required)
```

---

## Grant Inheritance (apply_tree)

```
Epic [grant: company, view, apply_tree=true]
    ├── Story A [inherits company view from Epic]
    │       ├── Task A1 [inherits]
    │       └── Task A2 [inherits]
    └── Story B [owner adds: user@x.com, execute] ← additional grant
            └── Task B1 [inherits both: company view + user@x.com execute]
```

---

## Tiêu chí chấp nhận

- [ ] `hasTaskAccess()` resolve: owner > admin > direct > team > company
- [ ] apply_tree: grant propagates to all descendants (lazy — resolved at query time)
- [ ] Grant expiry: ignore expired grants
- [ ] Grant CRUD: add/revoke/list grants (manage permission required)
- [ ] Public link: share token → read-only view without login
- [ ] Grant notification: WebSocket push to grantee
- [ ] Revoke: delete grant → immediate effect (no cache needed)
