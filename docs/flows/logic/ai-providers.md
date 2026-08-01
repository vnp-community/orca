# Luồng Dữ liệu — AI Provider Management

**Domain:** AI Provider Management  
**Nghiệp vụ:** BL-AIP-01 → BL-AIP-03  
**Kiến trúc tham chiếu:** HLD v1 — AI Provider Service (C3.11), C4.9, ADR-008, F35

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Admin/Lead Browser | UI | Provider registration form |
| Orca Web Server | Backend | /api/ai-providers REST API |
| AIProviderService | Business Logic | CRUD metadata (không lưu credential) |
| ProviderCredentialWriter | Security | Gửi credential qua WS → Dev Server ghi file |
| SubtleCrypto (Browser) | Client-side Crypto | AES-GCM encrypt API key trong browser |
| AgentConnectionManager | Backend | Pool WebSocket connection Dev Server đã mở |
| Dev Server Agent | Remote | Nhận JSON-RPC → decrypt → ghi `.enc` file |
| AIProviderResolver | Business Logic | Priority cascade resolution |
| ProviderHealthChecker | Background | Cron 15min — health check qua WS JSON-RPC |
| Server Database | Persistence | orca_ai_provider_accounts, orca_provider_usage |

> **Topology kết nối:**  
> Dev Server chủ động **mở WebSocket vào Orca Server** (`ws://orca:6768/agent`).  
> Orca Server gửi JSON-RPC request **ngược lại qua kết nối đó** khi cần viết credential hoặc health check.  
> Không dùng SSH relay.

---

## BL-AIP-01 — Đăng ký AI Provider Account trên Dev Server

```
Admin/Lead
    │
    ▼
[Admin SPA] AI Providers → "Add Provider Account"
    Input: {
      name: "Company Anthropic Account",
      provider: "anthropic",
      devServerId: "dev-01",
      priority: 1,
      scope: "server",
      apiKey: "sk-ant-xxx"    ← nhập trong browser, KHÔNG gửi plaintext
    }
    │
    ▼
STEP 1 — Encrypt trong browser (SubtleCrypto):
    [Browser JS]
    ├─ Derive session key: PBKDF2(sessionToken + userId, salt)
    ├─ AES-GCM encrypt: encryptedBlob = AES-GCM(sessionKey, apiKey)
    └─ POST /api/ai-providers/accounts
       Body: { name, provider, devServerId, priority, scope,
               credentialBlob: base64(encryptedBlob) }
       [apiKey KHÔNG có trong request body]
    │
    ▼
STEP 2 — Lưu metadata tại Orca Server:
    [AIProviderService.create()]
    ├─ requireAdmin/requireLead() guard
    ├─ INSERT orca_ai_provider_accounts {
    │     id, name, provider, devServerId, priority, scope,
    │     status: 'healthy', created_by, created_at
    │   }  ← Server DB  [KHÔNG lưu credential]
    └─ Trigger: ProviderCredentialWriter.write(accountId, devServerId, credentialBlob)
    │
    ▼
STEP 3 — Gửi credential đến Dev Server qua WebSocket:
    [ProviderCredentialWriter.write()]
    ├─ Lấy WS connection Dev Server đã mở:
    │     conn = AgentConnectionManager.getConnection(devServerId)
    ├─ Decrypt session layer (server-side): lấy lại plaintext apiKey
    │     key = deriveServerKey(sessionToken, userId)
    │     apiKey = AES-GCM-decrypt(key, credentialBlob)
    ├─ Gửi JSON-RPC qua WS:
    │     conn.call('ai.credential.write', {      ← WS Dev Server đã mở
    │       accountId,
    │       provider,
    │       credentials: { apiKey }               ← plaintext đến đây
    │     })
    │
    ▼
[Dev Server Agent — nhận JSON-RPC]
    ├─ Verify RpcExecutionContext (HMAC-SHA256, 30s TTL)
    ├─ Derive encryption key:
    │     masterKey = scrypt(ORCA_AI_CREDENTIAL_KEY + ':' + accountId, accountId)
    ├─ AES-256-GCM encrypt:
    │     stored = base64(iv) + '.' + base64(ciphertext) + '.' + base64(authTag)
    ├─ Ghi file: ~/.orca/ai-providers/<accountId>.enc
    └─ Trả JSON-RPC result: { ok: true }

Luồng:
Browser → SubtleCrypto.encrypt(apiKey) → POST (encrypted blob only)
Orca Server → INSERT metadata → ProviderCredentialWriter
            → AgentConnectionManager.getConnection(devServerId)
              [WS Dev Server đã mở vào Orca]
            → JSON-RPC: ai.credential.write → Dev Server
            → Dev Server: AES-256-GCM encrypt → ghi .enc file
            → JSON-RPC result: { ok: true }

Security: Orca Server decrypt tạm để relay, plaintext apiKey chỉ tồn tại
          in-memory trong thời gian transit qua WS (không persist)
```

---

## BL-AIP-02 — Provider Account Resolution cho Agent/Workflow

```
[AIProviderResolver.resolve(userId, projectId, devServerId, providerSpec)]
    → được gọi bởi AgentManager (BL-AG-01) hoặc WorkflowOrchestrator
    │
    ▼
[Priority Cascade — query Server DB]:

    1. Explicit accountId trong providerSpec
       → trả thẳng account đó (validate còn healthy)

    2. Model-based auto-detect:
       detectProviderFromModel(model)
       'claude-opus-4-5' → 'anthropic'
       'gpt-4o'          → 'openai'

    3. Cascade query (một lần, ORDER BY priority):
       SELECT FROM orca_ai_provider_accounts
       WHERE dev_server_id = ?
         AND provider      = ?
         AND status        = 'healthy'
       ORDER BY
         CASE WHEN scope='user'    AND user_id=?    THEN 0 END,
         CASE WHEN scope='project' AND project_id=? THEN 1 END,
         CASE WHEN scope='server'  AND is_default=1 THEN 2 END,
         CASE WHEN scope='server'                   THEN 3 END
       LIMIT 1

    4. Không tìm thấy → error: NoProviderAvailable
    │
    ▼
[Return ProviderConfig]:
    { accountId, provider, devServerId }
    [Không trả plaintext key — key đọc từ .enc file trên Dev Server khi spawn]

Luồng:
Caller → AIProviderResolver.resolve()
       → Server DB: cascade SELECT
       → Return ProviderConfig (accountId + devServerId)
       [agent.spawn sẽ truyền accountId, Dev Server tự đọc .enc file]
```

---

## BL-AIP-03 — Provider Health Check & Quota Management

```
[ProviderHealthChecker] cron mỗi 15 phút
    │
    ▼
FOR each account trong orca_ai_provider_accounts WHERE status IN ('healthy','degraded'):
    ├─ Lấy WS connection:
    │     conn = AgentConnectionManager.getConnection(account.devServerId)
    │
    ├─ Gửi JSON-RPC: conn.call('ai.ping', {     ← WS Dev Server đã mở
    │     accountId: account.id,
    │     provider: account.provider
    │   })
    │     → Dev Server: đọc .enc file → decrypt apiKey
    │                 → gọi test API (e.g. GET /v1/models)
    │                 → trả { latencyMs, ok }
    │
    ├─ IF ok: UPDATE status='healthy', latencyMs, lastCheckedAt  ← Server DB
    ├─ IF error:
    │     classify: quota_exceeded | invalid_key | unreachable
    │     UPDATE status=<error>                                   ← Server DB
    │     sendAlert(account, status) [Webhook + WS push]
    │
    └─ IF conn == null (Dev Server chưa kết nối):
          UPDATE status='unreachable', lastCheckedAt              ← Server DB
    │
    ▼
[Admin dashboard]
    GET /api/ai-providers/health
    → SELECT accounts + health_status + latencyMs + quota_json
    ← Response: table với health indicators

[Quota tracking — sau mỗi agent/workflow hoàn thành]:
    recordTokenUsage(accountId, tokensUsed)
    → UPSERT orca_provider_usage(account_id, date, tokens_used)  ← DB
    → IF usage > 80% quotaLimit: sendAlert('quota_warning_80pct')
    → IF usage >= quotaLimit:
         UPDATE status='quota_exceeded'
         sendAlert('quota_exceeded')
         [AIProviderResolver sẽ skip account này → next in cascade]

Luồng:
Cron (15min) → ProviderHealthChecker
             → AgentConnectionManager.getConnection(devServerId)
               [WS Dev Server đã mở — không SSH]
             → JSON-RPC: ai.ping × N accounts (parallel per server)
             → Dev Server: decrypt .enc → test API call
             → JSON-RPC result → Server DB (UPDATE health)
             → Alert webhook/WS nếu lỗi
```

---

## Sơ đồ tổng quan — AI Provider Management

```
┌──────────────┐  HTTP  ┌────────────────────────────────────────────────┐
│  Admin/Lead  │───────►│  Orca Server                                   │
│  Browser     │        │  AIProviderService (metadata CRUD)             │
│  SubtleCrypto│        │  ProviderCredentialWriter                      │
│  (encrypt)   │        │  AIProviderResolver (priority cascade)         │
└──────────────┘        │  ProviderHealthChecker (cron 15min)            │
                        │  AgentConnectionManager                        │
                        └──────────┬─────────────────────────────────────┘
                                   │
                        ┌──────────▼─────────────────────────────────────┐
                        │  Server Database                                │
                        │  orca_ai_provider_accounts (metadata only)     │
                        │  orca_provider_usage (quota logs)              │
                        └──────────┬─────────────────────────────────────┘
                                   │
                                   │ WebSocket JSON-RPC
                                   │ (Dev Server đã mở WS vào đây)
                                   │ conn.call('ai.credential.write' / 'ai.ping')
                                   ▼
                        ┌─────────────────────────────────────────────────┐
                        │  Dev Server (172.20.2.31)                        │
                        │  Dev Server Agent (WS client → Orca Server)     │
                        │                                                  │
                        │  ai.credential.write:                           │
                        │    AES-256-GCM encrypt → ~/.orca/ai-providers/  │
                        │    <accountId>.enc                              │
                        │                                                  │
                        │  ai.ping:                                        │
                        │    đọc .enc → decrypt apiKey (in-memory)        │
                        │    → test API call → trả { latencyMs, ok }      │
                        │                                                  │
                        │  agent.spawn (BL-AG-01):                        │
                        │    đọc .enc → inject apiKey vào PTY env         │
                        └─────────────────────────────────────────────────┘

Chiều kết nối:
  Dev Server ──WS connect──► Orca Server    (Dev Server = WS client)
  Orca Server ──JSON-RPC──► Dev Server      (request ngược qua WS đó)
  Dev Server ──JSON-RPC──► Orca Server      (result / stream events)

Security model:
  Browser ─ SubtleCrypto encrypt ─ POST ─ Orca Server decrypt tạm (in-memory)
                                         ─ JSON-RPC ─ Dev Server (encrypt + store)
  Plaintext apiKey: tồn tại in-memory tại Orca Server trong thời gian transit
  Không persist plaintext tại Orca Server
  Không dùng SSH relay
```
