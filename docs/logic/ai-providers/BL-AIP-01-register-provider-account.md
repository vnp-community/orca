# BL-AIP-01 — Đăng ký AI Provider Account trên Dev Server

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-AIP-01 |
| **Tên** | Đăng ký AI Provider Account trên Dev Server |
| **Domain** | AI Provider Management |
| **Actor** | Admin, Lead |
| **Priority** | P0 |

---

## Mô tả

Admin/Lead đăng ký một AI Provider Account (Anthropic, OpenAI, Ollama...) trên một Dev Server cụ thể.  
Credentials được mã hóa và lưu **trực tiếp trên Dev Server** — không lưu plaintext trên Orca Server.

**Topology kết nối quan trọng:**  
Dev Server chủ động **mở WebSocket vào Orca Server** (WS client).  
Orca Server gửi JSON-RPC `ai.credential.write` **ngược lại qua WS đó** — không dùng SSH relay.

---

## Luồng chính

```
Admin → Settings → Dev Server → [server] → AI Providers → Add Account
    │
    ├── Input:
    │   - provider: 'anthropic'
    │   - name: "Team Anthropic Key"
    │   - scope: 'server' | 'project' | 'user'
    │   - projectId?: (nếu scope = 'project')
    │   - models: ['claude-opus-4-5', 'claude-sonnet']
    │   - isDefault: true
    │   - quotaLimitPerDay?: 2_000_000
    │
    ├── Nhập credentials (tùy provider):
    │   Anthropic/OpenAI/Google: apiKey
    │   Azure: apiKey + endpoint URL + deployment name
    │   AWS Bedrock: accessKeyId + secretAccessKey + region
    │   Ollama/vLLM: endpoint URL + optional Bearer token
    │
    ├── Step 1: Encrypt trong browser (SubtleCrypto)
    │   key = PBKDF2(sessionToken + userId, salt)
    │   encryptedBlob = AES-GCM(key, JSON.stringify(credentials))
    │   [credentials rời browser ở dạng encrypted blob]
    │
    ├── Step 2: Test Connection (trước khi save)
    │   POST /api/ai-providers/test
    │   Body: { provider, devServerId, credentialBlob }
    │   │
    │   Orca Server:
    │     conn = AgentConnectionManager.getConnection(devServerId)
    │     [WS Dev Server đã mở vào Orca]
    │     conn.call('ai.testConnection', { provider, credentialBlob, sessionKey })
    │     → Dev Server: decrypt blob → gọi test API (e.g. GET /v1/models)
    │     → result: { ok, latencyMs, modelsAvailable }
    │   Nếu fail → hiển thị error, không save
    │
    ├── Step 3: Save metadata tại Orca Server (không chứa credentials)
    │   INSERT orca_ai_provider_accounts (
    │     id, dev_server_id, provider, name, scope,
    │     project_id, user_id, is_default, models,
    │     quota_limit_per_day, created_by, created_at
    │   )  ← Server DB
    │
    ├── Step 4: Gửi credential đến Dev Server qua WebSocket
    │   Orca Server:
    │     conn = AgentConnectionManager.getConnection(devServerId)
    │     conn.call('ai.credential.write', {    ← JSON-RPC qua WS đó
    │       accountId,
    │       provider,
    │       credentials: <decrypted-in-memory>  ← tạm decrypt để relay
    │     })
    │   Dev Server nhận JSON-RPC:
    │     Verify RpcExecutionContext (HMAC-SHA256, 30s TTL)
    │     masterKey = scrypt(ORCA_AI_CREDENTIAL_KEY + ':' + accountId, accountId)
    │     stored = AES-256-GCM.encrypt(JSON.stringify(credentials), masterKey)
    │     ghi: ~/.orca/ai-providers/<accountId>.enc  (chmod 0600)
    │     trả: { ok: true }
    │
    ├── Step 5: Nếu isDefault = true
    │   UPDATE orca_ai_provider_accounts
    │     SET is_default = false
    │     WHERE dev_server_id = ? AND provider = ? AND id != newId
    │
    └── audit_log('ai_provider.registered', adminId, accountId, devServerId)
```

---

## Validation Rules

| Field | Rule |
|-------|------|
| provider | Must be in SUPPORTED_PROVIDERS list |
| name | Non-empty, max 100 chars, unique per (devServer, provider) |
| scope=project | projectId must exist and be on same devServer |
| scope=user | userId must exist |
| models | Must be subset of provider's supported models |
| apiKey (Anthropic) | Format: `sk-ant-api03-...` |
| apiKey (OpenAI) | Format: `sk-...` |
| endpoint (Ollama) | Valid URL, reachable from Dev Server |
| quotaLimitPerDay | >= 1000 nếu set |

---

## Credential Derivation (Dev Server side)

```
accountId = "acc-a1b2c3"
ORCA_AI_CREDENTIAL_KEY = env var (min 32 chars)

masterKey = scrypt(
  password: ORCA_AI_CREDENTIAL_KEY + ':' + accountId,
  salt: accountId,
  N: 16384, r: 8, p: 1, keylen: 32
)

iv = randomBytes(12)
{ ciphertext, authTag } = AES256GCM.encrypt(JSON.stringify(credentials), masterKey, iv)
stored = base64(iv) + '.' + base64(ciphertext) + '.' + base64(authTag)
```

---

## JSON-RPC Contract (Orca → Dev Server qua WS)

```typescript
// Test connection
conn.call('ai.testConnection', {
  provider: 'anthropic',
  credentialBlob: string,   // encrypted blob từ browser
  sessionKey: string,       // để Dev Server decrypt blob
})
// → { ok: boolean, latencyMs: number, modelsAvailable: string[] }

// Write credential
conn.call('ai.credential.write', {
  accountId: string,
  provider: 'anthropic',
  credentials: { apiKey: string }  // in-memory plaintext, chỉ tồn tại trong transit
})
// → { ok: true }
```

---

## Error Cases

| Lỗi | Xử lý |
|-----|-------|
| Test connection fail (invalid key) | Show error "Invalid API key", không save |
| Test connection fail (timeout) | Show warning "Server unreachable, check endpoint" |
| Dev Server WS chưa kết nối | Show error "Dev Server chưa kết nối — kiểm tra agent service" |
| Duplicate default account | Auto-demote old default khi set new default |
| Quota limit 0 | Reject: "Quota limit must be >= 1000 tokens/day" |
