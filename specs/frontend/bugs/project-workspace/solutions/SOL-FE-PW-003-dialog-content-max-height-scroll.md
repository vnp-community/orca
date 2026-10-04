# SOL-FE-PW-003: Thêm `max-h-[85vh] overflow-y-auto` vào `DialogContent` của `ProjectSettings`

> **✅ ĐÃ IMPLEMENT (2026-09-15)** — code fix xong, `gitnexus impact` đã chạy trước khi sửa,
> test suite hiện có xác nhận không bị regress. **Chưa deploy.**

## Bug Reference
- **Bug:** BUG-FE-PW-003
- **Mức độ:** 🟡 MEDIUM
- **File:** `frontend/src/renderer/src/components/project/ProjectSettings.tsx`

---

## Impact Analysis (bắt buộc trước khi sửa theo AGENTS.md/CLAUDE.md)

```
gitnexus impact({target: "ProjectSettings", direction: "upstream", repo: "orca"})
→ risk: LOW, impactedCount: 1 (direct caller duy nhất: ProjectSwitcher.tsx)
```

An toàn để sửa className tại chỗ gọi (`ProjectSettings.tsx`'s own `<DialogContent>`), **không**
sửa `ui/dialog.tsx` dùng chung — tránh đổi hành vi của mọi dialog khác trong app (nhiều dialog
khác có nội dung ngắn, không cần scroll, thêm `max-h`/`overflow-y-auto` mặc định vào đó là thay
đổi không cần thiết và rủi ro hơn nhiều so với override tại 1 chỗ gọi).

## Root Cause

Xem [BUG-FE-PW-003](../BUG-FE-PW-003-project-settings-dialog-no-scroll-cuts-off-content.md).

## Giải pháp

**File:** `frontend/src/renderer/src/components/project/ProjectSettings.tsx`

```diff
-      <DialogContent className="max-w-2xl" data-testid="project-settings-dialog">
+      {/* max-h + overflow-y-auto: General tab stacks Filter + Dev Server picker +
+          Mobile Emulator + Danger zone, which can exceed viewport height — the
+          base DialogContent has no height cap, so without this the lower
+          sections render off-screen with no way to scroll to them. */}
+      <DialogContent
+        className="max-w-2xl max-h-[85vh] overflow-y-auto"
+        data-testid="project-settings-dialog"
+      >
```

`85vh` chọn thay vì `90vh`/`100vh` để luôn còn khoảng hở với mép trên/dưới màn hình (dialog vẫn
căn giữa qua `translate-y-[-50%]` của `dialog.tsx`), tránh dialog dính sát viền trên các màn hình
thấp.

## Không làm ở solution này

- **Không sửa `ui/dialog.tsx` (component `DialogContent` dùng chung)** — blast radius rộng hơn
  nhiều (dùng ở hầu hết mọi dialog trong app), và phần lớn các dialog khác không có vấn đề tràn
  nội dung. Nếu sau này phát hiện thêm dialog khác bị cùng lỗi, nên cân nhắc đưa `max-h`/
  `overflow-y-auto` thành default trong `dialog.tsx` lúc đó — không làm trước khi có bằng chứng
  thứ 2.
- **Không fix 2 bug liên quan khác** ghi trong BUG-FE-PW-003's "Liên quan" (project trùng tên
  "My Repos", repo mới thêm không có `dev_server_id` mặc định) — ngoài phạm vi bug này, cần
  report riêng.

## Testing

- `npx vitest run src/renderer/src/components/project/__tests__/ProjectSettings.test.tsx` — 8/8
  test fail **cả trước và sau fix** (xác nhận bằng `git stash` rồi chạy lại), lỗi
  `useConfirmationDialog must be used inside ConfirmationDialogProvider` — lỗi test-setup có sẵn
  từ trước, không liên quan đến thay đổi này. Không có test nào mới fail do fix.
- `npx tsc --noEmit` — không có lỗi mới liên quan `ProjectSettings.tsx`.
- Chưa có test tự động cho hành vi scroll/overflow (cần jsdom đo layout thật — thường test bằng
  Playwright/visual mới bắt được loại bug CSS-overflow này). Verify thủ công sau khi deploy:
  thu nhỏ trình duyệt xuống chiều cao thấp (~600px), mở Project Settings → General, xác nhận
  dialog tự có scrollbar riêng và phần "Dev server" cuộn tới được.

## Deploy status

- **Chưa deploy** lên `b15.openledger.vn`. Chỉ cần rebuild+sync frontend (không cần rebuild
  17 service backend-go) — dùng `deploy/dev/scripts/build-local.sh` + `sync-to-server.sh`, hoặc
  build frontend riêng rồi rsync `deploy/dev/dist/` như đã làm với fix `DepartmentGate.tsx` trước
  đó trong cùng đợt làm việc này.
