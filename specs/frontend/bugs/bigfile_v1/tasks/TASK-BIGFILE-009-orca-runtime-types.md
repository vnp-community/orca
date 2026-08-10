# TASK-BIGFILE-009 — Move: `orca-runtime-types.ts`

**Loại:** Move (cơ học, nhưng 1 symbol cần thận trọng) · **Effort:** M
**Phụ thuộc:** — (độc lập với TASK-BIGFILE-008, có thể làm song song)
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai đoạn 2)

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
