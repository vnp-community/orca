# BL-INT-02: WebCredentialStore (API Token Management)

**Domain:** Remote Source Control Integrations  
**Priority:** P1  
**Actor chính:** Carlos, Alex  
**Tham chiếu:** FR-18.2, F30

---

## Mô tả

`WebCredentialStore` lưu API tokens cho các integrations HTTP-based (Bitbucket, Azure DevOps, Gitea, Linear, Jira) với per-user AES-256-GCM encryption. Thay thế global env vars.

## Supported Services

| Service | Auth Method | Token Scope |
|---------|------------|-------------|
| Bitbucket | App password | per-user |
| Azure DevOps | PAT | per-user |
| Gitea | API token | per-user |
| Linear | API key | per-user |
| Jira | Basic auth token | per-user |

## Encryption Scheme

```
Key derivation:
  masterKey = PBKDF2(ORCA_CREDENTIAL_KEY + userId, salt=userId, iterations=100000, sha256)
  
Per credential:
  iv = crypto.randomBytes(12)  // 12 bytes for GCM
  { ciphertext, authTag } = AES-256-GCM.encrypt(plaintext, masterKey, iv)
  stored = JSON.stringify({ iv: hex(iv), ct: hex(ciphertext), tag: hex(authTag) })
  
File: ~/.orca/users/<userId>/credentials.enc
  JSON object: { [service]: <encrypted_blob> }
```

## RPC API

```typescript
// credentials.set(service, token)
// scope: userId injected from session context
await rpc.call('credentials.set', { service: 'bitbucket', token: 'app-password-123' });

// credentials.get(service)
const token = await rpc.call('credentials.get', { service: 'bitbucket' });
// Returns plaintext token hoặc null nếu không có

// credentials.delete(service)
await rpc.call('credentials.delete', { service: 'bitbucket' });

// credentials.list() — returns service names only (no tokens)
const services = await rpc.call('credentials.list');
// Returns: ['bitbucket', 'linear']
```

## Frontend (CredentialInputForm)

```
Integration Settings → Bitbucket

  API Token: [●●●●●●●●●●] [Edit] [Test Connection] [Delete]

  Status: ✓ Connected (last verified 2h ago)
```

## Security Notes

- ORCA_CREDENTIAL_KEY phải được set (env hoặc Kubernetes secret)
- Nếu key thay đổi: tất cả credentials vô hiệu (cần re-enter)
- credentials.get() không log token
- credentials.list() không trả về token values
- Encryption file permissions: 0600

## Source References

- `src/main/integrations/web-credential-store.ts`
- `src/renderer/src/components/CredentialInputForm.tsx`
- `src/main/rpc/credentials-rpc.ts` — RPC handlers
