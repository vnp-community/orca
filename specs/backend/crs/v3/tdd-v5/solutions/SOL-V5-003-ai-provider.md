# SOL-V5-003: AI Provider Account Management (TDD-16)

**Solution:** SOL-V5-003  
**TDD:** TDD-16 — AI Provider Account Management  
**Date:** 2026-07-28  
**Status:** ✅ IMPLEMENTED  
**Implementation Date:** 2026-07-29  
**Tests:** 43 pass (AIProviderService 16 + ProviderResolver 17 + ProviderHealthChecker 10) | TypeScript: 0 errors  
**Strategy:** Additive-only — reuse `RelayConnectionPool`, `DevServerManager`, credential never traverses server

---

## 1. Phân tích gap

| TDD yêu cầu | Hiện trạng code | Gap |
|-------------|-----------------|-----|
| `src/main/ai-providers/AIProviderService.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/ai-providers/ProviderResolver.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/ai-providers/ProviderHealthChecker.ts` | Không tồn tại | ❌ Tạo mới |
| `src/relay/ai-provider-handler.ts` | Không tồn tại | ❌ Tạo mới |
| `src/shared/ai-provider-types.ts` | Không tồn tại | ❌ Tạo mới |
| Migration 0008 | Không tồn tại | ❌ Tạo mới |

**Code có thể reuse:**
- `RelayConnectionPool.getOrConnect()` từ SOL-006 — gọi relay operations
- `DevServerManager.getServer()` — validate dev server
- `IConnectionPool.query()` — DB operations, pattern từ `auth-session-store.ts`
- `DevServerRelayBridge.call()` — đã có sẵn, dùng cho credential relay

**Dependency:** SOL-006 (RelayConnectionPool)

---

## 2. `src/shared/ai-provider-types.ts`

Đúng theo TDD-16 §2, copy nguyên không thay đổi.

```typescript
export type AIProviderType = 'anthropic' | 'openai' | 'gemini' | 'azure' | 'bedrock' | 'ollama' | 'vllm'
export type AIProviderScope = 'server' | 'project' | 'user'
export type AIProviderStatus = 'pending' | 'active' | 'invalid' | 'quota_exceeded' | 'unreachable'

export interface AIProviderAccount {
  id: string
  devServerId: string
  provider: AIProviderType
  scope: AIProviderScope
  scopeRefId?: string
  label: string
  model?: string
  baseUrl?: string
  status: AIProviderStatus
  lastHealthCheck?: Date
  quotaLimitDay: number
  quotaUsedToday?: number
  createdBy: string
  createdAt: Date
  updatedAt: Date
}

export interface CredentialWriteRequest {
  accountId: string
  encryptedBlob: string
  iv: string
}

export const PROVIDER_ENV_KEYS: Record<AIProviderType, string[]> = {
  anthropic: ['ANTHROPIC_API_KEY'],
  openai:    ['OPENAI_API_KEY'],
  gemini:    ['GEMINI_API_KEY', 'GOOGLE_API_KEY'],
  azure:     ['AZURE_OPENAI_API_KEY', 'AZURE_OPENAI_ENDPOINT'],
  bedrock:   ['AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY', 'AWS_DEFAULT_REGION'],
  ollama:    ['OLLAMA_BASE_URL'],
  vllm:      ['VLLM_BASE_URL', 'VLLM_API_KEY'],
}
```

---

## 3. Migration 0008

### `src/main/db/migrations/0008_ai_providers.ts`

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
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_ai_providers_server
        ON orca_ai_provider_accounts(dev_server_id, status)
    `)

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
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_provider_usage_date
        ON orca_provider_usage(account_id, date DESC)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_provider_usage_date')
    await db.exec('DROP TABLE IF EXISTS orca_provider_usage')
    await db.exec('DROP INDEX IF EXISTS idx_orca_ai_providers_server')
    await db.exec('DROP TABLE IF EXISTS orca_ai_provider_accounts')
  }
}
```

### Update `src/main/db/migrations/index.ts`

```typescript
import { migration0008AiProviders } from './0008_ai_providers'

export const ALL_MIGRATIONS = [
  // ... 0001–0007 ...
  migration0008AiProviders,  // ← NEW
]
```

---

## 4. `src/main/ai-providers/AIProviderService.ts`

```typescript
import type { IConnectionPool } from '../db/pool'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'
import type { AIProviderAccount, AIProviderScope } from '../../shared/ai-provider-types'
import { randomUUID } from 'node:crypto'

export class AIProviderService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly devServerManager: DevServerManager,
    private readonly relayPool: RelayConnectionPool
  ) {}

  async createAccount(params: Omit<AIProviderAccount, 'id' | 'status' | 'createdAt' | 'updatedAt'>): Promise<AIProviderAccount> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.query(
      `INSERT INTO orca_ai_provider_accounts
         (id, dev_server_id, provider, scope, scope_ref_id, label, model, base_url, status, quota_limit_day, created_by, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?, ?)`,
      [id, params.devServerId, params.provider, params.scope, params.scopeRefId ?? null,
       params.label, params.model ?? null, params.baseUrl ?? null, params.quotaLimitDay,
       params.createdBy, now, now]
    )
    return (await this.getAccount(id))!
  }

  async getAccount(accountId: string): Promise<AIProviderAccount | null> {
    const rows = await this.pool.query<Record<string, unknown>>(
      `SELECT id, dev_server_id as devServerId, provider, scope, scope_ref_id as scopeRefId,
              label, model, base_url as baseUrl, status, last_health_check as lastHealthCheck,
              quota_limit_day as quotaLimitDay, created_by as createdBy, created_at as createdAt, updated_at as updatedAt
       FROM orca_ai_provider_accounts WHERE id = ?`,
      [accountId]
    )
    return rows[0] ? this.mapRow(rows[0]) : null
  }

  async listAccounts(devServerId: string, scope?: AIProviderScope): Promise<AIProviderAccount[]> {
    const rows = await this.pool.query<Record<string, unknown>>(
      `SELECT id, dev_server_id as devServerId, provider, scope, scope_ref_id as scopeRefId,
              label, model, base_url as baseUrl, status, last_health_check as lastHealthCheck,
              quota_limit_day as quotaLimitDay, created_by as createdBy, created_at as createdAt, updated_at as updatedAt
       FROM orca_ai_provider_accounts WHERE dev_server_id = ? ${scope ? 'AND scope = ?' : ''}`,
      scope ? [devServerId, scope] : [devServerId]
    )
    return rows.map(r => this.mapRow(r))
  }

  async getAllAccounts(): Promise<AIProviderAccount[]> {
    const rows = await this.pool.query<Record<string, unknown>>(
      `SELECT id, dev_server_id as devServerId, provider, scope, scope_ref_id as scopeRefId,
              label, model, base_url as baseUrl, status, last_health_check as lastHealthCheck,
              quota_limit_day as quotaLimitDay, created_by as createdBy, created_at as createdAt, updated_at as updatedAt
       FROM orca_ai_provider_accounts`
    )
    return rows.map(r => this.mapRow(r))
  }

  async updateAccount(accountId: string, patch: Partial<AIProviderAccount>): Promise<void> {
    const fields: string[] = []
    const values: unknown[] = []
    if (patch.status !== undefined) { fields.push('status = ?'); values.push(patch.status) }
    if (patch.model !== undefined) { fields.push('model = ?'); values.push(patch.model) }
    if (patch.label !== undefined) { fields.push('label = ?'); values.push(patch.label) }
    if (patch.baseUrl !== undefined) { fields.push('base_url = ?'); values.push(patch.baseUrl) }
    if (patch.quotaLimitDay !== undefined) { fields.push('quota_limit_day = ?'); values.push(patch.quotaLimitDay) }
    if (patch.lastHealthCheck !== undefined) { fields.push('last_health_check = ?'); values.push(patch.lastHealthCheck.getTime()) }
    fields.push('updated_at = ?'); values.push(Date.now())
    values.push(accountId)
    await this.pool.query(`UPDATE orca_ai_provider_accounts SET ${fields.join(', ')} WHERE id = ?`, values)
  }

  async deleteAccount(accountId: string): Promise<void> {
    await this.pool.query('DELETE FROM orca_ai_provider_accounts WHERE id = ?', [accountId])
  }

  /**
   * Write credential to dev server via relay.
   * ORCA SERVER NEVER SEES PLAINTEXT CREDENTIAL.
   */
  async writeCredentialToDevServer(accountId: string, encryptedBlob: string, iv: string): Promise<void> {
    const account = await this.getAccount(accountId)
    if (!account) throw new Error('ACCOUNT_NOT_FOUND')

    const server = this.devServerManager.getServer(account.devServerId)
    if (!server) throw new Error('DEV_SERVER_NOT_FOUND')

    const relay = await this.relayPool.getOrConnect(account.devServerId, server)
    await relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })
    await this.updateAccount(accountId, { status: 'pending' })
  }

  async testConnection(accountId: string): Promise<{ ok: boolean; latencyMs: number; error?: string }> {
    const account = await this.getAccount(accountId)
    if (!account) throw new Error('ACCOUNT_NOT_FOUND')

    const server = this.devServerManager.getServer(account.devServerId)
    if (!server) return { ok: false, latencyMs: 0, error: 'DEV_SERVER_NOT_FOUND' }

    try {
      const relay = await this.relayPool.getOrConnect(account.devServerId, server)
      const result = await relay.call('ai.provider.healthCheck', { accountId, provider: account.provider, model: account.model }) as { ok: boolean; latencyMs: number }
      return result
    } catch (err: any) {
      return { ok: false, latencyMs: 0, error: err.message }
    }
  }

  async recordUsage(accountId: string, tokensUsed: number, requests: number, costUsd: number): Promise<void> {
    const date = new Date().toISOString().split('T')[0]
    await this.pool.query(
      `INSERT INTO orca_provider_usage (account_id, date, tokens_used, requests, cost_usd)
       VALUES (?, ?, ?, ?, ?)
       ON CONFLICT(account_id, date) DO UPDATE SET
         tokens_used = tokens_used + excluded.tokens_used,
         requests = requests + excluded.requests,
         cost_usd = cost_usd + excluded.cost_usd`,
      [accountId, date, tokensUsed, requests, costUsd]
    )
  }

  async getUsageToday(accountId: string): Promise<{ tokens: number; requests: number; cost: number }> {
    const date = new Date().toISOString().split('T')[0]
    const rows = await this.pool.query<{ tokens: number; requests: number; cost: number }>(
      'SELECT tokens_used as tokens, requests, cost_usd as cost FROM orca_provider_usage WHERE account_id = ? AND date = ?',
      [accountId, date]
    )
    return rows[0] ?? { tokens: 0, requests: 0, cost: 0 }
  }

  /** Helper for ProfileAwareAgentSpawner */
  async resolveForProject(devServerId: string, projectId: string, userId: string, modelHint?: string): Promise<AIProviderAccount | null> {
    const { ProviderResolver } = await import('./ProviderResolver')
    const resolver = new ProviderResolver(this)
    try {
      return await resolver.resolve({ devServerId, projectId, userId, modelHint })
    } catch {
      return null
    }
  }

  private mapRow(r: Record<string, unknown>): AIProviderAccount {
    return {
      id: r.id as string,
      devServerId: r.devServerId as string,
      provider: r.provider as any,
      scope: r.scope as any,
      scopeRefId: r.scopeRefId as string | undefined,
      label: r.label as string,
      model: r.model as string | undefined,
      baseUrl: r.baseUrl as string | undefined,
      status: r.status as any,
      lastHealthCheck: r.lastHealthCheck ? new Date(r.lastHealthCheck as number) : undefined,
      quotaLimitDay: r.quotaLimitDay as number,
      createdBy: r.createdBy as string,
      createdAt: new Date(r.createdAt as number),
      updatedAt: new Date(r.updatedAt as number),
    }
  }
}
```

---

## 5. `src/main/ai-providers/ProviderResolver.ts`

Đúng theo TDD-16 §4, không thay đổi logic.

---

## 6. `src/main/ai-providers/ProviderHealthChecker.ts`

Đúng theo TDD-16 §5 — background cron, 15-minute interval.

---

## 7. `src/relay/ai-provider-handler.ts`

Đúng theo TDD-16 §6:
- `ai.provider.writeCredential` — AES-256-GCM encrypt → `~/.orca/ai-providers/<id>.enc`
- `ai.provider.readCredential` — decrypt, return credential
- `ai.provider.healthCheck` — minimal API call

> **Key insight:** Relay handler chạy **trên Dev Server**, không phải Orca Server. File này được bundled vào relay binary khi build. Đặt trong `src/relay/` theo đúng vị trí TDD.

---

## 8. server-bootstrap.ts — step 9

```typescript
// Sau step 8 (ProjectService):

// 9. AIProviderService + ProviderHealthChecker
const { AIProviderService } = await import('./ai-providers/AIProviderService')
const { ProviderHealthChecker } = await import('./ai-providers/ProviderHealthChecker')
const aiProviderService = new AIProviderService(pool, devServerManager, relayConnectionPool)
const providerHealthChecker = new ProviderHealthChecker()
providerHealthChecker.start(aiProviderService, relayConnectionPool)
console.log('[ServerBootstrap] ✅ AIProviderService + ProviderHealthChecker initialized')
```

---

## 9. Test files cần tạo

```
src/main/ai-providers/__tests__/
├── AIProviderService.test.ts     (≥ 15 tests)
├── ProviderResolver.test.ts      (≥ 15 tests)
├── ProviderHealthChecker.test.ts (≥ 7 tests)
└── ai-provider-handler.test.ts  (relay, ≥ 3 tests — no Orca Server)
```

**Total: ≥ 40 tests**

---

## 10. Checklist

- [x] `src/shared/ai-provider-types.ts`
- [x] `src/main/db/migrations/0008_ai_providers.ts`
- [x] `src/main/db/migrations/index.ts` — add 0008
- [x] `src/main/ai-providers/AIProviderService.ts`
- [x] `src/main/ai-providers/ProviderResolver.ts`
- [x] `src/main/ai-providers/ProviderHealthChecker.ts`
- [x] `src/relay/ai-provider-handler.ts`
- [x] `src/main/runtime/rpc/methods/ai-provider.ts`
- [x] `src/main/server-bootstrap.ts` — step 9 + extend interface
- [x] Test files (≥ 40 tests)

## 11. Implementation Notes

| Spec Path | Actual Path | Note |
|-----------|------------|------|
| `src/main/runtime/rpc/methods/ai-provider.ts` | `src/main/ai-providers/ai-provider-rpc-handler.ts` | Co-located với domain |
| Bootstrap step 9 | `server-bootstrap.ts` step 11 | Wired at step 11, includes ProviderHealthChecker.stop() in shutdown |

**Test Results:** 43 pass (AIProviderService 16 + ProviderResolver 17 + ProviderHealthChecker 10)  
**Implemented:** 2026-07-29 ✅
