# SOLUTION: BUG-FE-HLD-007 — `ConflictPanel` (F39) chưa implement

**Source-verified:** ✅ Dựa trên source code + đối chiếu TDD
**TDD tham chiếu:** [tdd/v5/16-remote-git-ui.md](../../../tdd/v5/16-remote-git-ui.md) — grep `ConflictPanel`/`Conflict` trong file này trả về **0 kết quả**. TDD chỉ mô tả `<GitLog projectId={...} worktreePath={...} />` (dòng 133) và các component khác đã tồn tại (`DiffViewer`, `CommitForm`, `BranchManager`, `PullRequestForm`). **Kết luận quan trọng: `ConflictPanel` không tồn tại trong TDD — nó chỉ được nhắc ở HLD (`docs/hld/web-server-architecture.md` §10.7), không hề được đặc tả kỹ thuật ở tầng TDD.**

Điều này đổi bản chất bug: đây không phải "code lỡ quên implement 1 thứ đã có TDD", mà là "HLD liệt kê 1 tính năng chưa từng được đưa xuống tầng thiết kế kỹ thuật (TDD) để implement". Ưu tiên xử lý vì vậy nên là **quyết định scope**, không phải viết code ngay.

---

## Fix — quyết định trước, code sau (nếu cần)

### Bước 1: Quyết định scope (việc cần làm ngay, rẻ)

Đưa câu hỏi cho product owner: `ConflictPanel` có còn trong roadmap F39 hay không?

- **Nếu KHÔNG còn trong roadmap:** xoá dòng `ConflictPanel` khỏi `docs/hld/web-server-architecture.md` §10.7. Đóng bug này với status "Won't implement — removed from HLD". Không cần thêm gì vào TDD.
- **Nếu CÒN trong roadmap:** viết 1 mục mới vào `specs/frontend/tdd/v5/16-remote-git-ui.md` đặc tả `ConflictPanel` trước khi code — theo đúng quy trình TDD-trước-implementation mà toàn bộ codebase đang tuân theo (mọi component khác trong file này đều có TDD trước).

### Bước 2 (chỉ nếu quyết định implement) — Đề xuất thiết kế tối thiểu, theo pattern có sẵn của `DiffViewer.tsx`

```tsx
// components/workspace/git/ConflictPanel.tsx
// Why: theo đúng data-flow đã có của GitPanel — repoPath/worktreePath truyền
// từ WorkspaceContext (tdd/v5/16-remote-git-ui.md), không tự fetch riêng.
type ConflictFile = { path: string; ourContent: string; theirContent: string; baseContent: string }

function ConflictPanel({ repoPath, worktreePath }: { repoPath: string; worktreePath: string }) {
  // 1. runtimeGitStatus(target, repoPath) → lọc file có conflict marker
  //    (tái dùng runtime-git-client.ts đã có theo TDD-FE-03 §8, KHÔNG viết
  //    lại 1 git-status call riêng)
  // 2. Danh sách conflict file — click mở trong Monaco diff 3-way (giống
  //    DiffViewer.tsx nhưng 3 panes: ours/theirs/base thay vì 2 pane)
  // 3. Action "AI resolve" (theo mô tả HLD) → gọi agent.exec với prompt
  //    chuẩn hoá (tái dùng launch-agent-in-new-tab.ts đã có, không tạo luồng
  //    agent-launch riêng)
  // 4. Action "Mark resolved" → runtimeGitAdd(target, repoPath, [file.path])
}
```

**Nguyên tắc thiết kế:** không viết lại git operations/agent-launch — tái dùng 100% các hàm `runtime-git-client.ts`/`launch-agent-in-new-tab.ts` đã có theo TDD, `ConflictPanel` chỉ là 1 lớp UI mới trên hạ tầng sẵn có.

## Test cần thêm (chỉ nếu implement)

- `ConflictPanel.test.tsx`: hiển thị đúng danh sách conflict file từ `runtimeGitStatus` mock; click "Mark resolved" gọi đúng `runtimeGitAdd`.

## Tóm tắt thay đổi

| File | Thay đổi |
|---|---|
| `docs/hld/web-server-architecture.md` §10.7 | Xoá `ConflictPanel` (nếu quyết định không làm) HOẶC giữ nguyên chờ TDD |
| `specs/frontend/tdd/v5/16-remote-git-ui.md` | Thêm mục đặc tả `ConflictPanel` trước khi code (nếu quyết định làm) |
| `components/workspace/git/ConflictPanel.tsx` (mới, chỉ nếu làm) | UI 3-way diff + AI resolve action, tái dùng `runtime-git-client.ts` |
