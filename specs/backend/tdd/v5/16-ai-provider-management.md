# TDD-16: AI Provider Account Management

**Document:** TDD-16 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** AI Provider Accounts — multi-provider, credential relay, quota, health check
**Feature:** F35
**ADR:** ADR-008
**HLD Ref:** C2.14, C3.11a, C4.9
**Source files (to create):**
- `src/main/ai-providers/AIProviderService.ts`
- `src/main/ai-providers/ProviderResolver.ts`
- `src/main/ai-providers/ProviderHealthChecker.ts`
- `src/main/runtime/rpc/methods/ai-provider.ts`
- `src/main/db/migrations/0008_ai_providers.ts`
- Relay: `src/relay/ai-provider-handler.ts`

> **Status: ❌ TODO** — v5.0 proposed; ADR-008: credentials ONLY on Dev Server

---

## 1. Mục tiêu

Admin có thể cấu hình nhiều AI provider accounts (Anthropic, OpenAI, Gemini, Azure, Bedrock, Ollama, vLLM) trên mỗi dev server. **Credentials không bao giờ đi qua Orca Server** — chỉ encrypt trên dev server. Orca Server chỉ lưu metadata.

---

## 2. Data Types

```typescript
// src/shared/ai-provider-types.ts

export type AIProviderType =
  | 'anthropic'
  | 'openai'
  | 'gemini'
  | 'azure'
  | 'bedrock'
  | 'ollama'
  | 'vllm'

export type AIProviderScope = 'server' | 'project' | 'user'
export type AIProviderStatus = 'pending' | 'active' | 'invalid' | 'quota_exceeded' | 'unreachable'

export interface AIProviderAccount {
  id: string
  devServerId: string
  provider: AIProviderType
  scope: AIProviderScope
  scopeRefId?: string       // projectId or userId
  label: string             // human-readable name
  model?: string            // default model
  baseUrl?: string          // for Ollama/vLLM
  status: AIProviderStatus
  lastHealthCheck?: Date
  quotaLimitDay: number     // 0 = unlimited
  quotaUsedToday?: number   // from orca_provider_usage
  createdBy: string
  createdAt: Date
  updatedAt: Date
}

/** Credential write request — encrypted in browser, relayed to dev server */
export interface CredentialWriteRequest {
  accountId: string
  encryptedBlob: string     // base64(AES-GCM encrypted by ORCA_AI_CREDENTIAL_KEY)
  iv: string                // base64(16-byte random IV)
}

/** Env var mapping per provider type */
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

## 3. AIProviderService

```typescript
// src/main/ai-providers/AIProviderService.ts

export class AIProviderService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly devServerManager: DevServerManager,
    private readonly relayPool: RelayConnectionPool
  ) {}

  async createAccount(params: Omit<AIProviderAccount, 'id'|'status'|'createdAt'|'updatedAt'>): Promise<AIProviderAccount>
  async getAccount(accountId: string): Promise<AIProviderAccount | null>
  async listAccounts(devServerId: string, scope?: AIProviderScope): Promise<AIProviderAccount[]>
  async updateAccount(accountId: string, patch: Partial<AIProviderAccount>): Promise<void>
  async deleteAccount(accountId: string): Promise<void>

  /**
   * Write credential to dev server via relay.
   * ORCA SERVER NEVER SEES PLAINTEXT CREDENTIAL.
   *
   * Flow:
   *   Browser → encrypt(apiKey, sessionKey) → POST /rpc
   *   Orca Server → relay.call('ai.provider.writeCredential', encryptedBlob)
   *   Dev Server → decrypt(encryptedBlob, ORCA_AI_CREDENTIAL_KEY) → write .enc file
   */
  async writeCredentialToDevServer(
    accountId: string,
    encryptedBlob: string,
    iv: string
  ): Promise<void> {
    const account = await this.getAccount(accountId)
    if (!account) throw new Error('ACCOUNT_NOT_FOUND')

    const relay = await this.relayPool.getOrConnect(account.devServerId, /* server */)
    await relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })

    // Update status to pending until health check confirms
    await this.updateAccount(accountId, { status: 'pending' })
  }

  /** Test connection by calling relay health check */
  async testConnection(accountId: string): Promise<{ ok: boolean; latencyMs: number; error?: string }>

  /** Record daily usage */
  async recordUsage(accountId: string, tokensUsed: number, requests: number, costUsd: number): Promise<void>

  /** Get usage for today */
  async getUsageToday(accountId: string): Promise<{ tokens: number; requests: number; cost: number }>
}
```

---

## 4. ProviderResolver — Priority Resolution

```typescript
// src/main/ai-providers/ProviderResolver.ts

export interface ProviderResolveContext {
  devServerId: string
  projectId?: string
  userId?: string
  modelHint?: string           // preferred model from profile
  explicitAccountId?: string
}

export class ProviderResolver {
  constructor(private readonly service: AIProviderService) {}

  async resolve(context: ProviderResolveContext): Promise<AIProviderAccount> {
    const { devServerId, projectId, userId, modelHint, explicitAccountId } = context

    // 1. Explicit account
    if (explicitAccountId) {
      const acct = await this.service.getAccount(explicitAccountId)
      if (acct?.status === 'active') return acct
    }

    const allActive = await this.service.listAccounts(devServerId)
      .then(accts => accts.filter(a => a.status === 'active'))

    // 2. User-scope matching modelHint
    if (userId) {
      const userMatch = this.findBestMatch(allActive, 'user', userId, modelHint)
      if (userMatch) return userMatch
    }

    // 3. Project-scope matching modelHint
    if (projectId) {
      const projectMatch = this.findBestMatch(allActive, 'project', projectId, modelHint)
      if (projectMatch) return projectMatch
    }

    // 4. Server-scope matching modelHint
    const serverMatch = this.findBestMatch(allActive, 'server', undefined, modelHint)
    if (serverMatch) return serverMatch

    // 5. Any active server-scope
    const anyServer = allActive.find(a => a.scope === 'server')
    if (anyServer) return anyServer

    throw new Error('NO_AI_PROVIDER_CONFIGURED')
  }

  private findBestMatch(
    accounts: AIProviderAccount[],
    scope: AIProviderScope,
    scopeRefId: string | undefined,
    modelHint?: string
  ): AIProviderAccount | undefined {
    const scopedAccounts = accounts.filter(a =>
      a.scope === scope && (scopeRefId === undefined || a.scopeRefId === scopeRefId)
    )
    if (!modelHint) return scopedAccounts[0]

    // Prefer account whose model matches hint
    return scopedAccounts.find(a => a.model === modelHint) ?? scopedAccounts[0]
  }
}
```

---

## 5. ProviderHealthChecker — Background Cron

```typescript
// src/main/ai-providers/ProviderHealthChecker.ts

const HEALTH_CHECK_INTERVAL_MS = 15 * 60 * 1000  // 15 minutes
const QUOTA_ALERT_THRESHOLD = 0.8                  // 80%

export class ProviderHealthChecker {
  private timer: ReturnType<typeof setInterval> | null = null

  start(service: AIProviderService, relayPool: RelayConnectionPool): void {
    this.timer = setInterval(() => this.runChecks(service, relayPool), HEALTH_CHECK_INTERVAL_MS)
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer)
  }

  private async runChecks(service: AIProviderService, relayPool: RelayConnectionPool): Promise<void> {
    const allAccounts = await service.getAllAccounts()  // across all dev servers

    for (const account of allAccounts) {
      try {
        const result = await service.testConnection(account.id)
        const newStatus = result.ok ? 'active' : 'invalid'
        await service.updateAccount(account.id, {
          status: newStatus,
          lastHealthCheck: new Date(),
        })

        // Quota alert
        if (account.quotaLimitDay > 0) {
          const usage = await service.getUsageToday(account.id)
          const ratio = usage.tokens / account.quotaLimitDay
          if (ratio >= QUOTA_ALERT_THRESHOLD) {
            await emitQuotaAlert(account, ratio)
          }
        }
      } catch {
        await service.updateAccount(account.id, { status: 'unreachable' })
      }
    }
  }
}
```

---

## 6. Relay Handler — `ai.provider.*`

```typescript
// src/relay/ai-provider-handler.ts (NEW on Dev Server relay)

import { createCipheriv, createDecipheriv, scryptSync, randomBytes } from 'node:crypto'
import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs'
import { join, homedir } from 'node:path'

const CREDENTIAL_DIR = join(homedir(), '.orca', 'ai-providers')

function deriveKey(accountId: string): Buffer {
  const masterKey = process.env.ORCA_AI_CREDENTIAL_KEY
  if (!masterKey) throw new Error('ORCA_AI_CREDENTIAL_KEY not set on dev server')
  return scryptSync(`${masterKey}:${accountId}`, accountId, 32) as Buffer
}

export const aiProviderHandlers = {
  'ai.provider.writeCredential': async (params: {
    accountId: string
    encryptedBlob: string   // base64 AES-GCM ciphertext (encrypted by browser with session key)
    iv: string              // base64 IV
  }) => {
    // Note: The relay receives encrypted data from Orca Server.
    // The relay decrypts using ORCA_AI_CREDENTIAL_KEY (on dev server, not on Orca Server).
    const key = deriveKey(params.accountId)
    const iv = Buffer.from(params.iv, 'base64')
    // blob was encrypted with session-derived key — here we re-encrypt with server key
    // (simplified: assume Orca Server relays raw credential after session decryption)
    const credentialData = Buffer.from(params.encryptedBlob, 'base64')

    const reIv = randomBytes(16)
    const cipher = createCipheriv('aes-256-gcm', key, reIv)
    const encrypted = Buffer.concat([cipher.update(credentialData), cipher.final()])
    const authTag = cipher.getAuthTag()

    mkdirSync(CREDENTIAL_DIR, { recursive: true })
    const filePath = join(CREDENTIAL_DIR, `${params.accountId}.enc`)
    writeFileSync(filePath, Buffer.concat([reIv, authTag, encrypted]))
    return { ok: true }
  },

  'ai.provider.readCredential': async (params: { accountId: string }) => {
    const filePath = join(CREDENTIAL_DIR, `${params.accountId}.enc`)
    if (!existsSync(filePath)) throw new Error(`Credential not found: ${params.accountId}`)

    const key = deriveKey(params.accountId)
    const data = readFileSync(filePath)
    const iv = data.subarray(0, 16)
    const authTag = data.subarray(16, 32)
    const ciphertext = data.subarray(32)

    const decipher = createDecipheriv('aes-256-gcm', key, iv)
    decipher.setAuthTag(authTag)
    const decrypted = Buffer.concat([decipher.update(ciphertext), decipher.final()])

    return { credential: decrypted.toString('utf-8') }
  },

  'ai.provider.healthCheck': async (params: { accountId: string; provider: string; model?: string }) => {
    // Make a minimal API call to verify credentials work
    const startMs = Date.now()
    const cred = await aiProviderHandlers['ai.provider.readCredential']({ accountId: params.accountId })
    // ... call provider API with minimal payload ...
    return { ok: true, latencyMs: Date.now() - startMs }
  },
}
```

---

## 7. RPC Methods (`src/main/runtime/rpc/methods/ai-provider.ts`)

```typescript
// namespace: 'aiProvider'

'aiProvider.create'           // (admin) → AIProviderAccount metadata
'aiProvider.get'              // (admin) → AIProviderAccount
'aiProvider.list'             // (admin) → AIProviderAccount[] for devServerId
'aiProvider.update'           // (admin) → void
'aiProvider.delete'           // (admin) → void
'aiProvider.writeCredential'  // (admin) → relay writeCredential → void
'aiProvider.testConnection'   // (admin) → { ok, latencyMs, error? }
'aiProvider.getUsage'         // (admin) → { tokens, requests, cost }
'aiProvider.rotateCredential' // (admin) → write new → 30s grace → deactivate old
```

---

## 8. Key Rotation Procedure

```typescript
async function rotateCredential(
  oldAccountId: string,
  newEncryptedBlob: string,
  newIv: string,
  service: AIProviderService,
  router: ProjectServerRouter
): Promise<void> {
  // 1. Create new account with same config
  const old = await service.getAccount(oldAccountId)!
  const newAccount = await service.createAccount({ ...old, label: old.label + ' (rotating)' })

  // 2. Write new credential to dev server
  await service.writeCredentialToDevServer(newAccount.id, newEncryptedBlob, newIv)

  // 3. Test new credential
  const test = await service.testConnection(newAccount.id)
  if (!test.ok) throw new Error(`Rotation failed: ${test.error}`)

  // 4. 30s grace period (both accounts active)
  await new Promise(resolve => setTimeout(resolve, 30_000))

  // 5. Deactivate old account
  await service.updateAccount(oldAccountId, { status: 'invalid' })
  await service.deleteAccount(oldAccountId)
  await service.updateAccount(newAccount.id, { label: old.label })
}
```

---

## 9. Test Coverage

```
src/main/ai-providers/__tests__/
├── AIProviderService.test.ts
│   ├── createAccount (valid params)
│   ├── listAccounts filtered by scope
│   ├── writeCredentialToDevServer → relay.call invoked (mock relay)
│   ├── recordUsage → orca_provider_usage upsert
│   └── getUsageToday → correct sum
├── ProviderResolver.test.ts
│   ├── resolve: explicit accountId → direct return
│   ├── resolve: user-scope > project-scope > server-scope priority
│   ├── resolve: modelHint matching
│   ├── resolve: no active account → NO_AI_PROVIDER_CONFIGURED
│   └── resolve: inactive account skipped
├── ProviderHealthChecker.test.ts
│   ├── runChecks: active → stays active
│   ├── runChecks: invalid → status updated
│   └── runChecks: quota > 80% → alert emitted
└── ai-provider-handler.test.ts (relay unit)
    ├── writeCredential → .enc file created
    ├── readCredential → correct decryption
    ├── readCredential: missing file → error
    └── healthCheck: mock API call success
```

**Target:** ≥ 40 tests; relay handler tested in isolation without Orca Server
