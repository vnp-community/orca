# TDD-AG-09: AI Credential Relay (v5.0)

**Document:** TDD-AG-09 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** AI credential storage and retrieval on Dev Server (scrypt + AES-GCM)
**Feature:** F35
**ADR:** ADR-008
**HLD Ref:** C3.11a
**Backend TDD:** TDD-16

> **Status: ❌ TODO** — v5.0 proposed
> **CRITICAL CONSTRAINT**: Credentials phải lưu trên Dev Server. Orca Server chỉ lưu metadata.

---

## 1. Architecture

```
Frontend (Browser)
    │ SubtleCrypto encrypt(apiKey) → encryptedBlob+iv
    │
    ▼
Orca Server
    │ RPC: ai.provider.writeCredential({ accountId, encryptedBlob, iv })
    │ (Orca Server: không decrypt, không lưu, relay thẳng qua relay-bridge)
    │
    ▼
Dev Server Agent (agent.js)
    │ Handle 'ai.provider.writeCredential'
    │ → scrypt-derive key từ ORCA_AI_CREDENTIAL_KEY env var
    │ → AES-256-GCM encrypt(encryptedBlob) again (double-layer)
    │ → lưu vào ~/.orca/credentials/<accountId>.enc
    │
    └─ On 'ai.provider.readCredential':
       → decrypt file → return plaintext apiKey
       → used to inject into TOOL_ENV khi spawn claude_code
```

---

## 2. New RPC Methods

### ai.provider.writeCredential

```javascript
// Triggered từ Orca Server relay khi user saves AI provider account
case 'ai.provider.writeCredential':
  response = await handleAIWriteCredential(rpc);
  break;

async function handleAIWriteCredential(rpc) {
  const { accountId, encryptedBlob, iv, algorithm = 'aes-256-gcm' } = rpc.params;

  if (!accountId || !encryptedBlob || !iv) {
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32602, message: 'Missing required params' } };
  }

  const CRED_KEY = process.env.ORCA_AI_CREDENTIAL_KEY;
  if (!CRED_KEY) {
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32603, message: 'ORCA_AI_CREDENTIAL_KEY not set' } };
  }

  try {
    // 1. Derive key from ORCA_AI_CREDENTIAL_KEY using scrypt
    const salt = crypto.randomBytes(16);
    const key = crypto.scryptSync(CRED_KEY, salt, 32);  // 256-bit key

    // 2. Double-encrypt: encrypt the already-client-encrypted blob
    const iv2 = crypto.randomBytes(12);
    const cipher = crypto.createCipheriv('aes-256-gcm', key, iv2);
    const payload = JSON.stringify({ encryptedBlob, iv, algorithm });
    const encrypted = Buffer.concat([cipher.update(payload, 'utf8'), cipher.final()]);
    const authTag = cipher.getAuthTag();

    // 3. Store to ~/.orca/credentials/<accountId>.enc
    const credDir = path.join(os.homedir(), '.orca', 'credentials');
    fs.mkdirSync(credDir, { recursive: true, mode: 0o700 });
    const credFile = path.join(credDir, `${accountId}.enc`);

    const stored = {
      version: 1,
      salt: salt.toString('base64'),
      iv2: iv2.toString('base64'),
      authTag: authTag.toString('base64'),
      data: encrypted.toString('base64'),
    };
    fs.writeFileSync(credFile, JSON.stringify(stored), { mode: 0o600 });  // owner read-only

    log.info(`ai.provider.writeCredential: stored credential for accountId=${accountId}`);
    return { jsonrpc: '2.0', id: rpc.id, result: { ok: true } };
  } catch (err) {
    log.error(`ai.provider.writeCredential failed: ${err.message}`);
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32603, message: err.message } };
  }
}
```

### ai.provider.readCredential

```javascript
async function handleAIReadCredential(rpc) {
  const { accountId } = rpc.params;

  const CRED_KEY = process.env.ORCA_AI_CREDENTIAL_KEY;
  if (!CRED_KEY) {
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32603, message: 'ORCA_AI_CREDENTIAL_KEY not set' } };
  }

  try {
    const credFile = path.join(os.homedir(), '.orca', 'credentials', `${accountId}.enc`);
    if (!fs.existsSync(credFile)) {
      return { jsonrpc: '2.0', id: rpc.id, error: { code: -32601, message: `Credential not found: ${accountId}` } };
    }

    const stored = JSON.parse(fs.readFileSync(credFile, 'utf8'));
    const salt = Buffer.from(stored.salt, 'base64');
    const iv2 = Buffer.from(stored.iv2, 'base64');
    const authTag = Buffer.from(stored.authTag, 'base64');
    const data = Buffer.from(stored.data, 'base64');

    // Decrypt with ORCA_AI_CREDENTIAL_KEY
    const key = crypto.scryptSync(CRED_KEY, salt, 32);
    const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv2);
    decipher.setAuthTag(authTag);
    const decrypted = Buffer.concat([decipher.update(data), decipher.final()]);
    const payload = JSON.parse(decrypted.toString('utf8'));

    // payload.encryptedBlob + payload.iv: still client-encrypted
    // Return to Orca Server which returns to relay for agent spawn
    return {
      jsonrpc: '2.0', id: rpc.id,
      result: {
        accountId,
        encryptedBlob: payload.encryptedBlob,
        iv: payload.iv,
        algorithm: payload.algorithm,
      },
    };
  } catch (err) {
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32603, message: err.message } };
  }
}
```

### ai.provider.healthCheck

```javascript
async function handleAIHealthCheck(rpc) {
  const { accountId, provider, model } = rpc.params;

  // Read credential
  const credResult = await handleAIReadCredential({ params: { accountId }, id: null });
  if (credResult.error) return { ...credResult, id: rpc.id };

  // Minimal API test — small completion request
  const testPayloads = {
    anthropic: { model: model || 'claude-haiku-20241022', max_tokens: 10, messages: [{ role: 'user', content: 'hi' }] },
    openai:    { model: model || 'gpt-4o-mini', max_tokens: 10, messages: [{ role: 'user', content: 'hi' }] },
    gemini:    { contents: [{ parts: [{ text: 'hi' }] }], generationConfig: { maxOutputTokens: 10 } },
  };

  const start = Date.now();
  try {
    // Decrypt apiKey from credential blob (requires local scrypt key)
    const apiKey = await decryptApiKey(credResult.result.encryptedBlob, credResult.result.iv);
    await callProvider(provider, apiKey, testPayloads[provider]);
    return { jsonrpc: '2.0', id: rpc.id, result: { ok: true, latencyMs: Date.now() - start } };
  } catch (err) {
    return { jsonrpc: '2.0', id: rpc.id, result: { ok: false, latencyMs: Date.now() - start, error: err.message } };
  }
}
```

---

## 3. Credential File Structure

```
~/.orca/
└── credentials/
    ├── acct-abc123.enc        (mode: 0600 — owner read-only)
    ├── acct-xyz789.enc
    └── ...

File content (JSON):
{
  "version": 1,
  "salt": "<base64 16 bytes>",
  "iv2": "<base64 12 bytes>",
  "authTag": "<base64 16 bytes>",
  "data": "<base64 AES-256-GCM ciphertext>"
}
```

---

## 4. Inject Credential into claude_code spawn (v5.0)

```javascript
// Modified claude_code handler:
handler: async (params) => {
  const cwd  = params.cwd || WORK_DIR;
  const args = ['--print', params.prompt];
  if (params.model) args.unshift('--model', params.model);

  // v5.0: resolve API key from credential store
  let toolEnv = TOOL_ENV;
  if (params.accountId) {
    const apiKey = await resolveApiKey(params.accountId);
    if (apiKey) toolEnv = { ...TOOL_ENV, ANTHROPIC_API_KEY: apiKey };
  }

  return await runCommandCaptureWithEnv('claude', args, { cwd, timeout: 300000, env: toolEnv });
},
```

---

## 5. Key Rotation

```
Old key (ORCA_AI_CREDENTIAL_KEY=old):
  → read all .enc files, decrypt with old key
  → re-encrypt with new key
  → write back

Key rotation script (v5.0):
  ai.provider.rotateKey({ oldKey, newKey })
    → read all credentials
    → decrypt with oldKey
    → re-encrypt with newKey
    → atomic write (write to .enc.new, then rename)
```

---

## 6. New Environment Variables

```bash
# .env additions for v5.0:
ORCA_AI_CREDENTIAL_KEY=<256-bit random hex>  # scrypt KDF master key
# Generate: openssl rand -hex 32
```

---

## 7. Test Coverage

```
tests/unit/
├── ai-credential.test.js
│   ├── writeCredential: stores encrypted file with mode 0600
│   ├── writeCredential: file contains version=1, salt, iv2, authTag, data
│   ├── writeCredential: missing ORCA_AI_CREDENTIAL_KEY → error -32603
│   ├── readCredential: decrypts and returns encryptedBlob+iv
│   ├── readCredential: file not found → error -32601
│   ├── readCredential: wrong key → AES decryption error
│   └── writeCredential + readCredential: round-trip matches
└── ai-health-check.test.js
    ├── healthCheck: API call success → { ok: true, latencyMs }
    └── healthCheck: API call fails → { ok: false, error }
```

**Target:** ≥ 15 tests

---

## v2.1 Integration Note

**Source file:** `src/relay/agent-credential-store.ts` (không còn inline trong agent.js)

```typescript
// src/relay/agent-credential-store.ts
import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'
import { mkdirSync, writeFileSync, readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import type { AgentConfig } from './agent-config'

// Uses config.credentialDir = ~/.orca/credentials/
// Uses process.env.ORCA_AI_CREDENTIAL_KEY from AgentConfig (v5.0 extension)
```

**Test file:** `src/relay/__tests__/agent-credential-store.test.ts`
