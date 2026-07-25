# TASK-019: Sửa `src/shared/dev-server-types.ts` — Thêm `RemotePreflightStatus`

**Phase:** 2 — Remote Preflight  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §B.1  
**Depends on:** TASK-001  
**Blocks:** TASK-022

---

## Mục tiêu

Thêm type `RemotePreflightStatus` vào `dev-server-types.ts` để mô tả kết quả preflight check đầy đủ (gh + git) trên remote dev server.

---

## File cần sửa

**Path:** `src/shared/dev-server-types.ts`

---

## Thay đổi cần thực hiện

Thêm vào cuối file:

```typescript
export type RemotePreflightStatus = {
  devServerId: string
  platform: NodeJS.Platform
  checkedAt: number           // timestamp (Date.now())
  gh: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  git: {
    installed: boolean
    version?: string
    hasUserName: boolean
    hasUserEmail: boolean
  }
}
```

---

## Acceptance Criteria

- [x] `RemotePreflightStatus` được export từ `dev-server-types.ts`
- [x] Có đầy đủ các fields: `devServerId`, `platform`, `checkedAt`, `gh`, `git`
- [x] `gh.version` và `git.version` là optional (server có thể không cài)
- [x] TypeScript compile thành công
