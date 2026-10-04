# BUG-FE-PW-003: `ProjectSettings` dialog không cuộn được — tab General bị cắt cụt, phần "Dev server" biến mất khỏi màn hình

## Mức độ: 🟡 MEDIUM

## Trạng thái: ✅ FIXED (2026-09-15, code) — xem [SOL-FE-PW-003](./solutions/SOL-FE-PW-003-dialog-content-max-height-scroll.md).
⚠️ **Chưa deploy** lên `b15.openledger.vn` tại thời điểm ghi nhận — cần 1 lượt build+sync frontend
riêng (không cần rebuild backend-go).

## Tóm tắt

Người dùng report nhiều lần liên tiếp trên `b15.openledger.vn`, cùng một triệu chứng dưới nhiều
cách diễn đạt khác nhau:

- "từ bước 3 sang 4 k thấy" (bước 3 = chọn tab General, bước 4 = phần chọn dev server cho repo)
- "vẫn k thấy" (sau khi được gợi ý zoom out trình duyệt)
- "tab general k thấy chỗ nào có dev server"

Ban đầu bị nghi nhầm là do 2 nguyên nhân khác (đã loại trừ bằng cách kiểm tra DB production trực
tiếp trước khi kết luận đây là bug UI thật):

1. ~~Repo "aiops" chưa được thêm vào đúng project~~ → loại trừ: repo tồn tại đúng project
   (`project.repos` xác nhận `display_name='aiops'`/`aiops-v3` thuộc `project_id` đúng).
2. ~~2 project trùng tên "My Repos" (1 cái rỗng, 1 cái có repo)~~ → xác nhận **có thật** (bug khác,
   chưa fix, xem "Liên quan" bên dưới), nhưng ngay cả sau khi xác nhận user đã mở đúng project có
   repo, triệu chứng "không thấy phần Dev server" **vẫn tái diễn** — chứng tỏ còn 1 nguyên nhân
   thứ 3 độc lập, chính là bug này.

## Root Cause — CONFIRMED

`frontend/src/renderer/src/components/ui/dialog.tsx`'s `DialogContent` không có `max-height` hay
`overflow-y-auto` trong class mặc định:

```
'fixed top-[50%] left-[50%] z-50 grid w-full max-w-[calc(100%-2rem)] translate-x-[-50%]
translate-y-[-50%] gap-4 rounded-lg border ... sm:max-w-lg'
```

`ProjectSettings.tsx` chỉ override `max-w-2xl` (giới hạn bề ngang), không set thêm chiều cao:

```tsx
<DialogContent className="max-w-2xl" data-testid="project-settings-dialog">
```

Tab **General** xếp chồng nhiều section trong 1 `<div className="space-y-6">`:
`ProjectDevServerFilterSection` → `ProjectDevServerSection` (phần bị "biến mất") →
`ProjectMobileEmulatorAgentSection` → (nếu owner) khối "Danger zone". Vì `DialogContent` dùng
`position: fixed` và không tự giới hạn/cuộn chiều cao, khi tổng chiều cao nội dung vượt quá
viewport, phần vượt quá bị cắt ra ngoài khung nhìn **không có cách nào cuộn tới** — "kéo xuống"
không có tác dụng vì bản thân dialog không có scrollbar riêng.

Xác nhận qua đọc source trực tiếp (`ui/dialog.tsx`, `ProjectSettings.tsx`,
`ProjectDevServerSection.tsx`) — không phải suy đoán từ log, vì đây là bug CSS thuần, không cần
log runtime để chứng minh.

## Ảnh hưởng

- Chặn hoàn toàn thao tác gán dev server cho repo qua UI (`repo.rebindDevServer`) trên bất kỳ
  màn hình nào đủ ngắn để tab General bị tràn — không giới hạn riêng project/repo nào.
- Gây ra 1 chuỗi report lặp lại tưởng như "chưa tìm đúng chỗ" trong khi thực chất UI đúng vị trí
  nhưng không thể nhìn thấy được.
- Cùng pattern có thể ảnh hưởng bất kỳ dialog nào khác dùng `DialogContent` không tự thêm
  `max-h`/`overflow-y-auto` khi nội dung dài — solution này chỉ scope vào `ProjectSettings`
  (nơi đã xác nhận có bug thật), không sửa `dialog.tsx` dùng chung (xem "Không làm ở solution này"
  trong SOL-FE-PW-003).

## Liên quan

- **[SOL-FE-PW-003](./solutions/SOL-FE-PW-003-dialog-content-max-height-scroll.md)** — fix đã áp
  dụng.
- **Bug khác, CHƯA fix, phát hiện cùng lúc khi điều tra bug này**: `project.projects` trên
  `b15.openledger.vn` có 2 row cùng tên "My Repos" cùng `tenant_id` (`39864973-...` rỗng 0 repo,
  `c6b54fe0-...` có repo) — gây nhầm lẫn project switcher, làm phức tạp thêm việc chẩn đoán bug
  này. Chưa xác định nguyên nhân sinh ra 2 row trùng tên (nghi vấn: bootstrap tạo
  `DEFAULT_PROJECT_NAME` chạy nhiều lần) — cần bug report riêng nếu muốn fix tận gốc.
- **Bug khác, CHƯA fix**: repo mới thêm vào project (qua tab Repos) mặc định `dev_server_id` rỗng,
  khiến `dispatchExecutorForRepo` (git-gateway-service) fallback chạy git cục bộ trên container
  thay vì relay đúng dev server → `WORKTREE_CREATE_FAILED: ... no such file or directory`. Đây là
  hành vi tự nhiên của thiết kế Phase 10 (mỗi repo phải được gán dev server thủ công sau khi thêm),
  không hẳn là "bug", nhưng kết hợp với BUG-FE-PW-003 (không thấy được chỗ gán) làm luồng thêm
  repo mới gần như không dùng được nếu màn hình ngắn.
