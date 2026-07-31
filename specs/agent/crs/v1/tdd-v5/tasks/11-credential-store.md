# TASK-11: Create src/relay/agent-credential-store.ts

**Phase:** 5 (v5.0 extensions)  
**SOL Ref:** SOL-10  
**Estimated time:** 3h  
**Precondition:** TASK-03 (agent-config) hoàn thành  

---

## Tạo file mới: `src/relay/agent-credential-store.ts`

### Imports

```typescript
import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'
import { mkdirSync, writeFileSync, readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
```

### Constants

```typescript
const ALGORITHM = 'aes-256-gcm'
const SCRYPT_KEY_LEN = 32     // 256-bit key
const SALT_BYTES = 16
const IV_BYTES = 12
const FILE_VERSION = 1
```

### StoredCredential interface

```typescript
interface StoredCredential {
  version: number
  salt: string     // base64 16 bytes
  iv2: string      // base64 12 bytes (server-side IV)
  authTag: string  // base64 16 bytes (GCM auth tag)
  data: string     // base64 AES-256-GCM ciphertext
  // Inside data (plaintext before outer encryption):
  // JSON: { encryptedBlob: string, iv: string, algorithm: string }
}
```

### getCredentialKey() — private helper

```typescript
function getCredentialKey(): string
// Returns process.env.ORCA_AI_CREDENTIAL_KEY
// Throws with code AgentErrorCode.PermissionDenied if not set or empty
```

### credentialFilePath() — private helper

```typescript
function credentialFilePath(credentialDir: string, accountId: string): string
// Validates accountId: /^[\w-]+$/ — throws InvalidParams on invalid
// Returns: join(credentialDir, `${accountId}.enc`)
```

### encryptPayload() / decryptPayload() — private helpers

```typescript
function encryptPayload(masterKey: string, payload: string): Omit<StoredCredential, 'version'>
// Uses scryptSync(masterKey, randomSalt, SCRYPT_KEY_LEN) for key derivation
// Uses createCipheriv('aes-256-gcm', key, randomIv)
// Returns { salt, iv2, authTag, data } all base64

function decryptPayload(masterKey: string, stored: StoredCredential): string
// Reverse of encryptPayload
// Throws if auth tag fails (wrong key or tampered data)
```

### 3 exported handler functions

```typescript
export async function handleWriteCredential(
  id: string | number | null,
  params: Record<string, unknown>,  // { accountId, encryptedBlob, iv, algorithm? }
  config: AgentConfig,
  log: AgentLogger
): Promise<object>
// 1. Validate params (accountId, encryptedBlob, iv required)
// 2. getCredentialKey()
// 3. encryptPayload(masterKey, JSON.stringify({ encryptedBlob, iv, algorithm }))
// 4. mkdirSync(credentialDir, { mode: 0o700, recursive: true })
// 5. writeFileSync(filePath, JSON.stringify(stored), { mode: 0o600 })
// 6. Return { jsonrpc, id, result: { ok: true } }

export async function handleReadCredential(
  id: string | number | null,
  params: Record<string, unknown>,  // { accountId }
  config: AgentConfig,
  log: AgentLogger
): Promise<object>
// 1. Check existsSync(filePath) → PathNotFound if missing
// 2. decryptPayload() → parse JSON
// 3. Return { jsonrpc, id, result: { accountId, encryptedBlob, iv, algorithm } }

export async function handleHealthCheck(
  id: string | number | null,
  params: Record<string, unknown>,  // { accountId, provider? }
  config: AgentConfig,
  log: AgentLogger
): Promise<object>
// Call handleReadCredential, if ok → { ok: true, latencyMs, note: 'credential_readable' }
```

---

## Security Checklist

- [x] `credentialDir` created với mode `0o700` (only owner can list)
- [x] `.enc` file created với mode `0o600` (only owner can read)
- [x] `accountId` validation: `^[\w-]+$` — ngăn path traversal `../evil`
- [x] `ORCA_AI_CREDENTIAL_KEY` không có → error response (không crash, không log key)
- [x] Decryption failure (wrong key/tampered) → error response (không crash)

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-credential" || echo "No errors"
```

## Definition of Done

- [x] `src/relay/agent-credential-store.ts` created
- [x] `handleWriteCredential`, `handleReadCredential`, `handleHealthCheck` exported
- [x] `scryptSync` + AES-256-GCM: không dùng deprecated `createCipher`
- [x] File permissions enforced: dir=0o700, file=0o600
- [x] Path traversal blocked
- [x] `pnpm run typecheck:node` passes
