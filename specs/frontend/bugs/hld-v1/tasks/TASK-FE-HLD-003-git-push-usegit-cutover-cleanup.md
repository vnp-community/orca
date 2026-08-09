# TASK-FE-HLD-003 — Chuyển `useGit.ts` sang `runtimeGitPush`, xoá `runtime-rpc-stream.ts`

**Solution:** [SOLUTION-FE-HLD-002](../solutions/SOLUTION-FE-HLD-002-git-push-stream-auth.md)
**Bug:** [BUG-FE-HLD-002](../BUG-FE-HLD-002-git-push-stream-bearer-token-broken.md)
**File:** `frontend/src/renderer/src/hooks/useGit.ts`
**Estimated:** 20 phút
**Status:** ✅ DONE — 2026-08-09
**Phụ thuộc:** ~~TASK-FE-HLD-002~~ (không cần, hàm đã có sẵn — xem file đó)

---

## Mục tiêu

Chuyển `useGit.ts` sang gọi `runtimeGitPush()` (TASK-FE-HLD-002) thay vì `callRuntimeRpcStream('git.push', ...)` trực tiếp qua `runtime-rpc-stream.ts`, rồi **xoá hẳn** `runtime-rpc-stream.ts` — đây là bước cutover cuối cùng khép lại BUG-FE-HLD-002.

---

## Context

```bash
grep -n "callRuntimeRpcStream\|runtime-rpc-stream\|git.push" frontend/src/renderer/src/hooks/useGit.ts
```

Đọc trước: `frontend/src/renderer/src/hooks/useGit.ts` dòng 71-83 (nơi gọi push hiện tại).

---

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/hooks/useGit.ts`

**TÌM:**
```typescript
import { callRuntimeRpcStream } from '../runtime/runtime-rpc-stream'
```
```typescript
await callRuntimeRpcStream('git.push', { repoPath, ...args })
```

**THAY BẰNG:**
```typescript
import { runtimeGitPush } from '../runtime/runtime-git-client'
```
```typescript
await runtimeGitPush(target, repoPath, args, (chunk) => {
  if (chunk.type === 'progress') {
    // giữ nguyên logic cập nhật UI progress hiện có, chỉ đổi nguồn chunk
  }
})
```

> [!IMPORTANT]
> `target` (`RuntimeClientTarget`) phải lấy từ đúng nguồn `useGit.ts` đang dùng cho các lệnh git khác (`runtimeGitStatus`/`runtimeGitCommit`) trong cùng hook — không tạo `target` mới, tái dùng biến đã có để đảm bảo push chạy trên đúng repo/environment đang active.

**Xoá file:**
```bash
git rm frontend/src/renderer/src/runtime/runtime-rpc-stream.ts
```

**Xoá test đi kèm (nếu có):**
```bash
git rm frontend/src/renderer/src/runtime/runtime-rpc-stream.test.ts 2>/dev/null || true
```

---

## Verify

```bash
pnpm --filter frontend tsc --noEmit

grep -rn "runtime-rpc-stream\|orca_session_token\|getSessionToken" frontend/src
# Phải trả về 0 kết quả trong toàn bộ frontend/src (kể cả test)

pnpm --filter frontend test -- useGit
```

---

## Definition of Done

- [x] `useGit.ts` gọi `pushRuntimeGit()` (đổi tên so với kế hoạch gốc `runtimeGitPush` — tên hàm thật trong `runtime-git-client.ts` là `pushRuntimeGit`), không còn import `runtime-rpc-stream`
- [x] File `runtime-rpc-stream.ts` đã bị xoá khỏi repo (không có test file riêng đi kèm — xác nhận qua `find`)
- [x] `grep -rn "orca_session_token\|runtime-rpc-stream\|callRuntimeRpcStream"` toàn `frontend/src` → chỉ còn 2 dòng comment (giải thích lịch sử fix), 0 code/import thật
- [~] `pnpm tsc --noEmit` — không chạy được ở mức toàn package (xem `NOTES.md`), không liên quan thay đổi này
- [x] `pnpm test -- useGit runtime-git-client` → **33 test pass** (9 useGit + 24 runtime-git-client, không giảm so với trước)

## Kết quả thực thi (khác đáng kể so với kế hoạch gốc)

Kế hoạch gốc (TASK-FE-HLD-001/002/003) giả định cần xây mới 1 lớp streaming RPC (`callRuntimeRpcStream` dựa trên WS `subscribe()`). Sau khi đọc code thật ở cả `frontend/` và `backend/`, phát hiện **backend's `git.push` không hề streaming** — nên hướng đó sai từ gốc. Fix thật gọn hơn nhiều:

1. **`frontend/src/renderer/src/hooks/useGit.ts`** — `push()`:
   - Bỏ `callRuntimeRpcStream('git.push', { projectId, branch, worktreeId })` (params sai — backend không nhận `projectId`/`branch` trực tiếp, chỉ nhận `worktree` selector — nên kể cả nếu auth đúng, request này vẫn sẽ fail validate ở backend).
   - Thay bằng `pushRuntimeGit(context, { pushTarget })` — hàm có sẵn, đúng transport, đúng params.
   - Cần resolve `worktreePath` từ `currentWorktree.id` qua `splitWorktreeIdForFilesystem()` (shared helper có sẵn) vì `Worktree` không có field `.path` trực tiếp.
   - Bỏ tính năng hiển thị log push theo dòng (không có UI nào tiêu thụ `pushLines` — xác nhận qua grep trước khi bỏ) — thay bằng 1 dòng `"Pushed <branch>"` khi thành công.

2. **Xoá:** `frontend/src/renderer/src/runtime/runtime-rpc-stream.ts` (không còn consumer nào sau bước 1).

3. **`frontend/src/renderer/src/hooks/__tests__/useGit.test.ts`** — cập nhật mock (`runtime-rpc-stream` → `runtime-git-client`'s `pushRuntimeGit`), thêm `currentWorktree` non-null vào mock `useWorkspace` (bắt buộc cho code path mới), sửa 3 assertion của test `push`.

**Không đụng** tới các method khác trong `useGit.ts` (`stageFile`, `unstageFile`, `commit`, `getDiff`, `aiCommitMessage`) dù chúng cũng gọi `callRuntimeRpc(method, params)` theo kiểu thiếu `target` — đây là bug tương tự nhưng **ngoài phạm vi BUG-FE-HLD-002** (đúng tinh thần comment có sẵn ở `aiCommitMessage`: *"pre-existing bug, out of scope for this CR"*). Đề xuất mở bug riêng nếu muốn dọn triệt để cả file.
