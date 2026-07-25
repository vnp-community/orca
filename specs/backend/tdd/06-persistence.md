# TDD-06: Persistence Layer

**Document:** TDD-06  
**Domain:** SQLite Persistence — Schema, Migrations, Data Model  
**Source files:** `src/main/persistence.ts` (~253KB, 6568 lines)

---

## 1. Tổng quan

Orca dùng **SQLite** (qua `better-sqlite3` hoặc custom store) làm database duy nhất.  
Tất cả state được persist — không có in-memory-only state quan trọng.

```typescript
// src/main/persistence.ts
class Store {
  private state: PersistedState    // in-memory cache
  private filePath: string         // ~/.config/orca/store.json (hoặc .db)

  // Pattern: load → mutate in-memory → flush to disk
  // Không phải SQLite raw; là JSON-based store với schema migrations
}
```

**Lưu ý quan trọng:** Orca dùng **JSON file store**, không phải SQLite raw SQL. Đây là custom persistence layer serialize `PersistedState` thành JSON.

---

## 2. Data Model (`src/shared/types.ts`)

### 2.1 PersistedState — root structure

```typescript
type PersistedState = {
  // Core entities
  projects: Project[]
  repos: Repo[]
  worktrees: WorktreeMeta[]
  projectGroups: ProjectGroup[]
  folderWorkspaces: FolderWorkspace[]

  // SSH
  sshTargets: SshTarget[]
  removedSshTargetTombstones?: RemovedSshTargetTombstone[]
  sshRemotePtyLeases?: SshRemotePtyLease[]
  deletedSshConfigAliases?: string[]

  // Settings
  globalSettings: GlobalSettings
  notificationSettings: NotificationSettings

  // Automations
  automations: Automation[]
  automationRuns: AutomationRun[]

  // UI State
  uiState: PersistedUIState
  workspaceSession: WorkspaceSessionState

  // Onboarding
  onboarding: OnboardingState

  // Terminal layouts
  terminalLayouts?: TerminalLayoutSnapshot[]
  terminalTabs?: TerminalTab[]

  // Feature interactions
  featureInteractions?: Record<FeatureInteractionId, number>
  featureInteractionTelemetryBuckets?: Record<FeatureInteractionId, string>
}
```

### 2.2 Project

```typescript
type Project = {
  id: string
  name: string
  repoIds: string[]              // repos thuộc project
  executionHostId?: ExecutionHostId  // local | ssh-<id>
  sourceControlAiOverrides?: RepoSourceControlAiOverrides
  hostSetup?: ProjectHostSetup   // cách setup project trên execution host
  // ...layout, display settings
}
```

### 2.3 Repo

```typescript
type Repo = {
  id: string
  name: string
  path: string                // absolute path trên execution host
  projectId?: string
  connectionId?: ExecutionHostId  // ssh target id nếu remote
  gitRemoteIdentity?: GitRemoteIdentity  // GitHub/GitLab remote info
  icon?: string               // URL hoặc emoji
  sparsePresets?: SparsePreset[]
  // Agent settings:
  agentModeSettings?: AgentModeSettings
  hookSettings?: RepoHookSettings
  // Source control:
  sourceControlAiOverrides?: RepoSourceControlAiOverrides
}
```

### 2.4 WorktreeMeta

```typescript
type WorktreeMeta = {
  id: string                  // 'wt-<repoId>-<timestamp>'
  repoId: string
  path: string                // git worktree path
  branch?: string
  status?: WorktreeStatus
  agentStatus?: AgentStatusEntry
  lineage?: WorktreeLineage
  // Linked work items:
  linkedWorkItemMetadata?: WorktreeLinkedWorkItemMetadata
  // Session:
  sessionPatch?: WorkspaceSessionPatch
}
```

### 2.5 GlobalSettings

```typescript
type GlobalSettings = {
  // AI providers
  claudeApiKey?: string        // encrypted với Electron safeStorage
  openAiApiKey?: string
  geminiApiKey?: string

  // UI
  theme?: 'light' | 'dark' | 'system'
  language?: string
  fontFamily?: string
  fontSize?: number

  // Terminal
  shellPath?: string
  terminalScrollbackRows?: number
  terminalCustomThemes?: TerminalCustomTheme[]

  // Agent
  agentTrustLevel?: 'minimal' | 'standard' | 'full'
  agentModeDefault?: AgentMode

  // Git
  gitUserName?: string
  gitUserEmail?: string

  // Features
  featureFlags?: Record<string, boolean>
}
```

### 2.6 Automation

```typescript
type Automation = {
  id: string
  name: string
  repoId: string
  trigger: AutomationTrigger      // manual | schedule | event
  schedule?: AutomationSchedule   // cron expression
  steps: AutomationStep[]
  enabled: boolean
  createdAt: number
  updatedAt: number
}

type AutomationRun = {
  id: string
  automationId: string
  number: number                  // run số
  status: 'pending' | 'running' | 'success' | 'failure' | 'cancelled'
  startedAt: number
  finishedAt?: number
  outputSnapshot?: AutomationRunOutputSnapshot
  trigger: AutomationRunTrigger
}
```

---

## 3. Store Methods

```typescript
class Store {
  // ── Projects ─────────────────────────────────────────────
  getProjects(): Project[]
  getProject(id: string): Project | undefined
  addProject(project: Project): void
  updateProject(id: string, updates: ProjectUpdateArgs): Project
  removeProject(id: string): void

  // ── Repos ─────────────────────────────────────────────────
  getRepos(): Repo[]
  getRepo(id: string): Repo | undefined
  addRepo(repo: Repo): void
  updateRepo(id: string, updates: Partial<Repo>): Repo
  removeRepo(id: string): void

  // ── Worktrees ─────────────────────────────────────────────
  getWorktrees(): WorktreeMeta[]
  getWorktree(id: string): WorktreeMeta | undefined
  addWorktree(wt: WorktreeMeta): void
  updateWorktree(id: string, updates: Partial<WorktreeMeta>): WorktreeMeta
  removeWorktree(id: string): void

  // ── SSH Targets ───────────────────────────────────────────
  getSshTargets(): SshTarget[]
  getSshTarget(id: string): SshTarget | undefined
  addSshTarget(target: SshTarget): void
  updateSshTarget(id: string, updates: Partial<SshTarget>): SshTarget | null
  removeSshTarget(id: string): void

  // ── Settings ──────────────────────────────────────────────
  getGlobalSettings(): GlobalSettings
  updateGlobalSettings(updates: Partial<GlobalSettings>): GlobalSettings

  // ── Automations ───────────────────────────────────────────
  getAutomations(): Automation[]
  addAutomation(automation: Automation): Automation
  updateAutomation(id: string, updates: AutomationUpdateInput): Automation
  removeAutomation(id: string): void
  getAutomationRuns(automationId: string): AutomationRun[]
  addAutomationRun(run: AutomationRun): AutomationRun
  pruneAutomationRuns(automationId: string): void
}
```

---

## 4. Migrations

```typescript
// src/main/persistence.ts — migration system
// Tất cả migrations chạy khi store.init()
// Mỗi migration là một function transform state

const MIGRATIONS: Migration[] = [
  { version: 1, up: (state) => { /* add projectGroups */ } },
  { version: 2, up: (state) => { /* migrate ssh targets */ } },
  { version: 3, up: (state) => { /* add automation runs */ } },
  // ... nhiều migrations khác
]

// Pattern: bump version trong store, run migrations sequentially
```

---

## 5. Secure Storage (API Keys)

```typescript
// API keys được encrypt trước khi lưu:
// Electron's safeStorage API dùng OS keychain

import { safeStorage } from 'electron'

// Encrypt trước khi persist:
const encrypted = safeStorage.encryptString(apiKey)
store.updateGlobalSettings({ claudeApiKeyEncrypted: encrypted.toString('base64') })

// Decrypt khi đọc:
const decrypted = safeStorage.decryptString(Buffer.from(encrypted, 'base64'))
```

---

## 6. Profile System

```typescript
// src/main/orca-profiles/profile-index-store.ts
// Multi-profile support: mỗi profile có store riêng
// ~/.config/orca/profiles/<profileId>/store.json

type OrcaProfile = {
  id: string
  name: string
  isDefault: boolean
  userDataPath: string  // profile-specific data dir
}

function ensureActiveOrcaProfile(): OrcaProfile
function initOrcaProfilePaths(profile: OrcaProfile): void
```

---

## 7. Data Paths

```
~/.config/orca/                  # userData
├── store.json                   # main persistence
├── runtime/
│   └── runtime.json             # RPC server metadata
├── logs/                        # observability logs
├── terminal-history/            # PTY scrollback (per daemon session)
├── e2ee-keypair.json            # Curve25519 server keypair
├── daemon/
│   ├── daemon.pid               # daemon process PID
│   ├── daemon.sock              # daemon Unix socket
│   └── daemon.token             # daemon auth token
└── profiles/                   # multi-profile support
    └── <profileId>/
        └── store.json
```

---

## 8. Workspace Session (In-memory + Persist)

```typescript
// WorkspaceSessionState: layout state cho UI
type WorkspaceSessionState = {
  paneLayouts?: Record<string, TerminalPaneLayoutNode>
  browserHistory?: Record<string, string[]>
  terminalScrollbackBuffers?: Record<string, TerminalScrollbackBuffer>
  // ...
}

// Pruned on load để giới hạn kích thước:
pruneLocalTerminalScrollbackBuffers(session)
pruneWorkspaceSessionBrowserHistory(session)
```

---

## Addendum v2.0: Persistence trong Server Mode (restructure_v1) — IMPLEMENTED ✅

> **Date:** 2026-07-23

### initDataPath() trong Server Mode

```typescript
// Trong Electron mode:
// initDataPath(app.getPath('userData')) — nhận argument

// Trong Server mode (qua electron-node-wrapper.ts alias):
// initDataPath() — không argument
// Đọc userData từ NodeApp.getPath('userData') nội bộ
// NodeApp ưu tiên: options.userDataPath > ORCA_USER_DATA_PATH env > ~/.orca
```

### NodeSecureStorage — Thay thế safeStorage

```typescript
// Electron mode: safeStorage (Electron native)
// Server mode: NodeSecureStorage (AES-256-GCM)

// Encryption key: userData/.crypto/storage.key
// Algorithm: AES-256-GCM (authenticated encryption)
// Key file mode: 0o600 (owner read-only)
// IV: 12 bytes random per encrypt

// Tương thích:
// - encryptString(plaintext: string): Buffer → same signature as Electron
// - decryptString(ciphertext: Buffer): string → same signature
// - isEncryptionAvailable(): boolean
```

### Data Directory trong Docker

```
/data/orca/               (ORCA_USER_DATA_PATH=/data/orca)
├── store.json            # Main persistence store
├── orca-profiles.json    # Profile index
├── runtime.json          # RPC server metadata
├── .crypto/
│   └── storage.key       # AES-256-GCM encryption key (0o600)
├── logs/
├── terminal-history/
└── daemon/
    ├── daemon.pid
    ├── daemon.sock
    └── daemon.token
```

Volume mount trong docker-compose:
```yaml
volumes:
  - orca-data:/data/orca
```

Tham khảo: [TDD-10: Platform Layer](./10-platform-layer.md) — NodeSecureStorage

---

## Addendum v3.0: Database Abstraction Layer (sql-server CRs) — IMPLEMENTED ✅

> **Date:** 2026-07-23 | **TDD-12:** [12-database-layer.md](./12-database-layer.md)

### Hai lớp persistence song song

```
Electron mode  →  JSON file store (PersistedState.ts) — UNCHANGED
Server mode    →  IConnectionPool + IStateRepository   — NEW (sql-server CRs)
```

Cả hai tồn tại song song, không xung đột:

```typescript
// Electron: không thay đổi
const store = new Store()  // persistence.ts (JSON)

// Server: thêm mới (sql-server CRs)
const pool = new SqliteSingleConnectionPool(path)  // hoặc GenericConnectionPool
await runMigrations(pool)
const stateRepo = createStateRepository({ pool })  // IStateRepository
```

### IStateRepository (Strangler Fig Pattern)

```typescript
// src/main/repositories/factory.ts
function createStateRepository(options: {
  pool?: IConnectionPool    // → SqlStateRepository
  dataFile?: string         // → JsonFileRepository (wraps Store)
}): IStateRepository

// IStateRepository interface:
interface IStateRepository {
  projects: IProjectRepository       // getAll, get, add, update, remove
  repos: IRepoRepository
  worktrees: IWorktreeRepository
  sshTargets: ISshTargetRepository
  settings: ISettingsRepository
  automations: IAutomationRepository
}
```

### SQL Migrations

3 migrations cross-dialect (SQLite, MySQL, PostgreSQL):
- `0001_initial_schema`: Projects, Repos, SSH Targets, Settings tables
- `0002_add_automations`: Automations + AutomationRuns
- `0003_add_workspace_sessions`: WorkspaceSession table

```typescript
// Auto-run trong server-bootstrap.ts:
await pool.withConnection(async (db) => {
  const runner = new MigrationRunner(db, ALL_MIGRATIONS)
  const applied = await runner.migrate()
})
```

### Health Monitor → Persistence context

```typescript
// DatabaseHealthMonitor là HealthChecker interface:
interface HealthChecker {
  check(): Promise<DatabaseHealthCheck>  // SELECT 1
  startPeriodicCheck(ms: number): void
  stopPeriodicCheck(): void
}

// HealthStatus thresholds:
// latency < 500ms → 'healthy'
// latency ≥ 500ms → 'degraded'
// error → 'unhealthy'
```

### Environment Variables (bổ sung v3.0)

| Variable | Default | Description |
|----------|---------|-------------|
| `ORCA_DB_URL` | _(none)_ | Full DSN (auto-detect dialect) |
| `ORCA_DB_DIALECT` | `sqlite` | Dialect override |
| `ORCA_DB_HOST/PORT/NAME/USER/PASSWORD` | — | Individual fields |

Tham khảo: [TDD-12: Database Layer](./12-database-layer.md)

---

## Addendum v4.0: Auth Schema (login CRs CR-LOGIN-001) — IMPLEMENTED ✅

> **Date:** 2026-07-24 | **Status:** Complete

### Migration 0005 — `0005_add_auth_schema`

```sql
-- Users table (local auth + SSO)
CREATE TABLE orca_users (
  id               TEXT PRIMARY KEY,
  email            TEXT UNIQUE NOT NULL,
  name             TEXT NOT NULL,
  password_hash    TEXT,                    -- NULL nếu SSO-only
  role             TEXT DEFAULT 'developer', -- 'admin' | 'developer'
  provider         TEXT DEFAULT 'none',      -- 'none'|'github'|'google'|'keycloak'
  provider_user_id TEXT,
  avatar_url       TEXT,
  teams            TEXT DEFAULT '[]',        -- JSON array
  projects         TEXT DEFAULT '[]',
  created_at       INTEGER NOT NULL,
  last_login_at    INTEGER,
  is_active        INTEGER DEFAULT 1
);

-- Sessions (HTTP cookie-based, 8h TTL)
CREATE TABLE orca_sessions (
  session_id    TEXT PRIMARY KEY,           -- randomBytes(32).hex = 64 chars
  user_id       TEXT REFERENCES orca_users(id) ON DELETE CASCADE,
  created_at    INTEGER NOT NULL,
  expires_at    INTEGER NOT NULL,           -- created_at + 8h
  last_seen_at  INTEGER,
  ip_address    TEXT,
  user_agent    TEXT
);
CREATE INDEX idx_sessions_user    ON orca_sessions(user_id);
CREATE INDEX idx_sessions_expires ON orca_sessions(expires_at);

-- Audit log (append-only, sync writes)
CREATE TABLE orca_audit_log (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at  INTEGER NOT NULL,
  user_id     TEXT,
  user_email  TEXT,
  action      TEXT NOT NULL,    -- 'login.success', 'user.create', 'session.kill', 'ssh.connect', ...
  detail      TEXT,             -- JSON
  ip_address  TEXT
);
CREATE INDEX idx_audit_user   ON orca_audit_log(user_id, created_at DESC);
CREATE INDEX idx_audit_action ON orca_audit_log(action, created_at DESC);

-- RBAC access policies
CREATE TABLE orca_access_policies (
  id                    TEXT PRIMARY KEY,
  name                  TEXT NOT NULL,
  teams                 TEXT DEFAULT '[]',
  roles                 TEXT DEFAULT '[]',
  users                 TEXT DEFAULT '[]',
  allowed_servers       TEXT DEFAULT '"*"',
  allowed_projects      TEXT DEFAULT '"*"',
  agent_trust           TEXT DEFAULT 'standard',
  can_create_worktrees  INTEGER DEFAULT 1,
  can_delete_worktrees  INTEGER DEFAULT 1,
  can_access_production INTEGER DEFAULT 0,
  created_at            INTEGER NOT NULL,
  updated_at            INTEGER NOT NULL
);
```

Migration có đầy đủ `up()` và `down()`. `ALL_MIGRATIONS` registry hiện có 5 migrations (v1~v5).

### AuthUserStore — bcrypt

Password được hash với bcrypt (12 rounds) trước khi lưu. Không bao giờ persist plaintext.

```typescript
// Chỉ lưu hash, không lưu password
await bcryptHash(input.password, 12)  // → '$2b$12$...'
await bcryptCompare(password, hash)   // → true/false
```

### Session Cleanup

`AuthManager` chạy `setInterval(cleanupExpired, 30 * 60 * 1000)` — tự động xóa sessions expired mỗi 30 phút. `destroy()` gọi khi `shutdown()`.

Tham khảo:
- `src/main/db/migrations/0005_add_auth_schema.ts`
- `src/main/auth/auth-user-store.ts`
- `src/main/auth/auth-session-store.ts`
