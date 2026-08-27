# TASK-BIGFILE-081 — Move: worktree-terminal-stop domain

**Loại:** Move — composition pattern, rủi ro trung bình · **Effort:** M ·
**Phụ thuộc:** không (chỉ dùng method core sẵn có)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Theo lựa chọn của người dùng ("tiếp tục nhặt candidate nhỏ còn sót"): sweep
gap-analysis chạy lại với bộ lọc chặt hơn (loại cả forwarding-field 1 dòng
lẫn nhiều dòng khỏi phép đo "dòng thật" mỗi gap — bộ lọc trước chỉ bắt được
phần lớn nhưng bỏ sót vài biến thể format) phát hiện cụm liền mạch
`stopTerminalsForWorktree`/`stopExactTerminalsForWorktree`/
`getLivePtyIdsForWorktree`/`hasTerminalsForWorktree` (~150 dòng) — cùng một
mối quan tâm "dừng/kiểm tra terminal đang sống theo worktree", chia sẻ helper
`getLivePtyIdsForWorktree`.

## Kết quả thực thi (2026-08-12)

- Domain: `stopTerminalsForWorktree` (dòng gốc 3060–3085), `stopExactTerminalsForWorktree`
  (3087–3167), `getLivePtyIdsForWorktree` (3169–3194, private, chỉ dùng nội
  bộ cụm), `hasTerminalsForWorktree` (3196–3211) — 1 đoạn liền mạch.
- 7 host dependency, toàn bộ method sẵn có: `getGraph`, `getPtyController`,
  `resolveWorktreeSelector`, `captureReadyGraphEpoch`, `assertStableReadyGraph`,
  `refreshPtyWorktreeRecordsFromController`, `getResolvedWorktreeMap`.
- Áp dụng ngay bài học TASK-073 (capture `ptyController` vào biến cục bộ để
  TS narrow qua nhiều statement) từ lúc viết file mới, không đợi `tsc` báo.
- Cả 3 method public giữ forwarding field, không có internal caller nào
  khác trong `orca-runtime.ts`.
- 1 import move-only (`setsEqual`, độc quyền cụm này) dọn sau `tsc`.
- Xác minh fidelity bằng diff nguyên văn (chuẩn hoá `this.host.X` → `this.X`,
  khôi phục lại `const ptyController` thành truy cập field trực tiếp) so với
  `git show HEAD:...` — khớp, chỉ khác 1 chỗ reformat xuống dòng cosmetic.
- `orca-runtime.ts`: 4,948 → **4,819 dòng**. File mới
  `orca-runtime-worktree-terminal-stop.ts`: 190 dòng (164 non-blank/non-comment)
  — dưới ngưỡng 300, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (1
  lỗi move-only sửa ngay). `oxlint` sạch (exit 0) cả 2 config.
  `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Rủi ro trung bình — mutate live PTY (kill/stopAndWait), không phải hot
  path tần suất cao nhưng có side effect thật (dừng process). Khuyến nghị
  kiểm thử thủ công: sleep worktree (dùng `stopTerminalsForWorktree`), pane
  hibernation exact-stop (dùng `stopExactTerminalsForWorktree` với
  `targetOnly`), race giữa 2 lệnh stop đồng thời trên cùng worktree.
- Đã nhặt 1 candidate theo lựa chọn "tiếp tục" của người dùng — còn 3-5
  candidate tương tự cỡ 30-90 dòng theo sweep gap-analysis mới nhất (ví dụ
  cụm quanh dòng 2762–2830, 3446–3512, 1868–1930 trong numbering trước khi
  tách task này) — sẽ tiếp tục đánh giá và tách nếu còn thời gian/giá trị,
  nhưng KHÔNG đủ để đưa file xuống dưới 2.000 dòng (đã cảnh báo người dùng
  trước khi làm task này).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **4,819 dòng (82.0% giảm)** qua 49 task.
