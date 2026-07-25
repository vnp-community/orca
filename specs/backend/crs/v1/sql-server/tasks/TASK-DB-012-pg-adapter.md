# TASK-DB-012: Tạo `src/main/db/postgresql/pg-adapter.ts` + tests ✅ DONE

**Source:** SOL-DB-002  
**Phase:** 2 | **Effort:** M (1.5–2 giờ) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-001, TASK-DB-002, TASK-DB-003

---

## Objective

Tạo `PostgreSQLAdapter` implements `IAsyncDatabase`. Lazy-load `pg` package.

---

## Files to create

### 1. `src/main/db/postgresql/pg-adapter.ts`

```typescript
import type { IAsyncDatabase, IStatement, IDatabaseCapabilities, BindValue, StatementResult } from '../types'
import { registerDatabaseProvider } from '../provider'
import type { PostgresConfig } from '../config'

class PgStatement implements IStatement {
  constructor(private readonly sql: string, private readonly client: any) {}

  async run(...params: BindValue[]): Promise<StatementResult> {
    const result = await this.client.query(this.sql, params)
    return { changes: result.rowCount ?? 0, lastInsertRowid: 0 }
  }

  async get(...params: BindValue[]): Promise<Record<string, unknown> | undefined> {
    const result = await this.client.query(this.sql, params)
    return result.rows[0]
  }

  async all(...params: BindValue[]): Promise<Record<string, unknown>[]> {
    const result = await this.client.query(this.sql, params)
    return result.rows
  }
}

export class PostgreSQLAdapter implements IAsyncDatabase {
  readonly capabilities: IDatabaseCapabilities = {
    walMode: false, returning: true, nativeJson: true,
    placeholderStyle: 'positional', dialect: 'postgresql'
  }

  private constructor(private readonly client: any) {}

  static async connect(config: {
    host: string; port: number; database: string
    username: string; password: string; ssl?: boolean; schema?: string
  }): Promise<PostgreSQLAdapter> {
    let pg: any
    try {
      pg = await import('pg')
    } catch {
      throw new Error(
        'pg package is not installed. Run: pnpm add pg\n' +
        'pg is required for PostgreSQL support.'
      )
    }

    const client = new pg.Client({
      host: config.host, port: config.port,
      database: config.database, user: config.username,
      password: config.password,
      ssl: config.ssl ? { rejectUnauthorized: true } : undefined
    })
    await client.connect()

    if (config.schema && config.schema !== 'public') {
      await client.query(`SET search_path TO ${config.schema}`)
    }

    return new PostgreSQLAdapter(client)
  }

  async exec(sql: string): Promise<void> {
    await this.client.query(sql)
  }

  async prepare(sql: string): Promise<IStatement> {
    return new PgStatement(sql, this.client)
  }

  async close(): Promise<void> {
    await this.client.end()
  }

  async transaction<T>(fn: () => T | Promise<T>): Promise<T> {
    await this.client.query('BEGIN')
    try {
      const result = await fn()
      await this.client.query('COMMIT')
      return result
    } catch (err) {
      await this.client.query('ROLLBACK')
      throw err
    }
  }

  async query(sql: string, params?: BindValue[]): Promise<Record<string, unknown>[]> {
    const result = await this.client.query(sql, params ?? [])
    return result.rows
  }
}

registerDatabaseProvider({
  dialect: 'postgresql',
  async connect(config) {
    const cfg = config as PostgresConfig
    return PostgreSQLAdapter.connect({
      host: cfg.host, port: cfg.port ?? 5432,
      database: cfg.database, username: cfg.username,
      password: cfg.password ?? '', ssl: cfg.ssl, schema: cfg.schema
    })
  }
})
```

### 2. `src/main/db/postgresql/__tests__/pg-adapter.test.ts`

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { PostgreSQLAdapter } from '../pg-adapter'

const mockClient = {
  query: vi.fn(),
  connect: vi.fn().mockResolvedValue(undefined),
  end: vi.fn().mockResolvedValue(undefined)
}

vi.mock('pg', () => ({ Client: vi.fn(() => mockClient) }))

describe('PostgreSQLAdapter (mocked pg)', () => {
  let adapter: PostgreSQLAdapter

  beforeEach(async () => {
    vi.clearAllMocks()
    mockClient.query.mockResolvedValue({ rows: [], rowCount: 0 })
    adapter = await PostgreSQLAdapter.connect({
      host: 'localhost', port: 5432, database: 'test',
      username: 'postgres', password: 'pass'
    })
  })

  it('capabilities.dialect is postgresql', () => {
    expect(adapter.capabilities.dialect).toBe('postgresql')
  })

  it('capabilities.returning is true', () => {
    expect(adapter.capabilities.returning).toBe(true)
  })

  it('query() returns rows', async () => {
    mockClient.query.mockResolvedValueOnce({ rows: [{ id: 1 }], rowCount: 1 })
    const rows = await adapter.query('SELECT * FROM t')
    expect(rows).toEqual([{ id: 1 }])
  })

  it('query() passes params', async () => {
    mockClient.query.mockResolvedValueOnce({ rows: [], rowCount: 0 })
    await adapter.query('SELECT * FROM t WHERE id = $1', [1])
    expect(mockClient.query).toHaveBeenCalledWith('SELECT * FROM t WHERE id = $1', [1])
  })

  it('exec() calls client.query', async () => {
    await adapter.exec('CREATE TABLE t (id INT)')
    expect(mockClient.query).toHaveBeenCalledWith('CREATE TABLE t (id INT)')
  })

  it('transaction() commits on success', async () => {
    await adapter.transaction(async () => {})
    expect(mockClient.query).toHaveBeenCalledWith('BEGIN')
    expect(mockClient.query).toHaveBeenCalledWith('COMMIT')
  })

  it('transaction() rolls back on error', async () => {
    await expect(
      adapter.transaction(async () => { throw new Error('pg fail') })
    ).rejects.toThrow('pg fail')
    expect(mockClient.query).toHaveBeenCalledWith('ROLLBACK')
  })

  it('close() calls client.end()', async () => {
    await adapter.close()
    expect(mockClient.end).toHaveBeenCalledOnce()
  })
})
```

---

## Verification

```bash
pnpm vitest run src/main/db/postgresql/__tests__/pg-adapter.test.ts
```

Expected: 8/8 unit tests pass

---

## Done criteria

- [x] `PostgreSQLAdapter` implements `IAsyncDatabase`
- [x] `capabilities.returning = true` (PG supports RETURNING)
- [x] Lazy imports `pg`
- [x] Registers `postgresql` provider
- [x] `transaction()` rollback khi fn throws
- [x] Sets `search_path` khi schema != 'public'
- [x] 8/8 unit tests pass (mocked)
