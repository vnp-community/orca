# BUG-PI-001 [BACKEND]: `project-integration.md` — `WebCredentialStore` hỗ trợ `bitbucket|azure-devops|gitea|linear|jira` nhưng KHÔNG hỗ trợ `github|gitlab`

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-PI-001  
**Note:** project/GitProviderCredentialService.ts: GitHub/GitLab PAT via WebCredentialStore V2  

## Mức độ: 🔴 HIGH

## Tóm tắt

`src/main/credentials/web-credential-store.ts:13-18`:
```typescript
export type CredentialService =
  | 'bitbucket'
  | 'azure-devops'
  | 'gitea'
  | 'linear'
  | 'jira'
```

**`github` và `gitlab` KHÔNG có trong CredentialService type.**

Nhưng HLD BL-PI-01 mô tả:
```
[GitHubService.fetchIssues()]
    ├─ Load token: WebCredentialStore.get('github', userId)  ← sẽ TypeScript error
```

Và `project-integration.md` BL-PI-04:
```
Load GitHub token: WebCredentialStore.get('github', userId)  ← FAIL
```

`relay.call('github.pr.create', ...)` có trong dispatch (line 396). Nhưng token management thì không.

## Ảnh hưởng

1. `WebCredentialStore.get('github', userId)` sẽ fail TypeScript compilation
2. GitHub PR creation (BL-PI-04) không thể load GitHub token
3. GitLab integration cũng không thể store token
4. `relay.call('github.pr.create')` → Dev Server cần GitHub token nhưng không có mechanism lưu trữ

## Fix đề xuất

```typescript
export type CredentialService =
  | 'github'      // ← thêm
  | 'gitlab'      // ← thêm
  | 'bitbucket'
  | 'azure-devops'
  | 'gitea'
  | 'linear'
  | 'jira'
```

Và thêm UI Settings page cho GitHub/GitLab token management (thêm vào renderer).

## Files liên quan

- `src/main/credentials/web-credential-store.ts:13-18`: missing github/gitlab
- `src/relay/agent-rpc-dispatch.ts:396,407,418,429`: github/gitlab relay handlers tồn tại
- `docs/flows/logic/project-integration.md`: BL-PI-01, BL-PI-04 dùng github token
