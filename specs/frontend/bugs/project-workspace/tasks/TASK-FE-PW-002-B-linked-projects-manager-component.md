# TASK-FE-PW-002-B: Tạo component `LinkedProjectsManager.tsx`

**Domain:** project-workspace
**Solution Ref:** SOL-FE-PW-002 Bước 2
**Priority:** 🔴 P0
**Estimated:** 2 giờ
**Status:** ✅ DONE (2026-09-01)

**Kết quả thực tế:** `LinkedProjectsManager.tsx` tạo mới đúng theo mirror của `MemberManager.tsx`.
`currentUserRole` prop dùng `'owner' | 'member' | null` (đã sửa từ giả định `'viewer'` ban đầu —
xem TASK-FE-PW-002-A). `myProjects` đọc qua `useAppStore.getState()` (không phải hook), khớp
convention hiện có. Tất cả `data-testid` đã liệt kê đều có mặt. 7 test mới trong
`LinkedProjectsManager.test.tsx` — pass (load/filter theo orcaProjectId, empty state, link chỉ
hiện project chưa link, unlink chỉ hiện khi owner, lỗi FORBIDDEN hiển thị đúng message). Ghi chú
kỹ thuật: mock `../../../runtime/runtime-rpc-client` trong test phải tự định nghĩa class
`RuntimeRpcCallError` (component `instanceof` check nó) — gap này cũng tồn tại âm thầm ở
`MemberManager.test.tsx` (không lộ vì không có test nào ở đó chạm nhánh lỗi FORBIDDEN).

---

## Mục tiêu

Component mới, mirror đúng pattern `MemberManager.tsx` (cùng thư mục), quản lý vòng đời link/unlink
1 `Project` (sidebar chính) vào 1 `OrcaProject`.

---

## Files cần tạo

1. `frontend/src/renderer/src/components/project/LinkedProjectsManager.tsx` (CREATE)

---

## Các bước thực thi

### Bước 1: Copy khung `MemberManager.tsx` làm điểm bắt đầu

Giữ đúng cấu trúc: `describeError()` helper, `useState`/`useEffect`/`useCallback` cho load-on-mount,
`Table`/`TableBody`/`TableRow` để hiển thị danh sách, form thêm mới ở trên cùng.

### Bước 2: Implement đầy đủ theo code mẫu trong SOL-FE-PW-002 Bước 2

Điểm khác biệt quan trọng so với `MemberManager.tsx` cần chú ý khi code:

1. **Load danh sách:** không có RPC `orcaProjects.get(orcaProjectId)` riêng — phải gọi
   `orcaProjects.list()` (trả về TẤT CẢ OrcaProject của caller) rồi tự lọc đúng `orcaProjectId`
   đang xem. Không tối ưu bằng 1 lần gọi trực tiếp, nhưng đây là contract RPC thật hiện có —
   không tự chế thêm RPC mới ở task này.
2. **Nguồn Project để link:** `useAppStore(s => s.projects)` (client-side, đã có sẵn) — KHÔNG gọi
   RPC nào để lấy danh sách Project của user, vì Project (mô hình cũ) là per-user JSON, đã có ở
   store.
3. **RBAC hiển thị nút Unlink:** nhận `currentUserRole` qua props (không tự gọi RPC để suy luận
   role trong component này — xem TASK-FE-PW-002-C cho việc resolve giá trị này ở `ProjectSettings`).
4. **Lọc dropdown:** loại bỏ Project đã link rồi khỏi danh sách chọn (`linkableProjects`), tránh
   gọi `linkSourceProject` dư thừa dù backend đã idempotent.

### Bước 3: `data-testid` bắt buộc (để TASK-FE-PW-002-D viết test đối chiếu được)

`linked-projects-manager`, `link-project-form`, `link-project-select`, `link-project-submit`,
`linked-loading`, `linked-empty`, `linked-row-<projectId>`, `unlink-project-<projectId>`.

---

## Verify

```bash
grep -n "orcaProjects.list\|orcaProjects.linkSourceProject\|orcaProjects.unlinkSourceProject" \
  frontend/src/renderer/src/components/project/LinkedProjectsManager.tsx
```

`tsc --noEmit` sạch — component chưa được import ở đâu tại thời điểm này (chỉ tồn tại độc lập),
nên không có rủi ro runtime cho tới TASK-FE-PW-002-C.

## Depends on
TASK-FE-PW-002-A

## Blocking
TASK-FE-PW-002-C, TASK-FE-PW-002-D
