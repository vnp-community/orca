# TASK-BIGFILE-009 — Move: `orca-runtime-types.ts`

**Loại:** Move (cơ học, nhưng 1 symbol cần thận trọng) · **Effort:** M
**Phụ thuộc:** — (độc lập với TASK-BIGFILE-008, có thể làm song song)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai đoạn 2)

## Kết quả thực thi (2026-08-10)

- Đã di chuyển đúng 14 type export như task mô tả (không có type private
  nào bị kéo theo — khác với TASK-008, ở đây ranh giới "chỉ export" giữ
  đúng vì không có state runtime nào gắn với type).
- `RuntimePtyController`: chạy kiểm tra thực tế bằng `grep` toàn bộ
  `frontend/src` — KHÔNG có importer nào ngoài `orca-runtime.ts` chính nó
  (gitnexus MCP báo "not found", có thể do index cũ chưa cập nhật — grep
  thủ công xác nhận đủ tin cậy). Di chuyển bình thường, không cần loại trừ.
- Cả 14 type đều được `OrcaRuntimeService` dùng làm kiểu tham số/trả về
  rải rác trong thân class → phải thêm `import type { ... } from
  './orca-runtime-types'` (không chỉ 1 dòng `export { ... } from ...` như
  kế hoạch gốc — cùng bài học với TASK-008).
- Dọn thêm 4 import top-level bị unused sau khi type-body chuyển đi
  (`AutomationCreateInput`, `RuntimeTerminalDriverState`, `PtyProcessInfo`,
  `RateLimitState` — chỉ được dùng bên trong các type đã chuyển, không có
  chỗ dùng nào khác trong `orca-runtime.ts`), và bỏ 2 type
  (`MobileNotificationDispatchEvent`/`DismissEvent`) khỏi khối import-back
  vì chỉ dùng gián tiếp qua `MobileNotificationEvent` (giữ nguyên export
  ở cuối file để không phá barrel bên ngoài).
- `orca-runtime.ts`: 24,837 → **24,729 dòng** (giảm ~108 dòng, ít hơn ước
  tính ~1,335 vì phần lớn khối 773–2,108 hoá ra là type PRIVATE không nằm
  trong phạm vi 14 type — task doc gốc ước tính cả vùng, không chỉ 14 type
  liệt kê; không mở rộng phạm vi ngoài kế hoạch để giữ rủi ro thấp).
- File mới `orca-runtime-types.ts`: 160 dòng — không vướng oxlint
  `max-lines`, không cần đăng ký baseline.
- Xác minh: `tsc --noEmit --composite false` — 251 lỗi trước/sau thay đổi
  (baseline giống hệt, xác nhận qua `git stash`) → 0 lỗi mới.
- Phát hiện thêm: pre-commit hook (`husky` + `lint-staged`) CHẶN commit vì
  2 lỗi `consistent-type-imports` pre-existing (từ TASK-008, dòng
  2210/2977 gốc → 2082/2849 sau khi dòng phía trên bị xoá) — dù không do
  task này gây ra, dòng bị dịch chuyển vẫn nằm trong file staged nên hook
  vẫn fail. Đã sửa luôn (an toàn, cơ học): thay 2 chỗ dùng inline
  `import('../notifications/web-push-manager').WebPushManager` bằng 1
  `import type { WebPushManager } from '../notifications/web-push-manager'`
  ở đầu file — không đổi logic, chỉ đổi cú pháp type-import. `oxlint`
  sạch hoàn toàn sau khi sửa (exit 0).

## Input

- File nguồn: `frontend/src/main/runtime/orca-runtime.ts`
- Đọc **đúng dòng 773–2,108** (trước `export class OrcaRuntimeService` ở
  dòng 2,109 — KHÔNG đọc vào trong class).
- Symbol cần chuyển (14 type, không có logic runtime):
  `RemoteFetchResult`, `RemoteTrackingBase`, `AccountsSnapshot`,
  `RuntimeAutomationCreateInput`, `RuntimeAutomationUpdateInput`,
  `RuntimeTerminalAgentStatusEvent`, `RuntimePtyController`,
  `MobileNotificationDispatchEvent`, `MobileNotificationDismissEvent`,
  `MobileNotificationEvent`, `DriverState`, `PtyLayoutTarget`,
  `PtyLayoutState`, `ApplyLayoutResult`

## ⚠️ Lưu ý đặc biệt: `RuntimePtyController`

Type này (dòng 1,171) có khả năng cao được import ở NHIỀU nơi khác ngoài
`orca-runtime.ts` (đã xuất hiện trực tiếp trong investigation
`BUG-FE-PTY-001` gần đây — xem memory session
`bug-fe-pty-001-investigation.md`). **Bắt buộc chạy
`gitnexus impact({target: "RuntimePtyController", direction: "upstream"})`
TRƯỚC KHI làm bất kỳ bước nào khác trong task này.**

- Nếu `impactedCount` thấp: tiến hành bình thường theo các bước dưới.
- Nếu `impactedCount` cao hoặc risk HIGH/CRITICAL: **dừng lại**, báo cáo kết
  quả impact, KHÔNG tự ý di chuyển type này — cân nhắc phương án giữ lại
  `RuntimePtyController` tại `orca-runtime.ts` (tách 13 type còn lại, bỏ
  type này ra khỏi phạm vi task) và ghi chú lại quyết định trong
  `../BUG-FE-BIGFILE-002-orca-runtime.md`.

## Output

- File mới: `frontend/src/main/runtime/orca-runtime-types.ts`
- File nguồn thay 14 (hoặc 13, xem lưu ý trên) định nghĩa bằng
  `export { ... } from './orca-runtime-types'`

## Các bước

1. Chạy impact cho `RuntimePtyController` theo lưu ý ở trên — quyết định có
   di chuyển type này hay không TRƯỚC.
2. `gitnexus impact` cho 13 type còn lại (nhóm lại nếu công cụ cho phép batch
   — nếu không, chạy tuần tự, dừng nếu bất kỳ risk HIGH/CRITICAL).
3. Đọc dòng 773–2,108, copy nguyên văn các type đã xác nhận + import cần
   thiết.
4. Tạo `orca-runtime-types.ts`, paste.
5. Sửa `orca-runtime.ts`: xoá các định nghĩa đã chuyển, thêm barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `orca-runtime.ts` giảm
      ~1,335 dòng (hoặc ít hơn nếu `RuntimePtyController` được giữ lại)
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/main/runtime/orca-runtime.ts
rm frontend/src/main/runtime/orca-runtime-types.ts
```
