# TASK-025: Sửa `src/main/ipc/repo-ipc.ts` — Remote Repo IPC Handlers

**Phase:** 2 — Remote Repo  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §C.3  
**Depends on:** TASK-023, TASK-024, TASK-026  
**Blocks:** TASK-027

---

## Mục tiêu

Thêm 4 IPC handlers cho remote repo operations vào `repo-ipc.ts`:
1. `repo.listRemoteDirectory` — list thư mục trên dev server
2. `repo.addRemote` — thêm remote repo vào store
3. `repo.cloneRemote` — clone repo trên dev server
4. `repo.scanRemote` — scan git repos trong một thư mục

---

## File cần sửa

**Path:** `src/main/ipc/repo-ipc.ts`

---

## Thay đổi cần thực hiện

Trong hàm register IPC handlers, thêm:

```typescript
import type { DevServerManager } from '../dev-server/dev-server-manager'
import { basename } from 'node:path'

// Cần inject devServerManager vào repo-ipc handlers
// (thêm vào function signature nếu cần)

// === listRemoteDirectory ===
ipc.handle('repo.listRemoteDirectory', async (_, params: {
  devServerId: string
  path: string
  includeGitStatus?: boolean
}): Promise<{ entries: DirectoryEntry[]; platform: NodeJS.Platform }> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')
  return relay.session!.call('fs.listDirectory', {
    path: params.path,
    includeGitStatus: params.includeGitStatus ?? false
  })
})

// === addRemote ===
ipc.handle('repo.addRemote', async (_, params: {
  devServerId: string
  path: string
  name?: string
}): Promise<Repo> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')

  // Validate path tồn tại trên remote
  const statResult = await relay.session!.call('fs.stat', { path: params.path })
  if (!statResult.exists) {
    throw new Error(`Path does not exist on dev server: ${params.path}`)
  }

  const devServer = devServerManager.get(params.devServerId)!
  return runtimeService.addRepo({
    path: params.path,
    name: params.name ?? basename(params.path),
    connectionId: devServer.sshTargetId,
    devServerId: params.devServerId
  })
})

// === cloneRemote ===
ipc.handle('repo.cloneRemote', async (_, params: {
  devServerId: string
  url: string
  targetDir?: string
}): Promise<{ repoId: string; path: string }> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')

  const devServer = devServerManager.get(params.devServerId)!
  const workspaceDir = devServer.workspaceDir ?? '~/orca/workspaces'
  const repoName = params.url.split('/').pop()?.replace(/\.git$/, '') ?? 'repo'
  const targetPath = params.targetDir ?? `${workspaceDir}/${repoName}`

  await relay.session!.call('git.clone', { url: params.url, targetPath })

  const repo = await runtimeService.addRepo({
    path: targetPath,
    name: repoName,
    connectionId: devServer.sshTargetId,
    devServerId: params.devServerId
  })
  return { repoId: repo.id, path: targetPath }
})

// === scanRemote ===
ipc.handle('repo.scanRemote', async (_, params: {
  devServerId: string
  rootPath: string
  maxDepth?: number
}): Promise<{ path: string; name: string }[]> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')

  const { entries } = await relay.session!.call('fs.listDirectory', {
    path: params.rootPath,
    includeGitStatus: true
  })

  return entries
    .filter((e: DirectoryEntry) => e.isGitRepo)
    .map((e: DirectoryEntry) => ({
      path: e.path,
      name: basename(e.path)
    }))
})
```

---

## Acceptance Criteria

- [x] `repo.listRemoteDirectory`: forward đến relay, trả về entries + platform
- [x] `repo.addRemote`: validate path tồn tại trên remote trước khi add
- [x] `repo.addRemote`: lưu `devServerId` vào repo record
- [x] `repo.cloneRemote`: clone trên relay, add repo vào store
- [x] `repo.cloneRemote`: `targetDir` mặc định theo `devServer.workspaceDir`
- [x] `repo.scanRemote`: chỉ trả về directories có `.git`
- [x] Tất cả handlers throw Error khi relay không connected
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Tìm `runtimeService.addRepo()` hoặc tương đương trong codebase
2. Kiểm tra `fs.stat` có được đăng ký trong relay không — nếu chưa, thêm basic stat handler
3. `DevServerManager` cần được inject vào `repo-ipc.ts` (sửa function signature)
