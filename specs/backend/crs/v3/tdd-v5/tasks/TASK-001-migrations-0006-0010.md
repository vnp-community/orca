# TASK-001: DB Migrations 0006–0010

**Phase:** 1 — Foundation  
**Solution ref:** [SOL-V5-001](../solutions/SOL-V5-001-profile-hierarchy.md) §3, [SOL-V5-002](../solutions/SOL-V5-002-project-binding.md) §3, [SOL-V5-003](../solutions/SOL-V5-003-ai-provider.md) §3, [SOL-V5-004](../solutions/SOL-V5-004-workflow-orchestration.md) §2, [SOL-V5-005](../solutions/SOL-V5-005-task-graph.md) §2  
**Prerequisite:** None  
**Status:** ✅ DONE — 2026-07-28

> **Kết quả:** 5 migration files tạo thành công. Fixed: migration 0007 dùng `orca_v5_projects` để tránh conflict với `orca_projects` legacy từ migration 0004. Migration 0010 FK updated tương ứng. 167/167 DB tests pass.


---

## Mô tả

Tạo 5 DB migration files mới theo đúng pattern của migration runner hiện tại (`src/main/db/migrations/runner.ts`). Mỗi migration phải implement `Migration` interface với `version`, `name`, `up(db)`, `down(db)`.

---

## Files cần tạo

### 1. `src/main/db/migrations/0006_company_dept.ts`

```typescript
import type { Migration } from './types'

export const migration0006CompanyDept: Migration = {
  version: 6,
  name: 'company_dept',

  async up(db) {
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_companies (
        id            TEXT    PRIMARY KEY,
        name          TEXT    NOT NULL,
        profile_json  TEXT    NOT NULL DEFAULT '{}',
        admin_user_id TEXT,
        created_at    INTEGER NOT NULL,
        updated_at    INTEGER NOT NULL,
        updated_by    TEXT
      )
    `)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_departments (
        id             TEXT    PRIMARY KEY,
        company_id     TEXT    NOT NULL REFERENCES orca_companies(id) ON DELETE CASCADE,
        name           TEXT    NOT NULL,
        parent_dept_id TEXT    REFERENCES orca_departments(id) ON DELETE SET NULL,
        profile_json   TEXT    NOT NULL DEFAULT '{}',
        created_at     INTEGER NOT NULL,
        updated_at     INTEGER NOT NULL,
        updated_by     TEXT
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_departments_company ON orca_departments(company_id)`)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_user_profiles (
        user_id      TEXT    PRIMARY KEY REFERENCES orca_users(id) ON DELETE CASCADE,
        profile_json TEXT    NOT NULL DEFAULT '{}',
        updated_at   INTEGER NOT NULL
      )
    `)
    try {
      await db.exec(`ALTER TABLE orca_users ADD COLUMN department_id TEXT REFERENCES orca_departments(id) ON DELETE SET NULL`)
    } catch { /* idempotent */ }
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_user_profiles')
    await db.exec('DROP INDEX IF EXISTS idx_orca_departments_company')
    await db.exec('DROP TABLE IF EXISTS orca_departments')
    await db.exec('DROP TABLE IF EXISTS orca_companies')
  }
}
```

### 2. `src/main/db/migrations/0007_projects.ts`

```typescript
import type { Migration } from './types'

export const migration0007Projects: Migration = {
  version: 7,
  name: 'projects',

  async up(db) {
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_projects (
        id             TEXT    PRIMARY KEY,
        name           TEXT    NOT NULL,
        description    TEXT,
        dev_server_id  TEXT    NOT NULL,
        repo_path      TEXT    NOT NULL,
        default_branch TEXT    NOT NULL DEFAULT 'main',
        visibility     TEXT    NOT NULL DEFAULT 'team',
        created_by     TEXT    NOT NULL,
        created_at     INTEGER NOT NULL,
        updated_at     INTEGER NOT NULL
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_projects_server ON orca_projects(dev_server_id)`)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_project_members (
        project_id TEXT    NOT NULL REFERENCES orca_projects(id) ON DELETE CASCADE,
        user_id    TEXT    NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        role       TEXT    NOT NULL DEFAULT 'member',
        added_at   INTEGER NOT NULL,
        PRIMARY KEY (project_id, user_id)
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_project_members_user ON orca_project_members(user_id)`)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_project_members_user')
    await db.exec('DROP TABLE IF EXISTS orca_project_members')
    await db.exec('DROP INDEX IF EXISTS idx_orca_projects_server')
    await db.exec('DROP TABLE IF EXISTS orca_projects')
  }
}
```

### 3. `src/main/db/migrations/0008_ai_providers.ts`

```typescript
import type { Migration } from './types'

export const migration0008AiProviders: Migration = {
  version: 8,
  name: 'ai_providers',

  async up(db) {
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_ai_provider_accounts (
        id                TEXT    PRIMARY KEY,
        dev_server_id     TEXT    NOT NULL,
        provider          TEXT    NOT NULL,
        scope             TEXT    NOT NULL DEFAULT 'server',
        scope_ref_id      TEXT,
        label             TEXT    NOT NULL,
        model             TEXT,
        base_url          TEXT,
        status            TEXT    NOT NULL DEFAULT 'pending',
        last_health_check INTEGER,
        quota_limit_day   INTEGER NOT NULL DEFAULT 0,
        created_by        TEXT    NOT NULL,
        created_at        INTEGER NOT NULL,
        updated_at        INTEGER NOT NULL
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_ai_providers_server ON orca_ai_provider_accounts(dev_server_id, status)`)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_provider_usage (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        account_id  TEXT    NOT NULL REFERENCES orca_ai_provider_accounts(id) ON DELETE CASCADE,
        date        TEXT    NOT NULL,
        tokens_used INTEGER NOT NULL DEFAULT 0,
        requests    INTEGER NOT NULL DEFAULT 0,
        cost_usd    REAL    NOT NULL DEFAULT 0,
        UNIQUE(account_id, date)
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_provider_usage_date ON orca_provider_usage(account_id, date DESC)`)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_provider_usage_date')
    await db.exec('DROP TABLE IF EXISTS orca_provider_usage')
    await db.exec('DROP INDEX IF EXISTS idx_orca_ai_providers_server')
    await db.exec('DROP TABLE IF EXISTS orca_ai_provider_accounts')
  }
}
```

### 4. `src/main/db/migrations/0009_workflows.ts`

```typescript
import type { Migration } from './types'

export const migration0009Workflows: Migration = {
  version: 9,
  name: 'workflows',

  async up(db) {
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_workflow_templates (
        id                  TEXT    PRIMARY KEY,
        name                TEXT    NOT NULL,
        version             INTEGER NOT NULL DEFAULT 1,
        parent_template_id  TEXT    REFERENCES orca_workflow_templates(id) ON DELETE SET NULL,
        description         TEXT,
        definition_json     TEXT    NOT NULL DEFAULT '{"steps":[]}',
        owner_id            TEXT,
        scope               TEXT    NOT NULL DEFAULT 'user',
        created_at          INTEGER NOT NULL,
        updated_at          INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_workflow_executions (
        id                  TEXT    PRIMARY KEY,
        definition_snapshot TEXT    NOT NULL,
        status              TEXT    NOT NULL DEFAULT 'pending',
        inputs_json         TEXT    NOT NULL DEFAULT '{}',
        current_wave        INTEGER NOT NULL DEFAULT 0,
        triggered_by        TEXT    NOT NULL,
        project_id          TEXT,
        started_at          INTEGER,
        completed_at        INTEGER,
        error_message       TEXT,
        created_at          INTEGER NOT NULL
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_workflow_exec_status ON orca_workflow_executions(status, created_at DESC)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_workflow_exec_project ON orca_workflow_executions(project_id, status)`)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_workflow_step_executions (
        id            TEXT    PRIMARY KEY,
        execution_id  TEXT    NOT NULL REFERENCES orca_workflow_executions(id) ON DELETE CASCADE,
        step_id       TEXT    NOT NULL,
        status        TEXT    NOT NULL DEFAULT 'pending',
        started_at    INTEGER,
        completed_at  INTEGER,
        output_json   TEXT,
        error_message TEXT
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_step_exec_execution ON orca_workflow_step_executions(execution_id, step_id)`)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_step_exec_execution')
    await db.exec('DROP TABLE IF EXISTS orca_workflow_step_executions')
    await db.exec('DROP INDEX IF EXISTS idx_orca_workflow_exec_project')
    await db.exec('DROP INDEX IF EXISTS idx_orca_workflow_exec_status')
    await db.exec('DROP TABLE IF EXISTS orca_workflow_executions')
    await db.exec('DROP TABLE IF EXISTS orca_workflow_templates')
  }
}
```

### 5. `src/main/db/migrations/0010_tasks.ts`

```typescript
import type { Migration } from './types'

export const migration0010Tasks: Migration = {
  version: 10,
  name: 'tasks',

  async up(db) {
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_tasks (
        id               TEXT    PRIMARY KEY,
        project_id       TEXT    REFERENCES orca_projects(id) ON DELETE SET NULL,
        parent_id        TEXT    REFERENCES orca_tasks(id) ON DELETE CASCADE,
        title            TEXT    NOT NULL,
        description      TEXT,
        type             TEXT    NOT NULL DEFAULT 'task',
        status           TEXT    NOT NULL DEFAULT 'backlog',
        priority         TEXT    NOT NULL DEFAULT 'medium',
        labels           TEXT    NOT NULL DEFAULT '[]',
        visibility       TEXT    NOT NULL DEFAULT 'team',
        reporter_id      TEXT,
        assignee_id      TEXT,
        estimated_hours  REAL,
        progress_percent INTEGER NOT NULL DEFAULT 0,
        ai_context       TEXT,
        prompt_template  TEXT,
        due_date         INTEGER,
        created_at       INTEGER NOT NULL,
        updated_at       INTEGER NOT NULL
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_tasks_project ON orca_tasks(project_id, status)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_tasks_parent ON orca_tasks(parent_id)`)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_task_edges (
        from_task_id TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        to_task_id   TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        edge_type    TEXT NOT NULL DEFAULT 'depends_on',
        created_at   INTEGER NOT NULL DEFAULT (strftime('%s', 'now') * 1000),
        PRIMARY KEY (from_task_id, to_task_id, edge_type)
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_task_edges_from ON orca_task_edges(from_task_id)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_task_edges_to ON orca_task_edges(to_task_id)`)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_task_grants (
        id          TEXT    PRIMARY KEY,
        task_id     TEXT    NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        scope       TEXT    NOT NULL,
        scope_id    TEXT,
        permission  TEXT    NOT NULL,
        apply_tree  INTEGER NOT NULL DEFAULT 0,
        granted_by  TEXT    NOT NULL,
        expires_at  INTEGER,
        created_at  INTEGER NOT NULL
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_task_grants_task ON orca_task_grants(task_id)`)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_task_comments (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        task_id     TEXT    NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        user_id     TEXT    NOT NULL,
        content     TEXT    NOT NULL,
        type        TEXT    NOT NULL DEFAULT 'comment',
        created_at  INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_team_members (
        team_id  TEXT    NOT NULL,
        user_id  TEXT    NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        role     TEXT    NOT NULL DEFAULT 'member',
        added_at INTEGER NOT NULL,
        PRIMARY KEY (team_id, user_id)
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_orca_team_members_user ON orca_team_members(user_id)`)
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_team_members')
    await db.exec('DROP TABLE IF EXISTS orca_task_comments')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_grants_task')
    await db.exec('DROP TABLE IF EXISTS orca_task_grants')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_edges_to')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_edges_from')
    await db.exec('DROP TABLE IF EXISTS orca_task_edges')
    await db.exec('DROP INDEX IF EXISTS idx_orca_tasks_parent')
    await db.exec('DROP INDEX IF EXISTS idx_orca_tasks_project')
    await db.exec('DROP TABLE IF EXISTS orca_tasks')
  }
}
```

---

## Verification

```bash
# Kiểm tra TypeScript types
pnpm tsc --noEmit

# Run migration tests (nếu có)
pnpm test --run src/main/db/migrations/__tests__/
```

## Acceptance Criteria

- [x] 5 migration files tạo thành công
- [x] Mỗi file export đúng tên constant (`migration0006CompanyDept`, ...)
- [x] `up()` và `down()` đều implement
- [x] `version` tăng dần (6, 7, 8, 9, 10)
- [x] Không TypeScript errors
