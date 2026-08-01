# Enterprise Migration Impact Assessment

> **Scope**: Đánh giá tác động cho các tổ chức enterprise khi nâng cấp từ Orca v4 → v5 → v6
> **Ngày**: 2026-07-30
> **Version nguồn**: v4.x (pre-enterprise features)
> **Version đích**: v6.0 (full enterprise stack: Profile Hierarchy, AI Providers, Workflow, Task Graph, Dev Server Agent)

---

## Tổng quan Migration Path

```
v4.x (Desktop-only, single user)
    │
    ▼ Phase 1: Web Server Mode (F22–F31)
    │ → Multi-user auth, fleet management, multi-DB
    │ → Migrations 0001–0005
    │
    ▼ Phase 2: Enterprise Features (F32–F37)
    │ → Profile Hierarchy, Project Binding, AI Providers
    │ → Workflow Orchestration, Task Graph
    │ → Migrations 0006–0010
    │
    ▼ Phase 3: Dev Server Agent v6 (F38–F39)
      → Project Workspace, Remote Git UI
      → Agent binary update, HMAC context
```

---

## 1. Breaking Changes theo Version

### 1.1 v4.x → v5.0 (Phase 1+2)

| Component | Breaking Change | Impact | Mitigation |
|---|---|---|---|
| **Auth** | E2EE deviceToken → HTTP-only session cookie | Tất cả web clients cần re-authenticate | Auto-redirect to /login page |
| **Database** | `orca-data.json` → SQL database | Desktop SQLite vẫn hoạt động; Server mode cần `ORCA_DB_URL` | Set `ORCA_STORAGE_BACKEND=sql` |
| **Relay Protocol** | SSH relay protocol v1.4 → v1.5 | Old relay binaries không tương thích | Auto-deploy relay binary mới qua SFTP |
| **API** | `/api/*` routes thêm auth middleware | Tất cả API calls cần `orca_session` cookie | Client library update |
| **RPC Namespace** | `accounts.*` → `aiProvider.*` | AI provider RPC methods đổi tên | Client code migration |

### 1.2 v5.0 → v6.0 (Phase 3)

| Component | Breaking Change | Impact | Mitigation |
|---|---|---|---|
| **Dev Server Agent** | agent.js v1 → v6.0 binary | Old agents không hỗ trợ HMAC context | Force re-deploy tất cả agents |
| **RPC Context** | Bare RPC → signed `_ctx` field | Agent từ chối requests không có signature | Upgrade Orca Server trước Agent |
| **PTY Session** | Global PTY → per-userId PTY | Existing PTY sessions bị invalidate | Sessions cleanup on upgrade |
| **Git Author** | Global git config → per-user inject | Commits cũ có author từ server global | Document author change |

---

## 2. Database Migration Impact

### 2.1 Migration Changelog

```
0001 (initial):       orca_worktrees, orca_sessions
0002 (automations):   orca_automations
0003 (sessions):      orca_workspace_sessions
0004 (app_tables):    orca_dev_servers, orca_users, orca_access_policies
0005 (auth):          orca_audit_log + bcrypt column to orca_users

──────────── v5.0 BOUNDARY ────────────

0006 (profile):       orca_company (singleton), orca_departments
                      ALTER orca_users ADD department_id, profile_json

0007 (project):       orca_projects, orca_project_members

0008 (ai-providers):  orca_ai_provider_accounts, orca_provider_usage

0009 (workflow):      orca_workflow_templates, orca_workflow_executions,
                      orca_step_executions

0010 (task-graph):    orca_tasks, orca_task_edges, orca_task_grants,
                      orca_task_comments

──────────── v6.0 BOUNDARY ────────────
(No new migrations — v6.0 adds Dev Server Agent changes only)
```

### 2.2 ALTER TABLE Operations (non-destructive)

```sql
-- Migration 0006: non-destructive ALTER TABLE orca_users
ALTER TABLE orca_users ADD COLUMN department_id TEXT;
ALTER TABLE orca_users ADD COLUMN profile_json  TEXT DEFAULT '{}';
-- Existing rows: department_id = NULL (no department), profile_json = '{}'
-- Effect: Existing users inherit Company profile only (no dept override)
```

### 2.3 Data Migration Required

| Migration | Manual Data Action | Script |
|---|---|---|
| 0006 | Create default Company record | `INSERT INTO orca_company (id, name) VALUES ('default', 'My Company')` |
| 0007 | Create initial Projects (admin task) | Qua Admin Panel `/admin/projects` |
| 0008 | Register AI Provider accounts | Qua Settings → AI Providers |
| 0009 | Import workflow templates | Qua UI hoặc API |
| 0010 | Existing issue tracking → Tasks | Manual import hoặc GitHub/Linear sync |

---

## 3. Infrastructure Changes Required

### 3.1 Environment Variables

```bash
# v4.x (existing)
ORCA_USER_DATA_PATH=/data/orca
ORCA_RELAY_SECRET=<32-byte secret>

# v5.0 additions (REQUIRED)
ORCA_DB_URL=postgresql://orca:pass@db:5432/orca   # hoặc mysql:// hoặc file://
ORCA_STORAGE_BACKEND=sql                           # bắt buộc cho multi-user

# v6.0 additions (REQUIRED for Agent)
ORCA_AI_CREDENTIAL_KEY=<32-byte master key>        # AES key cho AI credentials
# Trên Dev Server Agent:
ORCA_RELAY_SECRET=<same 32-byte secret>            # Phải khớp với Orca Server
```

### 3.2 Docker Compose Changes

```yaml
# v4.x
services:
  orca-server:
    image: orca:4.x
    volumes:
      - orca-data:/data/orca

# v5.0+ additions
services:
  orca-server:
    image: orca:5.x
    environment:
      - ORCA_DB_URL=postgresql://orca:pass@db:5432/orca
      - ORCA_STORAGE_BACKEND=sql
      - ORCA_AI_CREDENTIAL_KEY=${ORCA_AI_CREDENTIAL_KEY}
    depends_on:
      - db
      - redis           # nếu dùng Redis cho session store

  db:
    image: postgres:16  # hoặc mysql:8 hoặc tidb
    environment:
      POSTGRES_USER: orca
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: orca
    volumes:
      - db-data:/var/lib/postgresql/data
```

### 3.3 Dev Server Agent (v6.0) — Update Required

```bash
# Trên mỗi Dev Server, phải update agent binary:
# v5 còn dùng deploy/dev/agent/agent.js (Node.js script)
# v6 dùng compiled single binary từ src/agent/

# Deploy mới (tự động qua Orca UI):
# Settings → Dev Servers → <server> → [Update Agent]
# Hoặc thủ công:
curl -sSL https://orca-releases.example.com/agent-linux-x64 -o /usr/local/bin/orca-agent
chmod +x /usr/local/bin/orca-agent
sudo systemctl restart orca-agent
```

---

## 4. Security Impact Analysis

### 4.1 New Security Requirements

| Requirement | Applies To | Detail |
|---|---|---|
| `ORCA_AI_CREDENTIAL_KEY` | Server + All Dev Servers | 32-byte random key, phải rotate định kỳ |
| `ORCA_RELAY_SECRET` | Server + All Dev Servers | Phải khớp nhau, dùng cho HMAC-SHA256 |
| SSH key cho relay | Orca Server container | `/home/orca/.ssh/id_ed25519` (mount volume) |
| TLS for DB connection | Orca Server → DB | Khuyến nghị dùng TLS: `?sslmode=require` |
| Admin account | Orca Web | Tạo admin user đầu tiên qua CLI: `orca user create --admin` |

### 4.2 New Audit Trail Coverage

```
v4.x audit: Không có
v5.0 audit (orca_audit_log):
  + auth.login.success / auth.login.failed
  + auth.logout
  + user.create / user.update / user.deactivate
  + session.revoke
  + server.connect / server.disconnect
  + agent.spawn / agent.stop

v6.0 audit additions:
  + ai_provider.register / ai_provider.delete
  + ai_provider.credential.write
  + task.create / task.update / task.runAgent
  + workflow.start / workflow.complete / workflow.failed
  + git.push / git.createPR
```

### 4.3 Credential Security Changes

| Area | v4.x | v5.0+ | Change |
|---|---|---|---|
| AI API Keys | Plain env var on server | AES-256-GCM on Dev Server, zero plaintext on Orca Server | **Major improvement** |
| Git credentials | Global `~/.gitconfig` | Per-user `GH_CONFIG_DIR` isolation | **Major improvement** |
| Session token | 48-hex E2EE deviceToken | 64-hex HTTP-only cookie (8h sliding) | Mode change |
| SSH key | User-managed on server | Mounted volume in container | Unchanged |

---

## 5. API Breaking Changes

### 5.1 RPC Method Renames

| Old Method (v4) | New Method (v5/v6) | Breaking? |
|---|---|---|
| `accounts.list` | `aiProvider.list` | ✅ Breaking |
| `accounts.add` | `aiProvider.register` | ✅ Breaking |
| `accounts.remove` | `aiProvider.delete` | ✅ Breaking |
| `ssh.listTargets` | `ssh.listTargets` (unchanged) | ✅ Safe |
| `orchestration.create` | `orchestration.create` (unchanged) | ✅ Safe |
| `project.list` | `projects.list` (plural) | ✅ Breaking |
| N/A | `profile.getEffective` | ✅ New |
| N/A | `projects.create` | ✅ New |
| N/A | `tasks.*` (12 methods) | ✅ New |
| N/A | `workflow.*` (10 methods) | ✅ New |

### 5.2 Payload Changes

```typescript
// v4.x — agent spawn:
rpc('orchestration.create', { worktreeId, agentId, prompt })

// v6.0 — agent spawn (via TaskAgentExecutor):
rpc('tasks.runAgent', { taskId })
// Hoặc trực tiếp qua project context:
rpc('projects.routeAgentSpawn', { projectId, prompt })
// → Automatically: resolve profile, find server, inject credentials
```

---

## 6. Migration Checklist

### Phase 1: Web Server Mode (v4 → v5.0 Baseline)

- [ ] Set `ORCA_DB_URL` và `ORCA_STORAGE_BACKEND=sql`
- [ ] Provision database (PostgreSQL/MySQL recommended for HA)
- [ ] Run Orca Server v5.0 → migrations 0001–0005 auto-apply
- [ ] Create first admin user: `orca user create --admin --email admin@co.com`
- [ ] Configure nginx/reverse proxy (WSS passthrough `:6768`, HTTPS `:6769`)
- [ ] Update all Dev Server agents (auto-deploy via UI)
- [ ] Test: Login via browser → `/login` page
- [ ] Test: Admin Panel `/admin` accessible

### Phase 2: Enterprise Features (v5.0 Baseline → v5.0 Full)

- [ ] Run migrations 0006–0010 (auto-apply on server start)
- [ ] Create Company profile (Admin Panel → Company Settings)
- [ ] Create Departments và assign users
- [ ] Create Projects và bind to Dev Servers
- [ ] Register AI Provider accounts (Settings → AI Providers)
- [ ] Set `ORCA_AI_CREDENTIAL_KEY` on all Dev Servers
- [ ] Create sample Workflow template
- [ ] Create sample Task và test AI decompose
- [ ] Test end-to-end: Task → Agent → Git → PR

### Phase 3: Dev Server Agent v6.0

- [ ] Set `ORCA_RELAY_SECRET` trên Orca Server VÀ tất cả Dev Servers (phải khớp)
- [ ] Deploy Agent v6.0 binary to all Dev Servers
- [ ] Restart systemd service: `sudo systemctl restart orca-agent`
- [ ] Test: Agent handshake với HMAC context
- [ ] Test: Per-userId PTY isolation (2 users, 2 terminals → không cross)
- [ ] Test: Remote Git UI (Explorer, diff, commit, push)
- [ ] Test: AI commit message generation
- [ ] Test: PR creation via `gh` CLI on Dev Server

---

## 7. Rollback Plan

### v5.0 → v4.x Rollback

> [!WARNING]
> Rollback mất toàn bộ data trong migrations 0003–0005 (users, sessions, audit log).

```bash
# 1. Stop Orca Server v5.0
# 2. Backup database
pg_dump orca > orca-backup-$(date +%Y%m%d).sql

# 3. Drop v5 tables (manual)
psql orca -c "DROP TABLE orca_audit_log, orca_access_policies, orca_workspace_sessions CASCADE;"

# 4. Start Orca Server v4.x
# → Sẽ không chạy migrations 0003–0005 (không biết chúng)
```

### Phase 2 → Phase 1 Rollback

> [!CAUTION]
> Rollback mất toàn bộ task graph, workflow history, AI provider accounts, projects.

```bash
# Drop migrations 0006–0010 tables:
psql orca -c "
  DROP TABLE orca_task_comments, orca_task_grants, orca_task_edges, orca_tasks CASCADE;
  DROP TABLE orca_step_executions, orca_workflow_executions, orca_workflow_templates CASCADE;
  DROP TABLE orca_provider_usage, orca_ai_provider_accounts CASCADE;
  DROP TABLE orca_project_members, orca_projects CASCADE;
  DROP TABLE orca_departments, orca_company CASCADE;
  -- Remove added columns:
  ALTER TABLE orca_users DROP COLUMN IF EXISTS department_id;
  ALTER TABLE orca_users DROP COLUMN IF EXISTS profile_json;
"
```

---

## 8. Performance Impact

### 8.1 Database Query Load (v5.0+)

| Operation | Frequency | Added Queries |
|---|---|---|
| Profile resolution | Per-agent-spawn (cached 60s) | 3 queries: user + dept + company |
| Session validation | Per WebSocket message | 1 query (indexed on token) |
| Git status poll | Every 5s per open workspace | 1 relay call → `git status` |
| Fleet health check | Every 30s | 1 query per server |
| Workflow progress | Per step completion | 2 queries: UPDATE step + execution |

### 8.2 Profile Cache Impact

```
Without cache: N agent spawns × 3 DB queries each
With cache (TTL=60s): N agent spawns × 0 queries (cache hit) for 60s

→ Cache hit rate expected: >95% for active teams
→ Estimated DB query reduction: 95%+ for profile-related queries
```

### 8.3 SSH Relay Pool Impact

```
v4.x (no pool): N users × 1 SSH connection each → N SSH connections
v5.0 (pooled): M dev servers × 1 SSH connection → M SSH connections (M << N)

→ For 50 users, 3 dev servers: 50 SSH → 3 SSH (94% reduction)
```

---

## 9. Feature Availability Matrix (Enterprise Rollout)

| Feature | v4.x | v5.0 Baseline | v5.0 Full | v6.0 |
|---|---|---|---|---|
| Desktop App | ✅ | ✅ | ✅ | ✅ |
| Web Server Mode | ❌ | ✅ | ✅ | ✅ |
| Multi-User Auth | ❌ | ✅ | ✅ | ✅ |
| Admin Panel | ❌ | ✅ | ✅ | ✅ |
| Multi-Database | ❌ | ✅ | ✅ | ✅ |
| Fleet Management | ❌ | ✅ | ✅ | ✅ |
| Team RBAC | ❌ | Partial | ✅ | ✅ |
| Profile Hierarchy (F33) | ❌ | ❌ | ✅ | ✅ |
| Project Binding (F34) | ❌ | ❌ | ✅ | ✅ |
| AI Provider Mgmt (F35) | ❌ | ❌ | ✅ | ✅ |
| Workflow Orchestration (F36) | ❌ | ❌ | ✅ | ✅ |
| Task Graph (F37) | ❌ | ❌ | ✅ | ✅ |
| Project Workspace (F38) | ❌ | ❌ | ❌ | ✅ |
| Remote Git UI (F39) | ❌ | ❌ | ❌ | ✅ |
| Per-userId PTY Isolation | ❌ | ❌ | ❌ | ✅ |
| HMAC-signed Context | ❌ | ❌ | ❌ | ✅ |

---

## 10. Recommended Rollout Strategy

```
Week 1-2: Phase 1 (Web Server Mode)
  → Deploy v5.0 server với database
  → Create admin users
  → Test web browser access
  → Train IT admin team

Week 3-4: Phase 2 (Enterprise Features)
  → Create Company + Department profiles
  → Create Projects, bind to Dev Servers
  → Register AI Provider accounts
  → Pilot group: 5-10 developers

Week 5-8: Phase 3 (Dev Server Agent v6.0)
  → Deploy agent v6.0 to all Dev Servers
  → Enable Remote Git UI for pilot group
  → Collect feedback, monitor performance
  → Full rollout

Week 9+: Stabilization
  → Monitor audit logs
  → Tune profile policies
  → Expand workflow templates
  → Task graph adoption
```

---

## 11. Support & Escalation

| Issue | Flow | Data to Collect |
|---|---|---|
| Migration fails | Check `orca_migrations` table + server logs | `SELECT * FROM orca_migrations ORDER BY id` |
| Agent handshake fails | Check ORCA_RELAY_SECRET mismatch | Agent logs: `sudo journalctl -u orca-agent -n 100` |
| AI key not working | Check AES credential file | `ls -la ~/.orca/ai-providers/*.enc` (Dev Server) |
| Profile not applying | Check cache invalidation | `profile.getEffective()` via RPC console |
| Workflow stuck | Check step_executions | `SELECT * FROM orca_step_executions WHERE status='running'` |

---

## Cross-References

| Resource | Mô tả |
|---|---|
| [multi-user-session.md](../flows/multi-user-session.md) | Auth và per-user sandbox |
| [profile-resolution.md](../flows/profile-resolution.md) | Profile 3-tier merge |
| [ai-provider-credential.md](../flows/ai-provider-credential.md) | AI credential security |
| [project-workspace-switch.md](../flows/project-workspace-switch.md) | Project workspace init |
| [task-agent-execution.md](../flows/task-agent-execution.md) | Task → Agent → PR |
| [workflow-orchestration.md](../flows/workflow-orchestration.md) | Workflow DAG execution |
| [remote-git-ui.md](../flows/remote-git-ui.md) | Remote Git operations |
| [HLD README](../hld/README.md) | HLD v6.0 overview + DB migration map |
| [HLD C2 Containers](../hld/C2-containers.md) | Containers mới v5.0 |
| [HLD C4 Code](../hld/C4-code.md) | Module-level detail (C4.7–C4.11) |
| **[Feature Impact Matrix](../flows/enterprise-migration-impact-assessment.md)** | **Phân tích chi tiết tác động từng feature (F01–F39), risks, và roadmap** |

> [!NOTE]
> Tài liệu này tập trung vào **deployment migration** (infrastructure, DB, environment).
> Xem [Feature Impact Matrix](../flows/enterprise-migration-impact-assessment.md) để biết chi tiết tác động kiến trúc cho từng feature (F01–F39), bao gồm BREAKING/MAJOR/MODERATE/MINOR analysis, risk register (R1–R7), và enterprise rollout roadmap.

