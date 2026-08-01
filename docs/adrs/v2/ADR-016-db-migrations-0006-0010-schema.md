# ADR-016 — Database Migrations 0006–0010: Enterprise Schema

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-016 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-30 |
| **HLD Ref** | C4.3 (MigrationRunner), C2 (containers 13–16), README (Architecture Layers L4–L5) |
| **CR Ref** | CR-DS-004 |
| **Code Ref** | `src/main/db/migrations/0006-profile.ts`, `0007-project.ts`, `0008-ai-providers.ts`, `0009-workflow.ts`, `0010-task-graph.ts` |
| **Feature Ref** | F33 (Profile), F34 (Project Binding), F35 (AI Providers), F36 (Workflow), F37 (Task Graph) |
| **Supersedes** | — |
| **Amends** | [ADR-002](../v1/ADR-002-multi-database-iconnectionpool.md) |

---

## Bối cảnh

ADR-002 đã quyết định dùng `IConnectionPool + MigrationRunner`. Migrations 0001–0005 đã được implement cho Web Server Mode (auth, users, sessions, fleet). HLD v6.0 bổ sung 5 new domains (Profile, Project, AI Provider, Workflow, Task Graph) đòi hỏi 5 migrations mới — mỗi migration phải:

1. **Idempotent** — chạy lại không gây lỗi
2. **Cross-dialect** — hoạt động trên SQLite, MySQL, PostgreSQL, TiDB
3. **Transaction-safe** — rollback nếu fail
4. **Non-destructive** — không DROP/ALTER existing columns từ migrations trước

---

## Quyết định

### Migration 0006 — Profile Hierarchy Schema

**Rationale:** Profile 3-tier (Company→Dept→User) cần 2 tables mới + ALTER TABLE orca_users.

```sql
-- orca_company: Singleton per Orca server (id='default')
CREATE TABLE orca_company (
  id           TEXT PRIMARY KEY DEFAULT 'default',
  name         TEXT NOT NULL,
  logo_url     TEXT,
  profile_json TEXT NOT NULL DEFAULT '{}',  -- OrcaProfile JSON
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
INSERT OR IGNORE INTO orca_company (id, name, created_at, updated_at)
VALUES ('default', 'My Organization', unixepoch() * 1000, unixepoch() * 1000);

-- orca_departments: Teams within the company
CREATE TABLE orca_departments (
  id           TEXT PRIMARY KEY,
  company_id   TEXT NOT NULL REFERENCES orca_company(id),
  name         TEXT NOT NULL,
  team_lead_id TEXT REFERENCES orca_users(id),
  profile_json TEXT NOT NULL DEFAULT '{}',
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
CREATE INDEX idx_departments_company ON orca_departments(company_id);

-- Extend orca_users (from migration 0004)
ALTER TABLE orca_users ADD COLUMN department_id TEXT REFERENCES orca_departments(id);
ALTER TABLE orca_users ADD COLUMN profile_json  TEXT NOT NULL DEFAULT '{}';
CREATE INDEX idx_users_department ON orca_users(department_id);
```

**Key design choices:**
- `profile_json TEXT DEFAULT '{}'` — JSON blob (không dùng separate columns để dễ thêm fields)
- `orca_company` singleton pattern với `id='default'` — auto-seeded
- `ALTER TABLE` non-destructive — existing users có `department_id=NULL` (inherit company only)

---

### Migration 0007 — Project & Project Membership

**Rationale:** Project gắn Dev Server là core binding (ADR-011). Cần `orca_projects` + `orca_project_members`.

```sql
CREATE TABLE orca_projects (
  id             TEXT PRIMARY KEY,
  name           TEXT NOT NULL UNIQUE,
  description    TEXT,
  repo_url       TEXT,
  repo_path      TEXT NOT NULL,           -- path on Dev Server
  dev_server_id  TEXT REFERENCES ssh_hosts(id),
  default_branch TEXT NOT NULL DEFAULT 'main',
  tags           TEXT NOT NULL DEFAULT '[]',   -- JSON array
  created_by     TEXT REFERENCES orca_users(id),
  created_at     INTEGER NOT NULL,
  updated_at     INTEGER NOT NULL
);
CREATE INDEX idx_projects_devserver ON orca_projects(dev_server_id);
CREATE UNIQUE INDEX idx_projects_name ON orca_projects(name);

CREATE TABLE orca_project_members (
  project_id TEXT NOT NULL REFERENCES orca_projects(id) ON DELETE CASCADE,
  user_id    TEXT NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
  role       TEXT NOT NULL DEFAULT 'developer',  -- developer | lead | admin
  joined_at  INTEGER NOT NULL,
  PRIMARY KEY (project_id, user_id)
);
CREATE INDEX idx_project_members_user ON orca_project_members(user_id);
```

---

### Migration 0008 — AI Provider Accounts

**Rationale:** ADR-008 quyết định credentials on Dev Server. Table này chỉ lưu **metadata** (không có key).

```sql
CREATE TABLE orca_ai_provider_accounts (
  id            TEXT PRIMARY KEY,
  account_name  TEXT NOT NULL,
  provider      TEXT NOT NULL,  -- anthropic | openai | google | azure | bedrock | ollama | vllm
  model         TEXT,           -- default model for this account
  dev_server_id TEXT REFERENCES ssh_hosts(id),
  scope         TEXT NOT NULL DEFAULT 'user',  -- user | project | server-default
  project_id    TEXT REFERENCES orca_projects(id),
  created_by    TEXT REFERENCES orca_users(id),
  is_active     INTEGER NOT NULL DEFAULT 1,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);
CREATE INDEX idx_ai_provider_scope    ON orca_ai_provider_accounts(scope, provider, is_active);
CREATE INDEX idx_ai_provider_user     ON orca_ai_provider_accounts(created_by, provider);
CREATE INDEX idx_ai_provider_project  ON orca_ai_provider_accounts(project_id, provider);

CREATE TABLE orca_provider_usage (
  id         TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES orca_ai_provider_accounts(id) ON DELETE CASCADE,
  user_id    TEXT NOT NULL REFERENCES orca_users(id),
  tokens_in  INTEGER NOT NULL DEFAULT 0,
  tokens_out INTEGER NOT NULL DEFAULT 0,
  cost_usd   REAL NOT NULL DEFAULT 0.0,
  period     TEXT NOT NULL,    -- 'YYYY-MM' format
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX idx_usage_period ON orca_provider_usage(account_id, user_id, period);
```

---

### Migration 0009 — Workflow Templates & Executions

**Rationale:** ADR-009 quyết định DAG + wave execution. Cần 3 tables: templates, executions, step_executions.

```sql
CREATE TABLE orca_workflow_templates (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  scope       TEXT NOT NULL DEFAULT 'personal',  -- company | team | personal
  team_id     TEXT REFERENCES orca_departments(id),
  extends_id  TEXT REFERENCES orca_workflow_templates(id),
  definition  TEXT NOT NULL,    -- JSON: WorkflowDefinition
  created_by  TEXT REFERENCES orca_users(id),
  version     INTEGER NOT NULL DEFAULT 1,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);
CREATE INDEX idx_workflow_scope ON orca_workflow_templates(scope, team_id);

CREATE TABLE orca_workflow_executions (
  id            TEXT PRIMARY KEY,
  template_id   TEXT REFERENCES orca_workflow_templates(id),
  project_id    TEXT REFERENCES orca_projects(id),
  triggered_by  TEXT REFERENCES orca_users(id),
  status        TEXT NOT NULL DEFAULT 'running',  -- running | paused | completed | failed | cancelled
  dag_snapshot  TEXT,                              -- JSON: resolved DAG at execution start
  inputs        TEXT NOT NULL DEFAULT '{}',
  outputs       TEXT NOT NULL DEFAULT '{}',
  started_at    INTEGER NOT NULL,
  completed_at  INTEGER
);
CREATE INDEX idx_wf_executions_project ON orca_workflow_executions(project_id, status);

CREATE TABLE orca_step_executions (
  id            TEXT PRIMARY KEY,
  execution_id  TEXT NOT NULL REFERENCES orca_workflow_executions(id) ON DELETE CASCADE,
  step_id       TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'pending',  -- pending | running | done | failed | skipped
  dev_server_id TEXT,
  stdout        TEXT,
  stderr        TEXT,
  exit_code     INTEGER,
  started_at    INTEGER,
  completed_at  INTEGER
);
CREATE INDEX idx_step_exec_execution ON orca_step_executions(execution_id, status);
```

---

### Migration 0010 — Task Graph

**Rationale:** ADR-010 quyết định dual-edge DAG (parent-child + depends-on) + BFS grant resolution.

```sql
CREATE TABLE orca_tasks (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES orca_projects(id),
  parent_id    TEXT REFERENCES orca_tasks(id),
  title        TEXT NOT NULL,
  description  TEXT,
  status       TEXT NOT NULL DEFAULT 'todo',  -- todo | in_progress | review | done | blocked
  priority     TEXT NOT NULL DEFAULT 'p2',    -- p0 | p1 | p2 | p3
  assignee_id  TEXT REFERENCES orca_users(id),
  reporter_id  TEXT NOT NULL REFERENCES orca_users(id),
  worktree_id  TEXT,
  pr_url       TEXT,
  estimate     REAL,
  actual_hours REAL,
  tags         TEXT NOT NULL DEFAULT '[]',
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
CREATE INDEX idx_tasks_project  ON orca_tasks(project_id, status);
CREATE INDEX idx_tasks_parent   ON orca_tasks(parent_id);
CREATE INDEX idx_tasks_assignee ON orca_tasks(assignee_id, status);

-- Dependency edges (separate from parent-child hierarchy)
CREATE TABLE orca_task_edges (
  from_task_id TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
  to_task_id   TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
  type         TEXT NOT NULL DEFAULT 'blocks',  -- blocks | relates_to
  PRIMARY KEY (from_task_id, to_task_id)
);
CREATE INDEX idx_task_edges_to ON orca_task_edges(to_task_id);

-- Grant system: 5 roles × multiple grantees
CREATE TABLE orca_task_grants (
  task_id    TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
  grantee    TEXT NOT NULL,     -- userId | teamId | 'company' | 'admin'
  role       TEXT NOT NULL,     -- owner | admin | user | viewer
  apply_tree INTEGER NOT NULL DEFAULT 0,  -- inherit to all subtasks
  PRIMARY KEY (task_id, grantee)
);
CREATE INDEX idx_task_grants_grantee ON orca_task_grants(grantee, role);

-- Comments (append-only by design)
CREATE TABLE orca_task_comments (
  id                TEXT PRIMARY KEY,
  task_id           TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
  author_id         TEXT REFERENCES orca_users(id),
  content           TEXT NOT NULL,
  is_internal       INTEGER NOT NULL DEFAULT 0,
  agent_session_id  TEXT,
  created_at        INTEGER NOT NULL
);
CREATE INDEX idx_task_comments_task ON orca_task_comments(task_id);
```

---

## Cross-Dialect Considerations

| SQL Feature | SQLite | MySQL | PostgreSQL | TiDB |
|---|---|---|---|---|
| `TEXT PRIMARY KEY` | ✅ | ✅ (VARCHAR 255) | ✅ | ✅ |
| `REFERENCES` (FK) | ✅ (pragma FK=ON) | ✅ | ✅ | ✅ |
| `ON DELETE CASCADE` | ✅ | ✅ | ✅ | ✅ |
| `INTEGER` timestamp | ✅ | BIGINT | BIGINT | BIGINT |
| `REAL` cost | ✅ | DOUBLE | DOUBLE | DOUBLE |
| `unixepoch()` | ✅ | `UNIX_TIMESTAMP()` | `EXTRACT(epoch...)` | `UNIX_TIMESTAMP()` |
| `INSERT OR IGNORE` | ✅ | `INSERT IGNORE` | `ON CONFLICT DO NOTHING` | `INSERT IGNORE` |

**MigrationRunner** đã có dialect adapter (`src/main/db/migrations/dialect-adapter.ts`) xử lý sự khác biệt này.

---

## Trạng thái Implementation

❌ Migrations 0006–0010 chưa được tạo  
🎯 `src/main/db/migrations/0006-profile.ts`  
🎯 `src/main/db/migrations/0007-project.ts`  
🎯 `src/main/db/migrations/0008-ai-providers.ts`  
🎯 `src/main/db/migrations/0009-workflow.ts`  
🎯 `src/main/db/migrations/0010-task-graph.ts`

---

## Cross-References

| Resource | Mô tả |
|---|---|
| [ADR-002](../v1/ADR-002-multi-database-iconnectionpool.md) | IConnectionPool + MigrationRunner base |
| [ADR-007](../v1/ADR-007-profile-hierarchy-deep-merge.md) | Profile deep-merge consumer của migration 0006 |
| [ADR-008](../v1/ADR-008-ai-provider-credential-on-dev-server.md) | AI credential consumer của migration 0008 |
| [ADR-009](../v1/ADR-009-workflow-dag-orchestration.md) | Workflow DAG consumer của migration 0009 |
| [ADR-010](../v1/ADR-010-task-graph-dag-model.md) | Task graph consumer của migration 0010 |
| [flows/runtime.md](../../flows/runtime.md#11-multi-database-bootstrap-v50) | Migration summary table |
| **HLD C4.3** | MigrationRunner, IDatabase, IConnectionPool |
