# F34 — Project-Dev Server Binding & Project-Centric Execution

| Trường | Giá trị |
|--------|---------|
| **ID** | F34 |
| **Tên** | Project-Dev Server Binding & Project-Centric Execution |
| **Ưu tiên** | P0 |
| **Trạng thái** | 🚧 Phát triển |
| **Phiên bản** | v5.0+ |
| **ADR References** | ADR-007, ADR-011 |
| **HLD References** | C3.10, C4.8 |

---

## Mô tả

Mỗi **Project** trong Orca Web Mode được gắn với một **Dev Server** cụ thể — là server chứa code của project. Khi user tạo worktree hoặc khởi động agent cho một project, Orca tự động route thực thi đến đúng dev server đó. User không cần chọn server thủ công.

---

## Vấn đề cần giải quyết

Trong Web Mode multi-user không có Project-Dev Server binding:
- User phải tự nhớ "project X code ở server Y"
- Không có guardrail: user có thể vô tình tạo worktree trên sai server
- Agent chạy trên Orca Server, không phải Dev Server — không trực tiếp access code
- Không có project-level context khi prompt agent

---

## Tính năng chi tiết

### Project Registry

```typescript
interface OrcaProject {
  id: string
  name: string                     // 'vnp-blc-backend', 'frontend-app'
  description?: string
  repoUrl: string                  // 'git@github.com:org/repo.git'
  defaultBranch: string            // 'main' | 'develop'
  devServerId: string              // FK → ssh_hosts.id  ← THE BINDING
  repoPath: string                 // path on dev server: '/srv/projects/vnp-blc'
  tags?: string[]                  // ['backend', 'production', 'nodejs']
  createdAt: number
  updatedAt: number
}
```

**Binding rule:** 1 project ↔ 1 dev server (primary). Optional: multiple dev servers (replicas, tiếp theo).

### User-Project Membership

```typescript
interface ProjectMember {
  projectId: string
  userId: string
  role: 'developer' | 'lead' | 'admin'
  joinedAt: number
}
```

User chỉ thấy projects mình là member. Lead/Admin có thể thêm members.

---

### Project-centric Worktree Creation

```
User: "New Worktree for project vnp-blc-backend"
    │
    ▼
ProjectService.getProject('vnp-blc-backend')
    → { devServerId: 'server-alpha', repoPath: '/srv/projects/vnp-blc' }
    │
    ▼
DevServerManager.getConnection(devServerId)
    → active SSH relay connection to server-alpha
    │
    ▼
relay.call('git.worktree.add', {
  basePath: '/srv/projects/vnp-blc',
  branch: 'feature/new-api',
  worktreePath: '/srv/projects/vnp-blc-worktrees/feature-new-api'
})
    │
    ▼
Worktree created ON dev server — same machine as code ✅
```

---

### Project-centric Agent Execution

```
User: "Start Claude Code for project vnp-blc-backend"
    │
    ▼
ProjectService.getProject('vnp-blc-backend')
    → { devServerId: 'server-alpha', repoPath: '/srv/projects/vnp-blc' }
    │
    ▼
Resolve effective profile: getResolvedProfile(userId)
    → { agent: { preferredModel: 'claude-opus-4-5', trustPreset: 'standard' } }
    → { shell: { envVars: { EDITOR: 'vim' }, pathAdditions: [...] } }
    │
    ▼
relay.call('pty.spawn', {
  cmd: 'claude',
  args: ['--trust', 'standard'],
  cwd: '/srv/projects/vnp-blc-worktrees/feature-new-api',
  env: {
    ...profileShellEnv,       // from resolved profile
    ANTHROPIC_MODEL: 'claude-opus-4-5',
    GH_CONFIG_DIR: '/home/dev/.config/gh/userId123/'
  }
})
    │
    ▼
Agent runs ON dev server, WITH code access ✅
PTY stream → WebSocket → Browser terminal
```

---

### Project Dashboard

```
Projects View (sidebar or main panel)
┌──────────────────────────────────────────────┐
│ 📁 My Projects                               │
│                                              │
│ ▼ vnp-blc-backend                            │
│   Server: dev-alpha.vnpblc.internal [●]      │
│   Repo: /srv/projects/vnp-blc                │
│   Branch: main | Worktrees: 3                │
│   [New Worktree] [Start Agent] [Terminal]    │
│                                              │
│ ▼ frontend-app                               │
│   Server: dev-beta.vnpblc.internal [●]       │
│   Repo: /srv/projects/frontend               │
│   Branch: develop | Worktrees: 1             │
│   [New Worktree] [Start Agent] [Terminal]    │
│                                              │
│ + Add Project                                │
└──────────────────────────────────────────────┘
```

### Project Context in Agent Prompts

Khi user gửi prompt, system tự động inject project context:

```typescript
// Injected preamble (invisible to user, prepended to prompt)
const projectContext = `
You are working on project: ${project.name}
Repository: ${project.repoUrl}
Dev Server: ${devServer.hostname}
Working directory: ${worktree.path}
Branch: ${worktree.branch}
Team: ${user.department.name}
`
```

### Project Settings (Lead/Admin)

- **Dev Server binding**: chọn/đổi dev server cho project
- **Repo path** trên dev server
- **Members management**: thêm/xóa members, set roles
- **Default agent settings**: override company/dept defaults cho project cụ thể
- **Webhook**: on agent complete → notify Slack/Teams channel của project

---

### Auto-routing Rules

| Hành động | Auto-route target |
|-----------|-------------------|
| New worktree | `project.devServerId` → relay.call('git.worktree.add') |
| Start agent | `project.devServerId` → relay.call('pty.spawn') |
| Open terminal | `project.devServerId` → relay.call('pty.create') |
| File explorer | `project.devServerId` → relay.call('fs.readDir', project.repoPath) |
| Git operations | `project.devServerId` → relay.call('git.*') |

---

## Luồng người dùng đầy đủ

```
1. Admin tạo Project: "vnp-blc-backend"
   → assign Dev Server: server-alpha
   → set Repo Path: /srv/projects/vnp-blc

2. Lead thêm developer vào project

3. Developer đăng nhập → thấy "vnp-blc-backend" trong Projects panel

4. Developer click "New Worktree"
   → modal: chọn branch/base-ref
   → Orca auto-routes đến server-alpha
   → worktree tạo tại /srv/projects/vnp-blc-worktrees/<branch>

5. Developer click "Start Agent" trong worktree
   → Profile resolved: model=claude-opus-4-5 (from dept), trust=standard
   → Agent spawn trên server-alpha tại worktree path
   → Terminal stream về browser

6. Agent hoàn thành → commit → PR tạo với github.com
   → notification về browser + mobile
```

---

## Tiêu chí chấp nhận

- [ ] `orca_projects` table với `dev_server_id` FK
- [ ] `orca_project_members` table (user-project-role)
- [ ] `ProjectService` CRUD + `getProjectsForUser(userId)`
- [ ] Worktree creation auto-route tới `project.devServerId`
- [ ] Agent spawn auto-route tới `project.devServerId` với `project.repoPath` as cwd
- [ ] Profile-aware env injection khi spawn agent
- [ ] Project Context preamble inject vào agent prompts
- [ ] Projects panel UI — list projects, worktrees, status
- [ ] Project settings: binding, repo path, members, agent defaults
- [ ] User chỉ thấy projects mình là member (RBAC filter)
- [ ] Lead/Admin có thể thêm/xóa project members
- [ ] Auto-detect nếu dev server offline → warning "Project server unavailable"

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Project types | `src/shared/project-types.ts` |
| Project service | `src/main/project/project-service.ts` |
| Project-server router | `src/main/project/project-server-router.ts` |
| Profile-aware spawner | `src/main/project/profile-aware-agent-spawner.ts` |
| Project context injector | `src/main/project/project-context-injector.ts` |
| DB migration | `src/main/db/migrations/0007_project_schema.ts` |
| RPC methods | `src/main/runtime/rpc/methods/projects.ts` |
| Projects Panel UI | `src/renderer/src/components/project/ProjectsPanel.tsx` |
| Project Worktree View | `src/renderer/src/components/project/ProjectWorktreeView.tsx` |
| Project Settings UI | `src/renderer/src/components/project/ProjectSettingsModal.tsx` |
| Members UI | `src/renderer/src/components/project/ProjectMembersPanel.tsx` |

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| Project list load | < 200ms |
| Project-server routing overhead | < 50ms |
| Agent spawn (from click to PTY active) | < 3s |
| Server availability check | < 500ms |
