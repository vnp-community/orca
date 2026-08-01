# AI Provider Credential Write Flow — F35 AI Provider Account Management

> **Scope**: Luồng ghi AI Provider credentials vào Dev Server — **zero plaintext** trên Orca Server
>
> **Key files**:
> - [`src/main/ai-providers/ai-provider-service.ts`](../../src/main/ai-providers/ai-provider-service.ts) — AIProviderService
> - [`src/main/ai-providers/provider-credential-relay.ts`](../../src/main/ai-providers/provider-credential-relay.ts) — ProviderCredentialRelay
> - [`src/main/ai-providers/provider-resolver.ts`](../../src/main/ai-providers/provider-resolver.ts) — AIProviderResolver (priority cascade)
> - [`src/main/ai-providers/provider-health-checker.ts`](../../src/main/ai-providers/provider-health-checker.ts) — ProviderHealthChecker
> - **Feature**: [F35 AI Provider Management](../features/F35-ai-provider-account-management.md)
> - **Business Logic**: [BL-AIP-01](../logic/ai-providers/BL-AIP-01-register-provider-account.md), [BL-AIP-02](../logic/ai-providers/BL-AIP-02-provider-resolution.md), [BL-AIP-03](../logic/ai-providers/BL-AIP-03-provider-health-quota.md)

---

## 1. Nguyên tắc bảo mật cốt lõi

> [!IMPORTANT]
> **Zero Plaintext trên Orca Server**: API key KHÔNG BAO GIỜ được gửi plaintext đến Orca Server.
> Key được mã hóa bởi **browser** (SubtleCrypto) trước khi gửi qua WebSocket.
> Orca Server chỉ relay encrypted blob qua SSH đến Dev Server.
> Chỉ Dev Server mới có thể decrypt (biết `ORCA_AI_CREDENTIAL_KEY`).

```
Browser             Orca Server                Dev Server
  │                      │                          │
  │ API key (raw)         │                          │
  │ encrypt(sessionKey)   │                          │
  │ ─────────────────────►│                          │
  │                       │ relay SSH channel        │
  │                       │ ─────────────────────────►│
  │                       │                          │ decrypt(ORCA_AI_CREDENTIAL_KEY)
  │                       │                          │ write ~/.orca/ai-providers/<id>.enc
  │                       │                          │ (AES-256-GCM)
  │                 [Orca Server không thấy plaintext key]
```

---

## 2. Flow: Đăng ký AI Provider Account

### 2.1 Browser-side Encryption

```
Admin mở Settings → AI Providers → Add Account
  │
  ▼ Nhập:
  │   provider: 'anthropic'
  │   apiKey:   'sk-ant-api03-...'  ← raw plaintext
  │   accountName: 'Production Anthropic'
  │   scope: 'project'  | 'user' | 'server-default'
  │
  ▼ Browser: SubtleCrypto.encrypt()

// src/renderer/src/components/ai-providers/ProviderAccountForm.tsx
const sessionKey = await deriveSessionKey(sessionToken)  // từ E2EE session
const encrypted = await crypto.subtle.encrypt(
  { name: 'AES-GCM', iv: crypto.getRandomValues(new Uint8Array(12)) },
  sessionKey,
  new TextEncoder().encode(apiKey)
)
// encrypted = ArrayBuffer (12-byte IV + ciphertext + 16-byte auth tag)
const encryptedB64 = btoa(String.fromCharCode(...new Uint8Array(encrypted)))

// Gửi qua WebSocket RPC:
// apiKey field = encryptedB64 (không phải raw key!)
```

### 2.2 RPC: aiProvider.register

```typescript
// Browser → WS RPC:
{
  method: 'aiProvider.register',
  params: {
    accountName: 'Production Anthropic',
    provider: 'anthropic',
    model: 'claude-opus-4-5',
    encryptedApiKey: '<base64-encrypted>',  // ← không phải raw key
    devServerId: 'svr-01',
    scope: 'project',
    projectId: 'proj-abc'
  }
}
```

### 2.3 Orca Server: Relay SSH

```typescript
// src/main/ai-providers/provider-credential-relay.ts
async writeCredential(input: WriteCredentialInput): Promise<void> {
  // 1. Generate accountId
  const accountId = randomUUID()

  // 2. INSERT metadata only (NO KEY) vào DB
  await db.run(
    'INSERT INTO orca_ai_provider_accounts (id, account_name, provider, model, dev_server_id, scope, project_id, created_by) VALUES (?,?,?,?,?,?,?,?)',
    [accountId, input.accountName, input.provider, input.model,
     input.devServerId, input.scope, input.projectId, input.userId]
  )

  // 3. Relay encrypted blob → Dev Server (qua SSH channel)
  const relay = await RelayConnectionPool.getOrConnect(input.devServerId)
  await relay.call('ai.provider.writeCredential', {
    accountId,
    provider: input.provider,
    encryptedBlob: input.encryptedApiKey,  // ← vẫn encrypted
    encryptionInfo: input.encryptionInfo,
  })
  // Orca Server trả về → KHÔNG thấy plaintext key
}
```

### 2.4 Dev Server: Decrypt và Write File

```typescript
// src/agent/ai-credentials/credential-writer.ts
// (chạy trên Dev Server)

async writeCredential(params: WriteCredentialParams): Promise<void> {
  const { accountId, provider, encryptedBlob, encryptionInfo } = params

  // 1. Derive decryption key từ master key + accountId
  // key = scrypt(ORCA_AI_CREDENTIAL_KEY + accountId)
  const masterKey = process.env.ORCA_AI_CREDENTIAL_KEY
  if (!masterKey) throw new Error('ORCA_AI_CREDENTIAL_KEY not set')

  const derivedKey = await scrypt(masterKey + accountId, salt, { N: 16384, r: 8, p: 1, dkLen: 32 })

  // 2. Decrypt encrypted blob từ browser
  const decrypted = await aesgcmDecrypt(derivedKey, encryptedBlob, encryptionInfo.iv)
  const apiKey = new TextDecoder().decode(decrypted)

  // 3. Re-encrypt với server-side key (AES-256-GCM)
  const serverKey = deriveServerKey(masterKey, accountId)  // scrypt
  const serverEncrypted = aesgcmEncrypt(serverKey, apiKey)

  // 4. Write encrypted file
  const credPath = path.join(os.homedir(), '.orca', 'ai-providers', `${accountId}.enc`)
  await fs.mkdir(path.dirname(credPath), { recursive: true })
  await fs.writeFile(credPath, serverEncrypted, { mode: 0o600 })

  // apiKey đã được giải phóng khỏi memory sau bước này
}
```

---

## 3. Flow: Provider Resolution (Priority Cascade)

```
Agent spawn: cần API key cho userId + projectId + provider

    ▼ AIProviderResolver.resolve(userId, projectId, provider)
    │
    ├── Priority 1 (highest): User-scope account
    │   → SELECT * FROM orca_ai_provider_accounts
    │     WHERE scope='user' AND created_by=userId AND provider=provider
    │   → FOUND: use this accountId
    │
    ├── Priority 2: Project-scope account
    │   → SELECT * FROM orca_ai_provider_accounts
    │     WHERE scope='project' AND project_id=projectId AND provider=provider
    │   → FOUND: use this accountId
    │
    ├── Priority 3: Server-default account
    │   → SELECT * FROM orca_ai_provider_accounts
    │     WHERE scope='server-default' AND provider=provider
    │   → FOUND: use this accountId
    │
    └── Priority 4: No account → null
        → Agent uses system env (ANTHROPIC_API_KEY etc. if set on dev server)

    ▼ relay.call('ai.provider.readCredential', { accountId })
    │
    │ Dev Server: đọc ~/.orca/ai-providers/<accountId>.enc
    │             decrypt với server key
    │             trả về { apiKey }  (qua relay — encrypted channel)
    │
    ▼ Inject vào agent env:
      ANTHROPIC_API_KEY = apiKey  (chỉ tồn tại trong process env)
```

---

## 4. DB Schema (Migration 0008)

```sql
-- AI Provider Accounts metadata (NO credentials stored here)
CREATE TABLE orca_ai_provider_accounts (
  id           TEXT PRIMARY KEY,
  account_name TEXT NOT NULL,
  provider     TEXT NOT NULL,   -- anthropic | openai | google | azure | bedrock | ollama | vllm
  model        TEXT,            -- claude-opus-4-5 | gpt-4o | etc.
  dev_server_id TEXT REFERENCES ssh_hosts(id),   -- WHERE credential file lives
  scope        TEXT DEFAULT 'user',               -- user | project | server-default
  project_id   TEXT REFERENCES orca_projects(id),-- null if scope!=project
  created_by   TEXT REFERENCES orca_users(id),
  is_active    INTEGER DEFAULT 1,
  created_at   INTEGER,
  updated_at   INTEGER
);
CREATE INDEX idx_ai_provider_scope ON orca_ai_provider_accounts(scope, provider);
CREATE INDEX idx_ai_provider_user ON orca_ai_provider_accounts(created_by, provider);

-- Usage tracking (quota management)
CREATE TABLE orca_provider_usage (
  id          TEXT PRIMARY KEY,
  account_id  TEXT REFERENCES orca_ai_provider_accounts(id),
  user_id     TEXT REFERENCES orca_users(id),
  tokens_in   INTEGER DEFAULT 0,
  tokens_out  INTEGER DEFAULT 0,
  cost_usd    REAL DEFAULT 0,
  period      TEXT NOT NULL,    -- '2026-07' (YYYY-MM)
  updated_at  INTEGER
);
CREATE UNIQUE INDEX idx_usage_period ON orca_provider_usage(account_id, user_id, period);
```

---

## 5. Supported Providers

| Provider | ID | Auth Type | Credential File |
|---|---|---|---|
| Anthropic Claude | `anthropic` | API Key | `<id>.enc` |
| OpenAI | `openai` | API Key | `<id>.enc` |
| Google Gemini | `google` | API Key / OAuth | `<id>.enc` |
| Azure OpenAI | `azure` | API Key + Endpoint | `<id>.enc` |
| AWS Bedrock | `bedrock` | AWS credentials | `<id>.enc` |
| Ollama | `ollama` | Base URL (no key) | `<id>.enc` |
| vLLM | `vllm` | Base URL + API key | `<id>.enc` |

---

## 6. Health Check & Quota Tracking

```typescript
// src/main/ai-providers/provider-health-checker.ts
// Background cron mỗi 15 phút

class ProviderHealthChecker {
  async checkAll(): Promise<void> {
    const accounts = await db.query(
      'SELECT * FROM orca_ai_provider_accounts WHERE is_active = 1'
    )

    for (const account of accounts) {
      const relay = await RelayConnectionPool.getOrConnect(account.dev_server_id)
      const result = await relay.call('ai.provider.healthCheck', {
        accountId: account.id,
        provider: account.provider,
      })
      // result: { status: 'ok' | 'quota_exceeded' | 'invalid_key' | 'unreachable' }

      if (result.status === 'quota_exceeded') {
        await this.markAccountDegraded(account.id)
        await this.notifyAdmin(account)
      } else if (result.status === 'invalid_key') {
        await this.deactivateAccount(account.id)
        await this.auditLog('ai_provider.key_invalid', account)
      }
    }
  }
}
```

---

## 7. RPC Methods — aiProvider.*

```typescript
'aiProvider.list'           // () → AIProviderAccount[] (metadata only, no keys)
'aiProvider.register'       // (input) → { accountId } — ghi encrypted key qua relay
'aiProvider.update'         // (accountId, { name, model }) — metadata update
'aiProvider.delete'         // (accountId) — xóa metadata + delete file trên dev server
'aiProvider.setScope'       // (accountId, scope, projectId?) — change priority
'aiProvider.healthCheck'    // (accountId) → { status, lastChecked }
'aiProvider.getUsage'       // (accountId, period) → { tokensIn, tokensOut, costUsd }
'aiProvider.resolveForUser' // (userId, projectId, provider) → { accountId } (no key)
```

---

## 8. Security Summary

| Property | Cơ chế |
|---|---|
| **In-transit** | SubtleCrypto AES-GCM (browser) → relay SSH channel (encrypted) |
| **At-rest** | AES-256-GCM per-account key (scrypt KDF) on Dev Server |
| **Server visibility** | Orca Server chỉ thấy encrypted blob — KHÔNG có plaintext |
| **File permissions** | `~/.orca/ai-providers/*.enc` chmod 600 |
| **Key rotation** | Gọi `aiProvider.delete` + `aiProvider.register` lại |
| **Audit** | Tất cả register/delete/health events ghi vào `orca_audit_log` |

---

## 9. Cross-References

| Resource | Mô tả |
|---|---|
| [profile-resolution.md](./profile-resolution.md) | ProfileResolver quyết định provider priority |
| [relay-management.md](./relay-management.md) | SSH relay channel để relay credential blob |
| [task-agent-execution.md](./task-agent-execution.md) | Agent execution cần API key từ ProviderResolver |
| **HLD C1 Flow 8** | AI Provider Credential Write (Relay-Only) |
| **HLD C2 Container 14** | AI Provider Service |
| **HLD C4.9** | AIProviderService module detail |
| **F35** | Feature specification |
| **BL-AIP-01** | Register provider account business logic |
| **BL-AIP-02** | Provider resolution business logic |
