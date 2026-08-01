# SOLUTION: Project Integration Domain — Fix tất cả Bugs

**Domain:** project-integration  
**TDD Reference:** TDD-05 (SSH Relay), TDD-15 (Project Binding), TDD-20 (Remote Git UI)  
**Files cần thay đổi:** `src/main/project/CredentialService.ts`, `src/main/auth/web-credential-store.ts`  
**Tổng số bugs:** 1 (PI-001)

---

## BUG-PI-001 — Fix CredentialService thiếu GitHub credential support

**Mức độ:** 🟠 HIGH  
**Root cause:** `CredentialService` chỉ support generic credentials, không có GitHub-specific flow (OAuth, PAT).

### Fix — Thêm GitHub (và GitLab) credential support

```typescript
// src/main/project/CredentialService.ts

export type CredentialType = 'github-pat' | 'github-oauth' | 'gitlab-pat' | 'gitlab-oauth' | 'generic'

export interface WebCredential {
  id:           string
  userId:       string
  type:         CredentialType
  provider:     'github' | 'gitlab' | 'other'
  label:        string       // e.g. "My GitHub Account"
  username?:    string       // GitHub username
  encryptedToken: string     // AES-256-GCM encrypted token
  scopes?:      string[]     // OAuth scopes granted
  expiresAt?:   number       // token expiry (ms timestamp)
  createdAt:    number
  updatedAt:    number
}

export class CredentialService {
  constructor(
    private readonly repository: IWebCredentialRepository,
    private readonly devServerManager: DevServerManager,
    private readonly encryptionKey: string,  // from ORCA_CREDENTIAL_KEY env
    private readonly log: Logger,
  ) {}

  /**
   * Store GitHub PAT (Personal Access Token).
   * Token được encrypt trên Orca Server trước khi lưu.
   */
  async storeGitHubPat(params: {
    userId:   string
    label:    string
    token:    string  // plaintext PAT — chỉ tồn tại trong memory ngắn
    username?: string
  }): Promise<WebCredential> {
    // Validate token format (GitHub PATs bắt đầu bằng 'ghp_' hoặc 'github_pat_')
    if (!params.token.startsWith('ghp_') && !params.token.startsWith('github_pat_')) {
      throw new Error('Invalid GitHub PAT format')
    }

    // Verify token (test API call)
    await this.verifyGitHubToken(params.token, params.username)

    // Encrypt
    const encryptedToken = await this.encryptToken(params.token)

    const credential: WebCredential = {
      id:             generateId(),
      userId:         params.userId,
      type:           'github-pat',
      provider:       'github',
      label:          params.label,
      username:       params.username,
      encryptedToken,
      createdAt:      Date.now(),
      updatedAt:      Date.now(),
    }

    await this.repository.create(credential)
    this.log.info(`[Credentials] GitHub PAT stored: ${credential.id}`)
    return credential
  }

  /**
   * Relay GitHub token đến Dev Server (cho gh CLI auth).
   * Dev Server lưu token trong GH_CONFIG_DIR/per-user.
   */
  async relayGitHubCredential(
    credentialId: string,
    devServerId:  string,
    userId:       string,
  ): Promise<void> {
    const cred = await this.repository.findById(credentialId)
    if (!cred || cred.userId !== userId) throw new Error('Credential not found')
    if (cred.provider !== 'github') throw new Error('Not a GitHub credential')

    // Decrypt
    const plaintext = await this.decryptToken(cred.encryptedToken)

    // Relay đến Dev Server
    const bridge = this.devServerManager.getBridge(devServerId)
    if (!bridge) throw new Error(`Dev server not connected: ${devServerId}`)

    await bridge.call('github.setToken', {
      userId,           // Dev Server isolates per userId
      token: plaintext,
      username: cred.username,
    })

    this.log.info(`[Credentials] GitHub token relayed to ${devServerId} for user ${userId}`)
  }

  /**
   * Store GitLab PAT hoặc OAuth token.
   */
  async storeGitLabPat(params: {
    userId:    string
    label:     string
    token:     string
    serverUrl?: string  // default: gitlab.com
  }): Promise<WebCredential> {
    const encryptedToken = await this.encryptToken(params.token)
    const credential: WebCredential = {
      id:             generateId(),
      userId:         params.userId,
      type:           'gitlab-pat',
      provider:       'gitlab',
      label:          params.label,
      encryptedToken,
      createdAt:      Date.now(),
      updatedAt:      Date.now(),
    }
    await this.repository.create(credential)
    return credential
  }

  async listCredentials(userId: string): Promise<Omit<WebCredential, 'encryptedToken'>[]> {
    const creds = await this.repository.listByUser(userId)
    // Never return encrypted tokens in list
    return creds.map(({ encryptedToken: _, ...rest }) => rest)
  }

  async deleteCredential(credentialId: string, userId: string): Promise<void> {
    const cred = await this.repository.findById(credentialId)
    if (!cred || cred.userId !== userId) throw new Error('Credential not found')
    await this.repository.delete(credentialId)
  }

  private async verifyGitHubToken(token: string, username?: string): Promise<void> {
    const response = await fetch('https://api.github.com/user', {
      headers: { Authorization: `Bearer ${token}`, 'User-Agent': 'Orca-Dev' },
      signal: AbortSignal.timeout(5000),
    })
    if (!response.ok) throw new Error(`GitHub token invalid: ${response.status}`)

    if (username) {
      const user = await response.json()
      if (user.login !== username) throw new Error(`Token does not match username: ${username}`)
    }
  }

  private async encryptToken(plaintext: string): Promise<string> {
    const { createCipheriv, randomBytes } = await import('node:crypto')
    const key = Buffer.from(this.encryptionKey, 'hex').slice(0, 32)
    const iv = randomBytes(12)
    const cipher = createCipheriv('aes-256-gcm', key, iv)
    const encrypted = Buffer.concat([cipher.update(plaintext, 'utf-8'), cipher.final()])
    const tag = cipher.getAuthTag()
    return Buffer.concat([iv, tag, encrypted]).toString('base64')
  }

  private async decryptToken(encryptedBase64: string): Promise<string> {
    const { createDecipheriv } = await import('node:crypto')
    const buf = Buffer.from(encryptedBase64, 'base64')
    const iv  = buf.slice(0, 12)
    const tag = buf.slice(12, 28)
    const enc = buf.slice(28)
    const key = Buffer.from(this.encryptionKey, 'hex').slice(0, 32)
    const decipher = createDecipheriv('aes-256-gcm', key, iv)
    decipher.setAuthTag(tag)
    return Buffer.concat([decipher.update(enc), decipher.final()]).toString('utf-8')
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/project/CredentialService.ts` | Add GitHub/GitLab credential methods | PI-001 |
| `src/main/repositories/web-credential-repository.ts` | NEW — repository interface + SQL impl | PI-001 |
| `src/main/db/migrations/0013_web_credentials.ts` | NEW migration | PI-001 |
| `src/main/ipc/credential-ipc.ts` | Wire GitHub/GitLab credential handlers | PI-001 |

---

## Verification Plan

```bash
pnpm vitest run src/main/project/__tests__/credential-service.test.ts

# Security tests:
# 1. Store GitHub PAT → verify token encrypted at rest
# 2. List credentials → verify encryptedToken NOT returned
# 3. Relay to Dev Server → verify plaintext reaches Dev Server (not ciphertext)
# 4. Invalid PAT format → verify rejected before storage
# 5. PAT for wrong username → verify verification fails
```
