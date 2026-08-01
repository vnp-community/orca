# TASK-PI-001: Thêm GitHub/GitLab PAT support vào CredentialService

**Priority:** 🟠 HIGH — GitHub/GitLab integration không có PAT authentication  
**Effort:** ~30 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-PI-001  
**Solution ref:** [SOLUTION-project-integration.md](../solutions/SOLUTION-project-integration.md)

## Bước 1 — Tìm CredentialService

```bash
grep -rn "CredentialService\|credential" src/main/project/ --include="*.ts" | grep -v test | head -10
```

## Bước 2 — Thêm GitHub/GitLab providers

```typescript
// src/main/project/CredentialService.ts (hoặc file tương đương)
export class CredentialService {
  // ... existing methods

  async setGitHubPAT(userId: string, token: string): Promise<void> {
    await this.store.set(`github:pat:${userId}`, this.encrypt(token))
  }

  async getGitHubPAT(userId: string): Promise<string | null> {
    const encrypted = await this.store.get(`github:pat:${userId}`)
    return encrypted ? this.decrypt(encrypted) : null
  }

  async setGitLabPAT(userId: string, projectId: string, token: string): Promise<void> {
    await this.store.set(`gitlab:pat:${userId}:${projectId}`, this.encrypt(token))
  }

  async getGitLabPAT(userId: string, projectId: string): Promise<string | null> {
    const encrypted = await this.store.get(`gitlab:pat:${userId}:${projectId}`)
    return encrypted ? this.decrypt(encrypted) : null
  }
}
```

## Verification

```bash
pnpm tsc --noEmit
```
