# TASK-FE-PW-003-A: Thêm `max-h-[85vh] overflow-y-auto` vào `ProjectSettings`'s `DialogContent`

**Solution:** [SOL-FE-PW-003](../solutions/SOL-FE-PW-003-dialog-content-max-height-scroll.md)
**Status:** ✅ DONE (2026-09-15, code) — chưa deploy

## Việc đã làm

1. `gitnexus impact({target: "ProjectSettings", direction: "upstream"})` → LOW risk, 1 caller
   (`ProjectSwitcher`) — báo cáo trước khi sửa theo đúng quy tắc bắt buộc.
2. Sửa `frontend/src/renderer/src/components/project/ProjectSettings.tsx`: className của
   `DialogContent` thêm `max-h-[85vh] overflow-y-auto`, kèm comment giải thích lý do (why, không
   phải what).
3. Chạy `npx vitest run .../ProjectSettings.test.tsx` — xác nhận 8 test fail sẵn có từ trước
   (bằng `git stash` so sánh trước/sau) không liên quan đến thay đổi này.
4. Chạy `npx tsc --noEmit` — không phát sinh lỗi type mới.

## Verify (sau khi deploy)

- [ ] Build frontend + sync lên `b15.openledger.vn`.
- [ ] Thu nhỏ chiều cao trình duyệt (~600px), mở project "My Repos" (project **có** repo, không
      phải bản trùng tên rỗng) → Settings → tab General.
- [ ] Xác nhận dialog tự có scrollbar riêng, cuộn thấy được phần "Dev server"
      (`ProjectDevServerSection`) và "Danger zone" (nếu là owner).
- [ ] Cập nhật dòng "Chưa deploy" ở BUG-FE-PW-003.md / SOL-FE-PW-003.md thành đã deploy +
      confirm thật sau khi verify xong.
