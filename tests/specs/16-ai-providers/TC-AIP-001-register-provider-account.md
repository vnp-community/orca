# TC-AIP-001 — Đăng ký AI Provider Account

**BL Reference:** BL-AIP-01  
**Flow Reference:** docs/flows/logic/ai-providers.md  
**Priority:** P0  
**Type:** Integration + Security  
**Actor:** Admin, Lead

---

## TC-AIP-001-01: Đăng ký Anthropic provider — server scope

**Priority:** P0

### Steps
1. Admin: `POST /api/ai-providers/register` với:
   ```json
   { "provider": "anthropic", "scope": "server", "apiKey": "sk-ant-...", "devServerId": "srv-1" }
   ```

### Expected Results
- API key encrypted với AES-256-GCM và lưu trên Dev Server tại `~/.orca/ai-providers/<id>.enc`
- NOT stored on Orca Server
- DB: metadata record (id, provider, scope, devServerId) — NOT the key
- `provider.status === 'active'`

### Assertions
```
await rpc.call('aiProvider.register', { provider: 'anthropic', scope: 'server', apiKey: 'sk-ant-...' })

// Verify key NOT on Orca Server DB
assert db.aiProviders.find({ provider: 'anthropic' }).apiKey === undefined

// Verify key encrypted on Dev Server
keyFile = relayFs.read(`~/.orca/ai-providers/${id}.enc`)
assert keyFile !== null
assert !keyFile.includes('sk-ant-') // not plaintext
```

---

## TC-AIP-001-02: Đăng ký OpenAI provider — project scope

**Priority:** P0

### Steps
1. `aiProvider.register { provider: 'openai', scope: 'project', projectId, apiKey: 'sk-openai-...' }`

### Expected Results
- Key stored trên Dev Server với scope tag = project
- Only accessible for this project

---

## TC-AIP-001-03: Đăng ký provider — user scope

**Priority:** P0

### Steps
1. User: `aiProvider.register { provider: 'google', scope: 'user', apiKey: 'AIza...' }`

### Expected Results
- Key stored tại user scope
- Only this user can use

---

## TC-AIP-001-04: Providers supported

**Priority:** P1

### Steps
1. Test register cho mỗi supported provider

| Provider | Type |
|---------|------|
| anthropic | API Key |
| openai | API Key |
| google | API Key |
| azure | Connection String |
| aws-bedrock | Access Key + Secret |
| ollama | URL + no auth |
| vllm | URL + API Key optional |

---

## TC-AIP-001-05: AES-256-GCM encryption verification

**Priority:** P0  
**Security:** CRITICAL

### Steps
1. Register provider với apiKey='sk-test-abc'
2. Read `.enc` file từ Dev Server

### Expected Results
- `.enc` file content không chứa 'sk-test-abc' (not plaintext)
- File có IV (12 bytes) + ciphertext + authTag

### Assertions
```
encFile = relayFs.read(`~/.orca/ai-providers/${id}.enc`)
assert !encFile.toString('utf8').includes('sk-test-abc')
// Verify it's encrypted (not readable)
```

---

## TC-AIP-001-06: Test connection — Valid credentials

**Priority:** P1

### Steps
1. Register provider
2. `aiProvider.testConnection { id }`

### Expected Results
- HTTP call to provider API (e.g., Anthropic /v1/models)
- Response: `{ status: 'connected', latency: 150 }`

---

## TC-AIP-001-07: Test connection — Invalid API key

**Priority:** P1

### Steps
1. Register với apiKey='invalid-key'
2. `aiProvider.testConnection { id }`

### Expected Results
- Response: `{ status: 'error', code: 'AUTH_FAILED' }`

