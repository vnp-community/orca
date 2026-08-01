# BL-PRF-03 — Project-Dev Server Assignment

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-PRF-03 |
| **Tên** | Project-Dev Server Assignment |
| **Domain** | Profile Management / Project |
| **Actor** | Admin, Lead |
| **Priority** | P0 |

---

## Mô tả

Admin hoặc Lead tạo project và gắn (bind) nó với một Dev Server cụ thể. Mọi hoạt động của project (worktree, agent, terminal) được auto-route đến server đó.

---

## Luồng tạo Project + Binding

```
Admin/Lead → Projects → New Project
    │
    ├── Input:
    │   - name: 'vnp-blc-backend'
    │   - description: 'Main backend service'
    │   - repoUrl: 'git@github.com:org/vnp-blc.git'
    │   - defaultBranch: 'main'
    │   - devServerId: [chọn từ fleet danh sách servers]  ← BINDING
    │   - repoPath: '/srv/projects/vnp-blc'  (path trên dev server)
    │
    ├── Validate:
    │   - name không trùng trong org
    │   - devServerId tồn tại trong ssh_hosts
    │   - devServerId is online (health check)
    │   - repoPath exists trên dev server: relay.call('fs.exists', repoPath)
    │
    ├── INSERT orca_projects (id, name, ..., dev_server_id, repo_path)
    ├── INSERT orca_project_members (project_id, userId, role='lead')  ← creator is lead
    │
    └── Success: project hiện trong Projects panel của creator
```

---

## Luồng thay đổi Dev Server binding

```
Admin → Projects → [project] → Settings → Change Dev Server
    │
    ├── Warning: "Changing dev server will affect all future worktrees.
    │            Existing worktrees on old server remain unchanged."
    ├── Confirm → UPDATE orca_projects SET dev_server_id = ?
    ├── Notify all project members (WebSocket push event)
    └── audit_log('project.devserver.changed', adminId, projectId, oldId, newId)
```

---

## Membership Management

```
Lead → Projects → [project] → Members → Add Member
    │
    ├── Search user by email/name
    ├── Select role: developer | lead
    ├── INSERT orca_project_members (project_id, user_id, role)
    │
    └── New member: project appears in their Projects panel immediately
```

---

## Project Visibility Rules

```typescript
// User thấy project nếu:
// 1. user là member (orca_project_members)
// 2. AND user có permission truy cập dev server của project (RBAC)
//    - developer: server phải match allowedServerTags trong profile
//    - lead/admin: không giới hạn

function getProjectsForUser(userId: string): OrcaProject[] {
  const userMemberships = db.projectMembers.findByUser(userId)
  const resolvedProfile = resolveProfile(userId)
  
  return userMemberships
    .map(m => db.projects.findById(m.projectId))
    .filter(p => {
      const server = db.sshHosts.findById(p.devServerId)
      return hasServerAccess(userId, server, resolvedProfile)
    })
}
```

---

## Tiêu chí chấp nhận

- [ ] Tạo project với devServerId binding
- [ ] Validate devServerId exists và online
- [ ] Validate repoPath exists trên dev server
- [ ] Thay đổi binding: confirm dialog + audit log + member notification
- [ ] Membership: add/remove members với role
- [ ] `getProjectsForUser()` filter theo membership + RBAC
- [ ] Project settings UI: binding, repoPath, members, agent defaults
