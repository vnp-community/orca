# TDD-BE-11: Persistence Layer

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/main/persistence.ts`, `src/main/repositories/`

---

## 1. Dual Persistence Model

Orca duy trì 2 persistence layers song song:

```
┌─────────────────────────────────────────┐
│  Legacy: persistence.ts (Store class)   │
│  - SQLite via better-sqlite3            │
│  - Schema: { key: TEXT, value: TEXT }   │
│  - Sync API (Electron compat)           │
│  - Used by: app state, DevServerStore,  │
│    WebPushManager (subscriptions)       │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│  New: IStateRepository (v3.0)           │
│  - DB-agnostic async API                │
│  - JsonFileRepository (wrap Store)      │
│  - SqlStateRepository (SQL backends)    │
│  - Used by: profile hierarchy (v5.0),   │
│    workflow state, task graph           │
└─────────────────────────────────────────┘
```

---

## 2. Store Class (Legacy)

```typescript
class Store {
  // Sync get/set (SQLite better-sqlite3 — sync by default)
  get<T>(key: string, defaultValue?: T): T
  set<T>(key: string, value: T): void
  delete(key: string): void
  clear(): void

  // Internal: SQLite DB tại userData/orca.db
  // Table: CREATE TABLE IF NOT EXISTS store (key TEXT PRIMARY KEY, value TEXT)
}
```

**initDataPath():** Set userData path từ `platform.app.getPath('userData')`. PHẢI gọi trước `new Store()`.

---

## 3. JsonFileRepository

```typescript
class JsonFileRepository implements IStateRepository {
  constructor(private store: Store) {}

  async get<T>(key: string): Promise<T | undefined> {
    return this.store.get<T>(key)
  }
  async set<T>(key: string, value: T): Promise<void> {
    this.store.set(key, value)
  }
  async delete(key: string): Promise<void> {
    this.store.delete(key)
  }
  async list(): Promise<string[]> {
    // Return all keys từ store table
  }
}
```

---

## 4. SqlStateRepository

```typescript
class SqlStateRepository implements IStateRepository {
  // Dùng IConnectionPool để query `state_kv` table
  // state_kv: { key TEXT PRIMARY KEY, value TEXT, updated_at INTEGER }
  async get<T>(key: string): Promise<T | undefined>
  async set<T>(key: string, value: T): Promise<void>
  async delete(key: string): Promise<void>
  async list(): Promise<string[]>
}
```

---

## 5. Repository Factory

```typescript
function createStateRepository(
  pool: IConnectionPool | null,
  store: Store
): IStateRepository {
  if (pool && pool.dialect !== 'sqlite') {
    return new SqlStateRepository(pool)
  }
  return new JsonFileRepository(store)
}
// SQLite: reuse existing Store (không tạo 2nd SQLite connection)
// MySQL/PG: SqlStateRepository với pool đã khởi tạo
```

---

## 6. DevServerStore

```typescript
class DevServerStore {
  constructor(private store: Store) {}

  list(): PersistedDevServer[]
  get(id: string): PersistedDevServer | undefined
  upsert(ds: PersistedDevServer): void
  remove(id: string): void

  // Persist key: 'devServers.v1' → JSON array
}
```

**PersistedDevServer:**
```typescript
type PersistedDevServer = {
  id:             string
  name:           string
  host:           string
  connectionType: 'relay-ssh' | 'relay-websocket' | 'direct-websocket'
  sshKeyId?:      string
  sshUser?:       string
  relayUrl?:      string
  createdAt:      number
}
```

---

## 7. Data Path Structure

```
userData/                     (ORCA_USER_DATA_PATH || ~/.orca)
├─ orca.db                   ← Store (legacy SQLite)
├─ auth.db                   ← AuthManager (dedicated SQLite)
├─ relay/                    ← Relay binary + versions
│   ├─ <version>/orca-relay
│   └─ current               ← symlink
├─ users/                    ← Per-user isolation (ORCA_MULTI_USER=1)
│   ├─ <userId-A>/
│   │   ├─ orca.sock         ← Unix socket for user process
│   │   └─ orca.db           ← Per-user Store
│   └─ <userId-B>/
│       └─ ...
└─ logs/                     ← Application logs
