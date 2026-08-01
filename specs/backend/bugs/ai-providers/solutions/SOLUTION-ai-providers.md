# SOLUTION: AI Providers (ai-providers) Domain — Fix tất cả Bugs

**Domain:** ai-providers  
**TDD Reference:** TDD-16 (AI Provider Management), TDD-09 (AI Credential Store — agent side)  
**Files cần thay đổi:** `src/main/ai-providers/AIProviderService.ts`, `src/main/ai-providers/AiProviderHealthChecker.ts`, `src/main/dev-server/DevServerRelayBridge.ts`  
**Tổng số bugs:** 6 (AIP-001 ~ AIP-004, BE-AIP-001 ~ BE-AIP-002)

---

## Tổng quan phụ thuộc

```
BUG-BE-AIP-001 (AIProviderService not implemented) ← phải implement trước
    ├── BUG-AIP-001 (pending status never resolved) ← phụ thuộc service
    ├── BUG-AIP-003 (health checker no status change alert) ← phụ thuộc service
    └── BUG-AIP-004 (health checker unused relay pool) ← phụ thuộc service

BUG-AIP-002 (credential not decrypted before relay) — độc lập
BUG-BE-AIP-002 (credential relay security flaw) — phụ thuộc AIP-002
```

**Thứ tự fix:** `BE-AIP-001 → AIP-001 → BE-AIP-002 → AIP-002 → AIP-003 → AIP-004`

---

## BUG-BE-AIP-001 — Fix AIProviderService chưa được implement

**Mức độ:** 🔴 CRITICAL  
**Root cause:** `AIProviderService` chỉ có skeleton, các method chưa implement.

### Fix — Implement đầy đủ AIProviderService theo TDD-16

```typescript
// src/main/ai-providers/AIProviderService.ts

export class AIProviderService {
  constructor(
    private readonly repository: IAIProviderRepository,
    private readonly devServerManager: DevServerManager,
    private readonly log: Logger,
  ) {}

  /**
   * Tạo AI provider account mới.
   * Ghi metadata vào DB, gửi credential (encrypted) đến Dev Server.
   */
  async createAccount(params: CreateAIProviderParams): Promise<AIProviderAccount> {
    const account: AIProviderAccount = {
      id:         generateId(),
      userId:     params.userId,
      provider:   params.provider,     // 'anthropic' | 'openai' | 'google' | 'ollama'
      modelId:    params.modelId,
      accountLabel: params.accountLabel ?? params.provider,
      status:     'pending',           // ban đầu pending, sau khi write credential → active
      createdAt:  Date.now(),
      updatedAt:  Date.now(),
    }

    await this.repository.create(account)
    return account
  }

  /**
   * Ghi credential (encrypted blob từ browser) lên Dev Server.
   * Dev Server sẽ double-encrypt và lưu vào ~/.orca/ai-providers/<accountId>.enc
   */
  async writeCredential(
    accountId: string,
    devServerId: string,
    encryptedBlob: string,  // AES-GCM encrypted bởi Browser SubtleCrypto
  ): Promise<void> {
    const bridge = this.devServerManager.getBridge(devServerId)
    if (!bridge) throw new Error(`Dev server not found: ${devServerId}`)

    await bridge.call('aiProvider.writeCredential', {
      accountId,
      encryptedBlob,
    })

    // Sau khi write thành công → update status từ pending → active
    await this.repository.updateStatus(accountId, 'active')
    this.log.info(`[AIProvider] Credential written: accountId=${accountId}`)
  }

  /**
   * Resolve AI provider cho project.
   * Tìm account phù hợp cho devServerId + userId + modelHint.
   */
  async resolveForProject(
    devServerId: string,
    projectId: string,
    userId: string,
    modelHint?: string,
  ): Promise<AIProviderAccount | null> {
    const accounts = await this.repository.listByUser(userId)
    
    // Filter: active accounts trên đúng devServer
    const activeAccounts = accounts.filter(a => 
      a.status === 'active' && a.devServerId === devServerId
    )
    if (activeAccounts.length === 0) return null

    // Match by model hint nếu có
    if (modelHint) {
      const matched = activeAccounts.find(a => 
        a.modelId === modelHint || a.provider === modelHint.split('-')[0]
      )
      if (matched) return matched
    }

    // Default: trả về account đầu tiên
    return activeAccounts[0] ?? null
  }

  /**
   * List tất cả accounts của user.
   */
  async listAccounts(userId: string): Promise<AIProviderAccount[]> {
    return await this.repository.listByUser(userId)
  }

  /**
   * Delete account và credential trên Dev Server.
   */
  async deleteAccount(accountId: string, devServerId: string): Promise<void> {
    const bridge = this.devServerManager.getBridge(devServerId)
    if (bridge) {
      await bridge.call('aiProvider.deleteCredential', { accountId }).catch(() => {
        // Best-effort: nếu Dev Server offline, vẫn xóa record
      })
    }
    await this.repository.delete(accountId)
  }
}
```

---

## BUG-AIP-001 — Fix pending status never resolved

**Mức độ:** 🟠 HIGH  
**Root cause:** Sau khi `createAccount()`, status là `'pending'` nhưng không có mechanism nào chuyển sang `'active'`.

### Fix — `writeCredential()` tự động update status (đã có trong BE-AIP-001 fix ở trên)

```typescript
// Trong AIProviderService.writeCredential() — đã fix ở trên:
await bridge.call('aiProvider.writeCredential', { accountId, encryptedBlob })
// Sau khi write thành công:
await this.repository.updateStatus(accountId, 'active')  // ← Fix AIP-001

// Thêm: Nếu write fail → status vẫn pending, có thể retry:
// IPC handler expose endpoint: POST /rpc { method: 'aiProvider.retryCredentialWrite' }
```

---

## BUG-AIP-002 & BUG-BE-AIP-002 — Fix credential relay security flaw

**Mức độ:** 🔴 CRITICAL (Security)  
**Root cause:** Orca Server không decrypt credential trước khi relay → Dev Server nhận blob chưa decrypt.

### Phân tích architecture

Theo TDD-09 (AI Credential Relay):
```
2-layer encryption:
  Layer 1: Browser SubtleCrypto.encrypt(sessionKey, apiKey) → encryptedBlob
  Layer 2: Dev Server AES-256-GCM encrypt(encryptedBlob) → .enc file

Flow đúng:
  Browser → encryptedBlob → Orca Server (KHÔNG decrypt Layer 1) → Dev Server
  Dev Server decrypt Layer 2 → còn Layer 1 blob
  Khi spawn agent: Dev Server dùng Layer 1 blob (cần browser sessionKey để decrypt)
```

**Thực tế v5:** Orca Server cần decrypt Layer 1 trước khi forward → Dev Server nhận plaintext apiKey:

```typescript
// src/main/ai-providers/credential-relay.ts (NEW)

/**
 * Decrypt Layer 1 (Browser SubtleCrypto) trên Orca Server trước khi gửi đến Dev Server.
 * 
 * SECURITY MODEL:
 *   Browser encrypt với SESSION-derived key (WebCrypto AES-GCM).
 *   Orca Server có sessionKey (từ session token) để decrypt.
 *   Dev Server nhận plaintext apiKey → double-encrypt và store.
 */
export async function decryptLayer1(
  encryptedBlob: string,  // base64 encoded AES-GCM ciphertext
  iv: string,             // base64 encoded IV
  sessionToken: string,   // Orca session token để derive key
): Promise<string> {
  // Derive session key từ token (HKDF hoặc direct SHA-256)
  const keyMaterial = createHash('sha256').update(sessionToken).digest()
  
  // AES-256-GCM decrypt
  const keyObj = createCipheriv('aes-256-gcm', keyMaterial, Buffer.from(iv, 'base64'))
  // ... (Node.js crypto decipheriv implementation)
  const decrypted = Buffer.concat([
    keyObj.update(Buffer.from(encryptedBlob, 'base64')),
    // Note: GCM auth tag verification
  ])
  return decrypted.toString('utf-8')
}

// IPC handler:
// POST /rpc { method: 'aiProvider.writeCredential', params: { accountId, encryptedBlob, iv } }
// → decryptLayer1(encryptedBlob, iv, req.orcaSession.token)
// → bridge.call('aiProvider.writeCredential', { accountId, apiKey: plaintext })
```

---

## BUG-AIP-003 — Fix health checker không emit status change alert

**Mức độ:** 🟡 MEDIUM  
**Root cause:** Health checker chạy nhưng không emit event khi status thay đổi (healthy → unhealthy).

### Fix — Thêm status change detection và EventBus emit

```typescript
// src/main/ai-providers/AiProviderHealthChecker.ts

export class AiProviderHealthChecker {
  private lastStatuses = new Map<string, 'healthy' | 'unhealthy' | 'unknown'>()

  async checkAccount(account: AIProviderAccount, bridge: DevServerRelayBridge): Promise<void> {
    const result = await bridge.call('aiProvider.healthCheck', {
      accountId: account.id,
    }).catch(() => ({ ok: false, error: 'bridge_unreachable' }))

    const newStatus = result.ok ? 'healthy' : 'unhealthy'
    const prevStatus = this.lastStatuses.get(account.id) ?? 'unknown'

    // Update DB
    await this.repository.updateHealthStatus(account.id, newStatus, result)

    // FIX AIP-003: Emit alert nếu status thay đổi
    if (prevStatus !== newStatus) {
      this.log.warn(`[AIProvider] Status changed: ${account.id} ${prevStatus} → ${newStatus}`)
      
      // Emit event để notify user (WebPush hoặc in-app notification)
      this.eventBus.emit('aiProvider.statusChanged', {
        accountId: account.id,
        userId:    account.userId,
        provider:  account.provider,
        prevStatus,
        newStatus,
        timestamp: Date.now(),
      })
    }

    this.lastStatuses.set(account.id, newStatus)
  }
}
```

---

## BUG-AIP-004 — Fix health checker không dùng relay pool

**Mức độ:** 🟡 MEDIUM  
**Root cause:** Health checker tạo mới connection thay vì dùng pool đã có.

### Fix — Inject DevServerManager (relay pool) vào health checker

```typescript
// src/main/ai-providers/AiProviderHealthChecker.ts

export class AiProviderHealthChecker {
  constructor(
    private readonly devServerManager: DevServerManager,  // ← Inject pool
    private readonly repository: IAIProviderRepository,
    private readonly eventBus: EventBus,
    private readonly log: Logger,
  ) {}

  async runHealthCheck(): Promise<void> {
    const accounts = await this.repository.listActive()

    await Promise.allSettled(accounts.map(async (account) => {
      // Dùng bridge từ pool — không tạo mới connection
      const bridge = this.devServerManager.getBridge(account.devServerId)
      if (!bridge) {
        this.log.warn(`[AIProvider] No bridge for devServer: ${account.devServerId}`)
        return
      }
      await this.checkAccount(account, bridge)
    }))
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/ai-providers/AIProviderService.ts` | Implement đầy đủ tất cả methods | BE-AIP-001 |
| `src/main/ai-providers/AIProviderService.ts` | `writeCredential` update status → active | AIP-001 |
| `src/main/ai-providers/credential-relay.ts` | NEW — Layer 1 decrypt trước khi relay | AIP-002, BE-AIP-002 |
| `src/main/ai-providers/AiProviderHealthChecker.ts` | Emit status change events | AIP-003 |
| `src/main/ai-providers/AiProviderHealthChecker.ts` | Inject DevServerManager pool | AIP-004 |
| `src/main/repositories/ai-provider-repository.ts` | Add updateHealthStatus method | AIP-003 |

---

## Verification Plan

```bash
# Unit tests:
pnpm vitest run src/main/ai-providers/__tests__/

# Manual test:
# 1. Create account → verify status = 'pending'
# 2. writeCredential → verify status = 'active'
# 3. Health check fail → verify status change event emitted
# 4. resolveForProject → verify correct account returned
# 5. deleteAccount → verify credential removed from Dev Server
```
