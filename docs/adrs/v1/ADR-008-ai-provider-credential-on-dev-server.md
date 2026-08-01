# ADR-008 — AI Provider Credentials Stored on Dev Server (Not Orca Server)

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-008 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-28 |
| **HLD Ref** | C2.14 (AI Provider Service), C3.11 |
| **Code Ref** | Cần tạo: `src/main/ai-providers/` |
| **Feature Ref** | F35 |
| **Liên quan** | ADR-006 (WebCredentialStore) |

---

## Bối cảnh

AI Provider credentials (Anthropic API keys, OpenAI keys, AWS credentials...) cần được lưu an toàn và sử dụng khi spawn AI agents. Có 2 nơi có thể lưu:

1. **Orca Server** — tập trung, dễ quản lý nhưng Orca Server trở thành single point của tất cả AI keys
2. **Dev Server** — key nằm gần agent, nhưng cần relay credentials từ browser

**Security concern:** Nếu Orca Server bị compromise, **tất cả** AI keys của tất cả users bị lộ cùng lúc.

---

## Quyết định

**AI Provider credentials được lưu trực tiếp trên Dev Server — không đi qua Orca Server.**

### Flow: Write credential

```
Browser (Admin)
    │ Nhập API key
    │ Encrypt trong browser: SubtleCrypto.encrypt(AES-256-GCM, sessionKey, apiKey)
    │ Send encrypted blob qua HTTPS → Orca Server
    │
Orca Server
    │ Không decrypt! Relay encrypted blob qua SSH
    │ relay.call('ai.provider.writeCredential', { accountId, encryptedBlob })
    │
Dev Server (relay binary)
    │ Decrypt với ORCA_AI_CREDENTIAL_KEY (env var on dev server)
    │ Write: ~/.orca/ai-providers/<accountId>.enc
    │ (AES-256-GCM, key = scrypt(ORCA_AI_CREDENTIAL_KEY + ':' + accountId, accountId))
```

### Flow: Read credential (when spawning agent)

```
Orca Server → relay.call('ai.provider.readCredential', { accountId })
Dev Server → decrypt → inject vào agent env:
  ANTHROPIC_API_KEY=sk-ant-...
  OPENAI_API_KEY=sk-...
```

**Orca Server NEVER sees plaintext credentials.**

### Metadata (trên Orca Server DB)

```typescript
// orca_ai_provider_accounts table (chỉ metadata, không có key)
interface AIProviderAccount {
  id: string
  devServerId: string
  provider: 'anthropic' | 'openai' | 'gemini' | 'azure' | 'bedrock' | 'ollama' | 'vllm'
  scope: 'server' | 'project' | 'user'
  label: string
  model?: string              // default model
  baseUrl?: string            // for Ollama/vLLM
  status: 'active' | 'invalid' | 'quota_exceeded' | 'unreachable'
  lastHealthCheck: Date
  quotaUsedToday: number
  quotaLimitDay: number
}
```

### Key derivation on Dev Server

```
masterKey = scrypt(ORCA_AI_CREDENTIAL_KEY + ':' + accountId, accountId, 32)
encryptedFile: [IV: 16 bytes][AuthTag: 16 bytes][Ciphertext]
Path: ~/.orca/ai-providers/<accountId>.enc
```

### Priority Resolution

```typescript
async function resolveProvider(context: {
  devServerId: string
  projectId?: string
  userId?: string
  modelHint?: string
}): Promise<AIProviderAccount> {
  // 1. Explicit accountId → return directly
  // 2. User-scope accounts matching modelHint
  // 3. Project-scope accounts matching modelHint
  // 4. Server-default accounts matching modelHint
  // 5. Any active server-default
  throw new Error('No AI provider configured')
}
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **Credentials on Dev Server only** ✅ | Orca Server compromise không lộ AI keys; principle of least privilege |
| Credentials on Orca Server | Single point of failure; GDPR/compliance risk |
| Credentials in browser localStorage | Không persist; XSS risk |
| HashiCorp Vault | External dependency; complex setup |
| Environment variables only | Không quản lý được; không rotate được |

---

## Hậu quả

**Tích cực:**
- Orca Server không bao giờ thấy plaintext AI API keys
- Compromise Orca Server → không lộ AI credentials
- Keys nằm gần agent (on same server) → không cần network round-trip khi spawn

**Tiêu cực:**
- Relay layer cần hỗ trợ `ai.provider.writeCredential` và `readCredential` methods
- `ORCA_AI_CREDENTIAL_KEY` phải được set trên mỗi dev server
- Key rotation cần: write new key → 30s grace period (cả old + new valid) → remove old
- Browser cần SubtleCrypto để encrypt trước khi gửi

---

## Trạng thái Implementation

❌ Chưa implement (v5.0 proposed)  
🎯 relay: `ai.provider.writeCredential`, `readCredential`, `listAccounts`, `healthCheck`  
🎯 `src/main/ai-providers/AIProviderService.ts`  
🎯 `src/main/ai-providers/ProviderResolver.ts`  
🎯 Migration 0008 (`orca_ai_provider_accounts`, `orca_provider_usage`)
