# TASK-FE-HLD-014 — Nêu quyết định scope `ConflictPanel`, chuẩn bị patch cho cả 2 hướng

**Solution:** [SOLUTION-FE-HLD-007](../solutions/SOLUTION-FE-HLD-007-conflictpanel-decision.md)
**Bug:** [BUG-FE-HLD-007](../BUG-FE-HLD-007-gitpanel-conflictpanel-not-implemented.md)
**File:** `docs/hld/web-server-architecture.md`, `specs/frontend/tdd/v5/16-remote-git-ui.md`
**Estimated:** 15 phút
**Status:** ⛔ BLOCKED — cần quyết định product owner, không tự thực thi (đúng theo thiết kế của chính task này). Đã xác nhận lại investigation, chưa áp patch nào — 2026-08-09

---

## Mục tiêu

Đây là **task quyết định scope**, không phải task viết feature code — AI không tự quyết định roadmap sản phẩm. Task này chuẩn bị sẵn 2 patch (A và B) và nêu câu hỏi rõ ràng cho product owner; chỉ áp dụng ĐÚNG MỘT patch sau khi có quyết định.

---

## Context

```bash
grep -n "ConflictPanel" docs/hld/web-server-architecture.md
grep -rn "ConflictPanel" specs/frontend/tdd/
# Xác nhận lại: HLD có nhắc, TDD không có mục nào — đúng như audit đã ghi nhận
```

---

## Câu hỏi cần trả lời trước (không tự quyết định)

> `ConflictPanel` (UI xử lý git merge conflict, "Conflict files + AI resolve") có còn nằm trong roadmap F39 Remote Git UI hay không?

Đặt câu hỏi này ở PR description hoặc issue tracker, gắn tag người phụ trách F39/product owner.

---

## Patch A — Nếu câu trả lời là "KHÔNG còn trong roadmap"

**File:** `docs/hld/web-server-architecture.md` §10.7

**TÌM:** dòng liệt kê `ConflictPanel` trong bảng sub-component của GitPanel.

**THAY BẰNG:** xoá dòng đó khỏi bảng, giữ nguyên các dòng khác (`DiffViewer`, `CommitForm`, `BranchManager`, `PullRequestForm`, `GitLog`).

Cập nhật [BUG-FE-HLD-007](../BUG-FE-HLD-007-gitpanel-conflictpanel-not-implemented.md): đổi `Status` từ 🔴 Open sang ✅ Closed (Won't implement — removed from HLD), thêm 1 dòng ghi ngày quyết định.

---

## Patch B — Nếu câu trả lời là "CÒN trong roadmap"

**File:** `specs/frontend/tdd/v5/16-remote-git-ui.md`

Thêm 1 mục mới (theo đúng format các mục component khác trong cùng file — component name, props, data flow, file path dự kiến):

```markdown
## X. ConflictPanel (F39 — pending implementation, xem BUG-FE-HLD-007)

**File dự kiến:** `src/renderer/src/components/workspace/git/ConflictPanel.tsx`

- Input: `repoPath`, `worktreePath` (giống `GitLog`)
- Data: `runtimeGitStatus(target, repoPath)` (đã có, TDD-FE-03 §8) — lọc file có conflict marker
- UI: danh sách conflict file, click mở Monaco 3-way diff (ours/theirs/base)
- Action "AI resolve": tái dùng `launch-agent-in-new-tab.ts` (đã có), không tạo luồng agent-launch riêng
- Action "Mark resolved": `runtimeGitAdd(target, repoPath, [file.path])`
```

Sau khi thêm mục TDD này, tạo 1 bug/task mới (ngoài phạm vi `hld-v1`, ví dụ domain `project-workspace` hoặc `code-review`) để theo dõi việc implement `ConflictPanel.tsx` thật — **không** implement trong task này.

Cập nhật [BUG-FE-HLD-007](../BUG-FE-HLD-007-gitpanel-conflictpanel-not-implemented.md): giữ Status 🔴 Open, thêm link tới task/bug mới vừa tạo để theo dõi implementation.

---

## Verify

```bash
# Sau khi áp dụng đúng 1 patch:
grep -n "ConflictPanel" docs/hld/web-server-architecture.md specs/frontend/tdd/v5/16-remote-git-ui.md
# Patch A: 0 kết quả ở cả 2 file
# Patch B: 0 kết quả ở HLD (không đổi), có 1 mục mới ở TDD
```

---

## Definition of Done

- [x] Câu hỏi scope đã được đặt ra — **chưa có câu trả lời**, xem "Kết quả thực thi"
- [ ] Đúng 1 trong 2 patch (A hoặc B) đã áp dụng — **CHƯA làm**, đúng chủ ý: đây là task quyết định scope sản phẩm, không phải task code — AI không tự chọn thay product owner
- [ ] `BUG-FE-HLD-007` cập nhật Status khớp patch đã chọn — chờ bước trên
- [ ] Nếu chọn Patch B: tạo task/bug theo dõi implementation ở domain phù hợp — chờ quyết định

## Kết quả thực thi

Xác nhận lại investigation (không đổi so với lúc viết task, không có thay đổi nào ở 2 file liên quan):

```
docs/hld/web-server-architecture.md:781 → | `ConflictPanel` | F39 | Conflict files + AI resolve |
specs/frontend/tdd/v5/16-remote-git-ui.md → grep "ConflictPanel" → 0 kết quả
frontend/src (toàn bộ) → grep "ConflictPanel" → 0 kết quả
```

**Không áp dụng patch nào.** Đây là quyết định phạm vi sản phẩm (giữ hay bỏ 1 tính năng đã tài liệu hoá nhưng chưa từng đưa vào thiết kế kỹ thuật/implementation) — nằm ngoài thẩm quyền của việc "thực thi task sửa bug". Để nguyên trạng thái `⛔ BLOCKED`, không tự chọn Patch A (xoá khỏi HLD) hay Patch B (thêm vào TDD + tạo task implement) thay cho product owner.
