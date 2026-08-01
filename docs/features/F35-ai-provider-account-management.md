# F35 — AI Provider Account Management

| Trường | Giá trị |
|--------|---------|
| **ID** | F35 |
| **Tên** | AI Provider Account Management |
| **Ưu tiên** | P0 |
| **Trạng thái** | 🚧 Phát triển |
| **Phiên bản** | v5.0+ |
| **ADR References** | ADR-008 |
| **HLD References** | C2.14, C3.11 |

---

## Mô tả

Quản trị viên có thể đăng ký và cấu hình **nhiều AI Provider Accounts** (Anthropic, OpenAI, Google Gemini, Azure OpenAI, AWS Bedrock, Ollama...) trên từng Dev Server. Mỗi project/workflow/agent có thể chọn provider phù hợp. Credentials được lưu **trên Dev Server** (gần code, không qua Orca Server), mã hóa AES-256-GCM.

---

## Vấn đề cần giải quyết

- Agent chạy trên Dev Server, nhưng API keys hiện lấy từ `process.env` global → xung đột khi nhiều user/workflow dùng key khác nhau
- Không có cơ chế quản lý tập trung: admin phải SSH vào từng server để set key
- Không audit trail: ai dùng key nào, quota bao nhiêu
- Self-hosted models (Ollama, vLLM) cần endpoint URL riêng per server

---

## Providers hỗ trợ

| Provider | Models | Auth type |
|---------|--------|-----------|
| **Anthropic** | claude-opus-4-5, claude-sonnet, claude-haiku | API Key |
| **OpenAI** | gpt-4o, gpt-4-turbo, o1, o3 | API Key |
| **Google Gemini** | gemini-2.0-flash, gemini-pro | API Key / Service Account |
| **Azure OpenAI** | (deployment-based) | API Key + Endpoint URL |
| **AWS Bedrock** | claude, titan, llama | AWS Access Key + Secret |
| **Ollama** | llama3, mistral, codellama, qwen | Endpoint URL (no key) |
| **vLLM** | (any model) | Endpoint URL + optional Bearer token |
| **OpenCode** | (uses above) | via proxy |

---

## Tính năng chi tiết

### Provider Account Registry

```typescript
interface AIProviderAccount {
  id: string
  devServerId: string                // Account sống trên dev server này
  provider: 'anthropic' | 'openai' | 'google' | 'azure' | 'aws' | 'ollama' | 'vllm'
  name: string                       // "Team Anthropic Key", "Prod OpenAI"
  scope: 'server' | 'project' | 'user'   // visibility scope
  projectId?: string                 // nếu scope = 'project'
  userId?: string                    // nếu scope = 'user'
  isDefault: boolean                 // default account cho provider này
  models: string[]                   // allowed models với account này
  quotaLimitPerDay?: number          // token quota per day (optional)
  createdBy: string
  createdAt: number
  // credentials NOT stored here — stored on dev server
}
```

### Credential Storage (on Dev Server)

Credentials **không** đi qua Orca Server. Lưu trực tiếp trên Dev Server:

```
Dev Server: ~/.orca/ai-providers/<accountId>.enc
  Format: AES-256-GCM encrypted JSON
  {
    "apiKey": "sk-ant-...",
    "endpoint": "https://...",   // for Azure/Ollama/vLLM
    "region": "us-east-1",       // for AWS Bedrock
    "accessKeyId": "...",        // for AWS
    "secretAccessKey": "..."
  }

Encryption key: scrypt(ORCA_AI_CREDENTIAL_KEY + accountId, accountId)
```

**Flow setup credential:**
```
Admin nhập key trong UI
    → encrypted locally in browser (SubtleCrypto)
    → gửi qua WebSocket RPC (TLS)
    → Orca Server relay qua SSH
    → Dev Server write ~/.orca/ai-providers/<accountId>.enc
```

### Provider Account UI (Admin Panel)

```
Settings → Dev Servers → [server] → AI Providers
┌──────────────────────────────────────────────────────┐
│ AI Provider Accounts on dev-alpha.vnpblc.internal    │
│                                                      │
│ [+ Add Provider Account]                             │
│                                                      │
│ ● Anthropic                                          │
│   • "Team API Key" [DEFAULT] [server-scope]          │
│     Models: claude-opus-4-5, claude-sonnet           │
│     Quota: 2M tokens/day | Used: 847K                │
│     [Edit] [Test] [Rotate Key] [Delete]              │
│                                                      │
│ ● OpenAI                                             │
│   • "Project X OpenAI" [project-scope: vnp-blc]     │
│     Models: gpt-4o                                   │
│     [Edit] [Test] [Delete]                           │
│                                                      │
│ ● Ollama (local)                                     │
│   • "Local LLM" http://localhost:11434               │
│     Models: llama3:8b, codellama:13b                 │
│     [Test] [Delete]                                  │
└──────────────────────────────────────────────────────┘
```

### Provider Selection Priority

Khi agent/workflow cần provider, hệ thống resolve theo priority:

```
1. Explicit selection trong workflow/agent config (highest)
2. Project-scope account cho project đó
3. User-scope account (nếu scope=user và userId match)
4. Server-scope default account
5. Error: "No AI provider configured for <provider>" (lowest)
```

### Health Check & Validation

```typescript
interface ProviderHealthStatus {
  accountId: string
  provider: string
  status: 'healthy' | 'quota_exceeded' | 'invalid_key' | 'unreachable'
  latencyMs?: number
  quotaUsedToday?: number
  quotaLimitPerDay?: number
  lastCheckedAt: number
  errorMessage?: string
}
```

- Test connection khi add account (gửi minimal API call)
- Background health check mỗi 15 phút
- Alert khi quota > 80%
- Alert khi key invalid/expired

### Key Rotation

```
Admin → [Rotate Key] → nhập new key
    → encrypt → gửi relay → update file trên dev server
    → old key đánh dấu 'rotating' (30s grace period)
    → invalidate all cached connections
    → audit_log('provider.key_rotated', adminId, accountId)
```

---

## Luồng người dùng

```
1. Admin vào Settings → Dev Server → [server] → AI Providers
2. Click "Add Provider Account"
3. Chọn provider (Anthropic), nhập name, scope (server/project/user)
4. Nhập API key → UI test connection → success → save
5. Key được encrypt + relay đến dev server
6. Account xuất hiện trong danh sách với status "healthy"
7. Agent/workflow chọn account này khi spawn
```

---

## Tiêu chí chấp nhận

- [ ] CRUD AI Provider Accounts (linked to dev server)
- [ ] 7+ providers: Anthropic, OpenAI, Google, Azure, AWS, Ollama, vLLM
- [ ] Credentials stored on Dev Server (AES-256-GCM), NOT Orca Server
- [ ] 3 scopes: server / project / user
- [ ] Provider selection priority resolution
- [ ] Test connection on add
- [ ] Background health check mỗi 15 phút
- [ ] Quota tracking + 80% alert
- [ ] Key rotation với grace period
- [ ] Audit log: add/update/delete/rotate
- [ ] Admin Panel UI với provider list + status badges

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Provider types | `src/shared/ai-provider-types.ts` |
| Provider service | `src/main/ai-providers/provider-service.ts` |
| Provider health checker | `src/main/ai-providers/provider-health-checker.ts` |
| Credential relay | `src/main/ai-providers/provider-credential-relay.ts` |
| Provider resolver | `src/main/ai-providers/provider-resolver.ts` |
| DB migration | `src/main/db/migrations/0008_ai_providers.ts` |
| RPC methods | `src/main/runtime/rpc/methods/ai-providers.ts` |
| Provider UI | `src/renderer/src/components/ai-providers/AIProviderPanel.tsx` |
| Add Provider dialog | `src/renderer/src/components/ai-providers/AddProviderDialog.tsx` |

**Env on Dev Server:** `ORCA_AI_CREDENTIAL_KEY` (min 32 chars)

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| Provider credential write to dev server | < 2s |
| Provider selection resolution | < 1ms |
| Health check latency | < 500ms per provider |
| Key rotation downtime | < 30s (grace period) |
