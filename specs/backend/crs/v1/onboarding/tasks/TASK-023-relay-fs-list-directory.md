# TASK-023: Sửa `src/relay/fs-handler.ts` — Thêm `fs.listDirectory` với `includeGitStatus`

**Phase:** 2 — Remote Repo  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §C.1  
**Depends on:** (không — relay-side)  
**Blocks:** TASK-025

---

## Mục tiêu

Thêm handler `fs.listDirectory` vào relay, cho phép list directories từ xa và optionally kiểm tra git status cho từng entry.

---

## File cần sửa/tạo

**Path:** `src/relay/fs-handler.ts` (MODIFY nếu đã tồn tại) hoặc tạo mới

Nếu cần tạo file riêng: `src/relay/fs-handler-directory-browse.ts`

---

## Nội dung cần implement

```typescript
import { readdir, stat } from 'node:fs/promises'
import { join } from 'node:path'

export type DirectoryEntry = {
  name: string
  path: string
  isDirectory: boolean
  isGitRepo: boolean
}

// Nếu thêm vào class hiện có:
// Nếu tạo class mới:
export class FsDirectoryBrowserHandler {
  constructor(private dispatcher: RelayDispatcher) {
    this.dispatcher.onRequest('fs.listDirectory', (p) => this.listDirectory(p as any))
  }

  private async listDirectory(params: {
    path: string
    includeGitStatus?: boolean
  }): Promise<{
    entries: DirectoryEntry[]
    platform: NodeJS.Platform
  }> {
    const { path: dirPath, includeGitStatus = false } = params

    let entries: DirectoryEntry[]
    try {
      const items = await readdir(dirPath, { withFileTypes: true })
      entries = await Promise.all(
        items
          .filter(item => item.isDirectory())
          .map(async item => {
            const fullPath = join(dirPath, item.name)
            let isGitRepo = false
            if (includeGitStatus) {
              isGitRepo = await this.isGitRepo(fullPath)
            }
            return {
              name: item.name,
              path: fullPath,
              isDirectory: true,
              isGitRepo
            }
          })
      )
    } catch (err) {
      throw new Error(`Cannot list directory ${dirPath}: ${(err as Error).message}`)
    }

    return { entries, platform: process.platform }
  }

  private async isGitRepo(dirPath: string): Promise<boolean> {
    try {
      await stat(join(dirPath, '.git'))
      return true
    } catch {
      return false
    }
  }
}
```

---

## Acceptance Criteria

- [x] `fs.listDirectory` được đăng ký trên dispatcher
- [x] Chỉ trả về directories (không phải files)
- [x] `includeGitStatus: true` → check `.git` folder cho mỗi directory
- [x] `includeGitStatus: false` → `isGitRepo: false` cho tất cả (không check)
- [x] Path không tồn tại → throw Error rõ ràng
- [x] Response có `platform: process.platform`
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Kiểm tra `src/relay/fs-handler.ts` có tồn tại chưa — nếu có, thêm method vào class
2. Nếu chưa có, tạo file mới và đăng ký trong relay bootstrap
3. Tìm `RelayDispatcher` type trong codebase để dùng đúng
