# TC-WF-003 — Workflow Sharing & Library Discovery

**BL Reference:** BL-WF-03  
**Flow Reference:** docs/flows/logic/workflow-orchestration.md  
**Priority:** P1  
**Type:** Integration  
**Actor:** Owner, Lead, Admin, Any User

---

## TC-WF-003-01: Change visibility to 'team'

**Priority:** P1

### Preconditions
- Template exists với `visibility: 'private'`, owner = userA
- userB is team member, userC is non-team member

### Steps
1. `PATCH /api/workflows/templates/:id { visibility: 'team' }`

### Expected Results
- DB: `orca_workflow_templates.visibility = 'team'`
- userA (owner): can see ✓
- userB (team member): can see ✓
- userC (other team): cannot see ✗

### Assertions
```
await api.patch('/api/workflows/templates/' + templateId, { visibility: 'team' })

loginAs(userB_sameTeam)
list = await api.get('/api/workflows/library')
assert list.some(t => t.id === templateId)

loginAs(userC_otherTeam)
list = await api.get('/api/workflows/library')
assert !list.some(t => t.id === templateId)
```

---

## TC-WF-003-02: Change visibility to 'company'

**Priority:** P1

### Steps
1. `PATCH /api/workflows/templates/:id { visibility: 'company' }`

### Expected Results
- All company users can discover template in library
- Non-company users cannot

---

## TC-WF-003-03: Change visibility to 'public' → generates share link

**Priority:** P1

### Steps
1. `PATCH /api/workflows/templates/:id { visibility: 'public' }`

### Expected Results
- DB: `workflow_share_links` record created: `{ token, templateId, expiresAt }`
- Share link URL returned: `/api/workflows/shared/<token>`
- Anyone with link can access (no auth required)

### Assertions
```
result = await api.patch('/api/workflows/templates/' + templateId, { visibility: 'public' })
assert result.shareLink !== undefined
assert result.shareLink.includes('/api/workflows/shared/')

// Access without auth
shareToken = extractToken(result.shareLink)
response = await fetch('/api/workflows/shared/' + shareToken)  // no auth header
assert response.status === 200
template = await response.json()
assert template.id === templateId
```

---

## TC-WF-003-04: Import from share link — Fork to personal

**Priority:** P1

### Steps
1. User gets share link token
2. `GET /api/workflows/shared/:token` → view template
3. `POST /api/workflows/shared/:token/fork` → creates personal copy

### Expected Results
- New template created: `{ scope: 'user', ownerId: currentUser, visibility: 'private' }`
- Parent template reference kept (optional inherit chain)
- Original template unchanged

### Assertions
```
forked = await api.post('/api/workflows/shared/' + token + '/fork')
assert forked.scope === 'user'
assert forked.ownerId === currentUser.id
assert forked.visibility === 'private'
assert forked.id !== originalTemplateId  // new ID
```

---

## TC-WF-003-05: Library discovery — Search by tags + scope filter

**Priority:** P1

### Steps
1. Templates in DB:
   - T1: `{ tags: ['ci-cd', 'nodejs'], visibility: 'company' }`
   - T2: `{ tags: ['deploy', 'nodejs'], visibility: 'company' }`
   - T3: `{ tags: ['ci-cd', 'python'], visibility: 'team' }`
2. `GET /api/workflows/library?scope=company&tag=ci-cd`

### Expected Results
- Returns: [T1] only
- T2 excluded (no ci-cd tag)
- T3 excluded (team scope, not company)

### Assertions
```
result = await api.get('/api/workflows/library?scope=company&tag=ci-cd')
assert result.length === 1
assert result[0].id === T1.id
```

---

## TC-WF-003-06: Only owner can change visibility

**Priority:** P1

### Steps
1. Template owned by userA
2. UserB (not owner, not admin): `PATCH /api/workflows/templates/:id { visibility: 'company' }`

### Expected Results
- Error: `{ code: 'FORBIDDEN' }` (403)
- Visibility unchanged

### Assertions
```
loginAs(userB)
result = await api.patch('/api/workflows/templates/' + templateId, { visibility: 'company' }).catch(e => e)
assert result.status === 403
assert result.body.code === 'FORBIDDEN'
// DB unchanged
assert db.select('orca_workflow_templates', { id: templateId }).visibility === 'private'
```

---

## TC-WF-003-07: Share link expiry

**Priority:** P1

### Steps
1. Create public share link với `expiresAt: now + 1 day`
2. Access at T+0 → success
3. Advance time to T+2 days
4. Access again → expired

### Expected Results
- T+0: 200 OK
- T+2 days: 410 GONE / 404

### Assertions
```
advanceTime(2 * 24 * 60 * 60 * 1000)  // 2 days
response = await fetch('/api/workflows/shared/' + token)
assert response.status === 410 || response.status === 404
```

---

*TC-WF-003 — Orca v5.0 — Updated 2026-08-01*
