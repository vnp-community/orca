# BL-WF-03 — Workflow Sharing & Library Discovery

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-WF-03 |
| **Tên** | Workflow Sharing & Library Discovery |
| **Domain** | Workflow Orchestration |
| **Actor** | Any User (share), Admin (approve company), Any User (discover) |
| **Priority** | P1 |

---

## Sharing Flow

```
Owner → workflow → Share Settings
    │
    ├── private → team:
    │   UPDATE visibility='team', team_id=user.departmentId
    │   → team members thấy workflow trong Library (Team section)
    │
    ├── team → company:
    │   Nếu user.role = 'lead': tạo approval request → Admin
    │   Nếu user.role = 'admin': direct publish
    │   INSERT orca_workflow_approvals (workflow_id, requested_by, status='pending')
    │   Admin: approve → visibility='company', xuất hiện trong Company Standards
    │
    └── → public (share link):
        token = randomBytes(16).hex()
        UPDATE workflow SET visibility='public', share_token=token
        share_url = https://orca.company.com/workflows/share/<token>
        Copy link → share via Slack/email
```

## Discovery & Search

```typescript
async function searchWorkflows(userId: string, query: WorkflowSearchQuery) {
  const user = await getUser(userId)
  const userTeamId = user.departmentId

  return db.workflowTemplates.findAll({
    where: {
      AND: [
        // Visibility filter
        {
          OR: [
            { visibility: 'company' },
            { visibility: 'team', team_id: userTeamId },
            { owner_id: userId },  // personal
          ]
        },
        // Text search
        query.text ? {
          OR: [
            { name: { contains: query.text } },
            { description: { contains: query.text } },
            { tags: { contains: query.text } }
          ]
        } : {},
        // Tag filter
        query.tags?.length ? { tags: { containsAny: query.tags } } : {}
      ]
    },
    orderBy: query.sort === 'trending'
      ? [{ usage_count: 'desc' }, { rating: 'desc' }]
      : [{ updated_at: 'desc' }],
    limit: 50
  })
}
```

## Import shared workflow (from share link)

```
Visitor → https://orca.company.com/workflows/share/<token>
    │
    ├── Validate token → load workflow preview (read-only)
    ├── Show: name, description, steps (sanitized), author, usage count
    │
    └── Click "Import to My Workflows":
        INSERT new workflow with owner=visitor, scope='personal',
          parent_template=original.id (or clone — user chooses)
```

---

## Tiêu chí chấp nhận

- [ ] Visibility change: private → team → company → public
- [ ] Company scope yêu cầu admin approval (nếu requester là lead)
- [ ] Share link tạo public token, visitor có thể preview + import
- [ ] Search/filter: text, tags, scope, sort (trending/recent)
- [ ] Rating: 1–5 sao, trung bình hiển thị trong Library
- [ ] Usage count tăng mỗi lần execution
