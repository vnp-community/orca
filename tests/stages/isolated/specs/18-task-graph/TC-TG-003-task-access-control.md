# TC-TG-003 — Task Access Control & Sharing (5-Level Grants)

**BL Reference:** BL-TG-03  
**Flow Reference:** docs/flows/logic/task-graph.md  
**Priority:** P0  
**Type:** Integration + Security  
**Actor:** Owner, Lead, Admin

---

## Grant Level Reference

| Level | Permissions |
|-------|------------|
| view | Read task, comments |
| comment | view + add comments |
| edit | comment + edit fields, add subtasks |
| execute | edit + run agent on task |
| manage | execute + grant/revoke for others |

---

## TC-TG-003-01: Grant user 'view' access

**Priority:** P0

### Steps
1. Owner: `task.grantAccess { taskId, grantee: userB, level: 'view' }`
2. User B: try to view task, edit task

### Expected Results
- View: allowed
- Edit: `{ code: 'FORBIDDEN' }` (level < edit)

### Assertions
```
await rpc.call('task.grantAccess', { taskId, grantee: userB.id, level: 'view' })

loginAs(userB)
task = await rpc.call('task.get', { taskId })
assert task !== null

result = await rpc.call('task.update', { taskId, title: 'New Title' }).catch(e => e)
assert result.code === 'FORBIDDEN'
```

---

## TC-TG-003-02: 5 grant levels — Permission matrix

**Priority:** P0

| Operation | view | comment | edit | execute | manage |
|-----------|:----:|:-------:|:----:|:-------:|:------:|
| task.get | ✓ | ✓ | ✓ | ✓ | ✓ |
| task.comment | ✗ | ✓ | ✓ | ✓ | ✓ |
| task.update | ✗ | ✗ | ✓ | ✓ | ✓ |
| task.runAgent | ✗ | ✗ | ✗ | ✓ | ✓ |
| task.grantAccess | ✗ | ✗ | ✗ | ✗ | ✓ |

### Steps
Verify each combination above with appropriate loginAs() and assertions

---

## TC-TG-003-03: apply_tree — Grant propagates to subtasks

**Priority:** P0

### Steps
1. Epic → Story × 2 → Subtask × 3
2. Grant User B `execute` on Epic với `applyTree: true`
3. Verify User B has `execute` on ALL descendant tasks

### Assertions
```
await rpc.call('task.grantAccess', {
  taskId: epic.id,
  grantee: userB.id,
  level: 'execute',
  applyTree: true
})

loginAs(userB)
story1 = await rpc.call('task.get', { taskId: story1.id })
assert story1.myGrant === 'execute'

subtask1 = await rpc.call('task.get', { taskId: subtask1.id })
assert subtask1.myGrant === 'execute'
```

---

## TC-TG-003-04: Grant scope — Company/Team/User

**Priority:** P1

### Steps
1. Grant company scope: `{ scope: 'company', level: 'view' }` → all company users can view
2. Grant team scope: `{ scope: 'team', teamId: 'engineering', level: 'edit' }` → team members can edit

### Assertions
```
await rpc.call('task.grantAccess', { taskId, scope: 'company', level: 'view' })

// Any company user can view
loginAs(anyCompanyUser)
task = await rpc.call('task.get', { taskId })
assert task !== null

// Team scope
await rpc.call('task.grantAccess', { taskId, scope: 'team', teamId: 'engineering', level: 'edit' })
loginAs(engineeringTeamMember)
result = await rpc.call('task.update', { taskId, title: 'Updated' })
assert result.title === 'Updated'

loginAs(nonEngineeringUser)
result = await rpc.call('task.update', { taskId, title: 'Updated' }).catch(e => e)
assert result.code === 'FORBIDDEN'
```

---

## TC-TG-003-05: Revoke access

**Priority:** P1

### Steps
1. Grant User B 'edit'
2. Revoke: `task.revokeAccess { taskId, grantee: userB.id }`
3. User B tries to access

### Expected Results
- After revoke: User B gets `{ code: 'FORBIDDEN' }`

### Assertions
```
await rpc.call('task.grantAccess', { taskId, grantee: userB.id, level: 'edit' })
loginAs(userB)
task = await rpc.call('task.get', { taskId })
assert task !== null  // before revoke: OK

loginAs(owner)
await rpc.call('task.revokeAccess', { taskId, grantee: userB.id })

loginAs(userB)
result = await rpc.call('task.get', { taskId }).catch(e => e)
assert result.code === 'FORBIDDEN'
```

---

## TC-TG-003-06: Time-limited grant — expiresAt

**Priority:** P1

### Steps
1. Grant User B 'view' với `expiresAt: now + 1h`
2. Access at T+0 → success
3. Advance time to T+2h
4. Access again → FORBIDDEN

### Expected Results
- T+0: User B can view task
- T+2h: `{ code: 'FORBIDDEN', reason: 'GRANT_EXPIRED' }`
- DB: grant record still exists (auditable)

### Assertions
```
const expiresAt = Date.now() + 60 * 60 * 1000  // 1h
await rpc.call('task.grantAccess', { taskId, grantee: userB.id, level: 'view', expiresAt })

loginAs(userB)
task = await rpc.call('task.get', { taskId })
assert task !== null  // before expiry: OK

advanceTime(2 * 60 * 60 * 1000)  // 2h later

result = await rpc.call('task.get', { taskId }).catch(e => e)
assert result.code === 'FORBIDDEN'

grant = db.taskGrants.find({ task_id: taskId, user_id: userB.id })
assert grant !== undefined   // still in DB (not deleted)
assert grant.expires_at < Date.now()
```

---

## TC-TG-003-07: Non-owner cannot grant (requires 'manage')

**Priority:** P1

### Steps
1. User B has 'edit' grant (not 'manage')
2. User B tries: `task.grantAccess { grantee: userC.id, level: 'view' }`

### Expected Results
- Error: `{ code: 'FORBIDDEN', required: 'manage', current: 'edit' }`
- No grant created for userC

### Assertions
```
loginAs(userB)  // has 'edit' grant, needs 'manage' to grant others
result = await rpc.call('task.grantAccess', {
  taskId, grantee: userC.id, level: 'view'
}).catch(e => e)
assert result.code === 'FORBIDDEN'
assert result.required === 'manage'
assert db.taskGrants.count({ task_id: taskId, user_id: userC.id }) === 0
```

---

*TC-TG-003 — Orca v5.0 — Updated 2026-08-01*
