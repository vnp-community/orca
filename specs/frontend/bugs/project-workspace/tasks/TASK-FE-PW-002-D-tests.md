# TASK-FE-PW-002-D: Test cho `LinkedProjectsManager` + `ProjectSettings` tab mới

**Domain:** project-workspace
**Solution Ref:** SOL-FE-PW-002 (Verification)
**Priority:** 🟡 P1
**Estimated:** 2 giờ
**Status:** ✅ DONE (2026-09-01) — Bước 1 và 2

**Kết quả thực tế:** `LinkedProjectsManager.test.tsx` (7 test, mục tiêu Bước 1) và mở rộng
`ProjectSettings.test.tsx` (+2 test, mục tiêu Bước 2) — toàn bộ pass, không đụng test cũ nào. Tổng
sau khi cả 6 task xong: **51/51 test pass** trong `components/project/` (6 file:
`CreateProjectDialog.test.tsx` 12, `ProjectSettings.test.tsx` 8, `MemberManager.test.tsx` 5,
`LinkedProjectsManager.test.tsx` 7, + `ProjectSwitcher.test.tsx`/`RepoMemberManager.test.tsx`
không đổi). **Bước 3 (test tích hợp end-to-end chống rò rỉ dữ liệu chéo-user) CHƯA làm** — vẫn
đúng như ghi chú "khuyến nghị, không bắt buộc" ban đầu; nếu cần, đây là việc còn lại duy nhất
trong toàn bộ 2 bug này.

---

## Mục tiêu

Phủ test cho component mới theo đúng pattern `MemberManager.test.tsx` đã có (mock `callRuntimeRpc`,
không gọi RPC thật), và guard chống rò rỉ dữ liệu chéo-user nếu logic lọc sau này bị sửa sai.

---

## Files cần tạo/sửa

1. `frontend/src/renderer/src/components/project/__tests__/LinkedProjectsManager.test.tsx` (CREATE)
2. `frontend/src/renderer/src/components/project/__tests__/ProjectSettings.test.tsx` (MODIFY)

---

## Các bước thực thi

### Bước 1: `LinkedProjectsManager.test.tsx` — theo khung `MemberManager.test.tsx`

Mock `callRuntimeRpc` (cùng cách `MemberManager.test.tsx` đã mock), cover:

```
- load(): gọi orcaProjects.list, lọc đúng orcaProjectId, hiển thị sourceProjects đúng
- empty state khi sourceProjects rỗng (data-testid="linked-empty")
- link: chọn 1 Project trong dropdown "linkableProjects" (loại bỏ Project đã link), submit gọi
  orcaProjects.linkSourceProject({orcaProjectId, projectId}) đúng tham số, reload sau khi xong
- unlink: chỉ hiện nút khi currentUserRole==='owner'; click gọi orcaProjects.unlinkSourceProject,
  xoá row khỏi UI ngay (optimistic, không chờ reload)
- currentUserRole==='member'|'viewer': KHÔNG render nút unlink cho bất kỳ row nào
- lỗi FORBIDDEN/UNAUTHENTICATED từ linkSourceProject → hiển thị "You do not have permission to do that."
  (khớp describeError, không leak message gốc từ backend)
```

### Bước 2: Mở rộng `ProjectSettings.test.tsx`

```
- tab "linked" render, click vào tab gọi project.getMembers để resolve currentUserRole (hoặc RPC
  đã chốt dùng thật sau khi xác nhận ở TASK-FE-PW-002-C), rồi render LinkedProjectsManager với
  đúng props
- 3 tab cũ (general/members/repos) không bị ảnh hưởng — chạy lại full suite cũ, 0 test nào đổi kết quả
```

### Bước 3 (khuyến nghị, không bắt buộc để đóng task): test tích hợp chống rò rỉ dữ liệu

Nếu môi trường test cho phép dựng RPC handler thật (theo pattern
`orca-project-sharing-rpc-handler.test.ts` ở backend), thêm 1 test end-to-end: user B (không phải
owner của Project P) cố `linkSourceProject` hộ user A → phải bị chặn ở tầng backend
(`ownerUserId !== actingUserId` → `FORBIDDEN`), UI hiển thị đúng thông báo lỗi thân thiện. Đây là
đường rò rỉ dữ liệu tiềm ẩn nếu code sau này lỡ truyền nhầm `ownerUserId` từ client thay vì luôn
lấy từ `ctx.userId` — ghi lại rõ trong bug này để không ai "tối ưu" bỏ ràng buộc đó trong tương lai.

---

## Verify

```bash
pnpm --filter frontend test -- LinkedProjectsManager
pnpm --filter frontend test -- ProjectSettings
```

Toàn bộ test cũ trong 2 file `MemberManager.test.tsx`/`ProjectSettings.test.tsx` (hiện có) phải
vẫn pass nguyên vẹn (regression guard).

## Depends on
TASK-FE-PW-002-B, TASK-FE-PW-002-C

## Blocking
Không có — task cuối trong chuỗi BUG-FE-PW-002
