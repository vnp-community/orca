# TASK-BIGFILE-008 — Move: `orca-runtime-tail-buffer.ts`

**Loại:** Move (cơ học, nhưng scope thực tế lớn hơn nhiều so với ước tính ban
đầu — xem "Kết quả thực thi") · **Effort:** S (ước tính ban đầu — **thực tế: L**)
· **Phụ thuộc:** — · **Status:** ✅ Done (commit `596be55bc`)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai đoạn 1)

## ⚠️ Kết quả thực thi (2026-08-10) — đọc trước khi dùng lại làm mẫu cho task khác

Ước tính ban đầu ("10 export, tất cả pure function") dựa trên grep nông
(chỉ export top-level) và **sai đáng kể**. Khi thực thi thực tế:

1. Vùng 24,786→hết file không chỉ có 10 hàm — có thêm **~90 hàm/const
   private khác** cũng nằm trong đúng vùng đó (cùng 1 khối liên tục, nên vẫn
   di chuyển được nguyên khối — điểm này ước tính ban đầu đúng).
2. Nhưng **~50 trong số các hàm/const đó được `OrcaRuntimeService` gọi trực
   tiếp** (tên trần, không qua `this`) rải khắp thân class (dòng ~3,670 đến
   ~21,956) — không phải chỉ 8-10 hàm như dự kiến. Phải thêm khối `import`
   lớn (50+ tên) vào `orca-runtime.ts` để class tiếp tục dùng được sau khi
   di chuyển — không chỉ 1 dòng `export { ... } from ...` như kế hoạch gốc.
3. Phát hiện thêm 1 const bị dùng CHÉO namespace: `RECENT_PTY_OUTPUT_LIMIT`
   (dòng 1248, cách xa vùng 24786) chỉ được dùng bởi `appendRecentPtyOutput`
   — phải di chuyển thêm cùng 3 const `RECENT_PTY_PATH_CANDIDATE_*` (1249-1251)
   và hàm `normalizeLocalBranchName` (1642-1644, dùng cả trong class lẫn
   trong vùng tail).
4. 3 type dùng chung nhiều (`RuntimeLeafRecord` 25 lượt,
   `RuntimePtyWorktreeRecord` 30 lượt, `ResolvedWorktree` rất nhiều) phải
   **giữ nguyên** trong `orca-runtime.ts` (export type) thay vì di chuyển —
   import type ngược lại vào file mới (an toàn, type-only, không circular
   runtime).
5. File mới sau khi tách vẫn **~1,818 dòng** — vượt ngưỡng oxlint max-lines.
   Đã đăng ký vào `config/max-lines-baseline.txt` kèm cảnh báo rõ "NEEDS PR
   REVIEW" (theo đúng `AGENTS.md`, không tự ý disable).
6. Phát hiện phụ: `pnpm exec node config/scripts/check-max-lines-ratchet.mjs`
   hiện **báo sai hàng loạt** trên toàn repo (kể cả `orca-runtime.ts` gốc,
   vốn đã có disable comment từ trước, hoàn toàn không liên quan tới thay
   đổi này) — cơ chế ratchet có vẻ đã lỗi thời so với cấu trúc repo hiện tại
   (`backend/`/`desktop/`/`frontend/`/`agent/` tách từ 1 monorepo). Không sửa
   trong task này (ngoài phạm vi) — ghi nhận lại để xử lý riêng.

**Bài học cho các task Move còn lại**: KHÔNG tin ranh giới "10 export" chỉ từ
grep — bắt buộc kiểm tra `grep -c "<tên>" file | awk '$1 < <dòng bắt đầu
vùng>'` cho MỌI symbol định di chuyển (không chỉ export) trước khi cắt, và
chạy `tsc --noEmit` lặp lại để bắt các phụ thuộc còn thiếu thay vì tin vào
kế hoạch tĩnh ban đầu.

---

## Input

- File nguồn: `frontend/src/main/runtime/orca-runtime.ts` (26,730 dòng —
  **chỉ đọc đúng dòng 24,786 đến hết file**, ~1,945 dòng, KHÔNG đọc phần
  khác của file này cho task này).
- Symbol cần chuyển (10 export, tất cả pure function, không phụ thuộc
  `this`):
  `appendRecentPtyOutput`, `appendRecentPtyPathCandidates`,
  `recentTerminalPathCandidatesIncludePath`,
  `recentTerminalOutputIncludesPath`, `buildPreview`,
  `type TerminalTailWaitState`, `computeTerminalTailWaitState`,
  `tailGainedNewerBlockedReason`, `appendNormalizedToTailBuffer`,
  `appendNormalizedToMultilineTailBufferUnwindowed`

## Output

- File mới: `frontend/src/main/runtime/orca-runtime-tail-buffer.ts`
- File nguồn thay 10 định nghĩa trên bằng 1 dòng
  `export { ... } from './orca-runtime-tail-buffer'` (liệt kê đủ 10 symbol)
  ở CUỐI file (thay cho toàn bộ nội dung dòng 24,786→hết).

## Các bước

1. `gitnexus impact` cho từng symbol trong 10 symbol trên — dừng nếu bất kỳ
   symbol nào risk HIGH/CRITICAL. Kiểm tra riêng: các hàm này có khả năng cao
   ĐÃ có test riêng (`orca-runtime.test.ts` hoặc tương tự) — nếu impact cho
   thấy file test import trực tiếp, xác nhận sau khi tách test vẫn pass mà
   không cần đổi import trong chính file test (nhờ barrel).
2. Đọc dòng 24,786–cuối file, copy nguyên văn 10 định nghĩa + import cần
   thiết.
3. Tạo `orca-runtime-tail-buffer.ts`, paste.
4. Sửa `orca-runtime.ts`: xoá khối 24,786→cuối, thêm dòng barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `orca-runtime.ts` giảm
      ~1,945 dòng (26,730 → ~24,785)
- [ ] Test liên quan (tail buffer / terminal wait state) pass

## Rollback

```
git checkout -- frontend/src/main/runtime/orca-runtime.ts
rm frontend/src/main/runtime/orca-runtime-tail-buffer.ts
```
