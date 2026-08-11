# TASK-BIGFILE-044 — Move (composition): Linear integration domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** L (lớn nhất trong toàn bộ các domain đã tách, cả về số dòng
lẫn số method) · **Phụ thuộc:** TASK-BIGFILE-042 (xác nhận Linear là
domain tách biệt, không gộp chung GitHub/GitLab)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3, mở rộng ngoài 5 domain gốc)

## Kết quả thực thi (2026-08-10)

- Xác nhận đúng như dự đoán ở TASK-042: Linear là 1 khối **liên tục,
  tách biệt hoàn toàn**, nằm ngay trước Jira (dòng
  `// ── Linear integration ──` → `// ── Jira integration ──`), ~1,975
  dòng, **75 method**.
- **Domain sạch nhất trong tất cả các domain đã tách**: dù có 75 method,
  rà soát `this.X` cho thấy tuyệt đại đa số là tự tham chiếu lẫn nhau
  (gọi method Linear khác trong cùng class mới) — chỉ **5 dependency
  ngoài thật sự**: `getStore`, `showTerminal`, `resolveWorktreeSelector`,
  `emitClientEvent`, `listResolvedWorktrees`.
- 2 hàm module-level private (`sameStringSet`, `labelsForIds`) và 2 type
  private (`LinearAgentWriteTarget`, `LinearCreateFieldIntent`) chỉ dùng
  trong domain này — di chuyển theo, xoá khỏi `orca-runtime.ts`.
- Domain tự sinh ra 2 method generic bị bỏ sót lúc quét tên bằng regex
  (`runLinearAgentWrite<T>`, `readLinearWriteLookup<T>`) — cả 2 đều
  `private`, tự chứa, không ảnh hưởng.
- Đăng ký `config/max-lines-baseline.txt` cho file mới (2,140 dòng).
- `orca-runtime.ts`: 19,958 → **17,936 dòng** (giảm ~2,022 dòng — **lớn
  nhất trong toàn bộ các task tách domain đã làm**, kể cả TASK-037). File
  mới: 2,140 dòng.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không
  đổi (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config.

## Việc tiếp theo (không nằm trong task này)

- Jira wrapper methods (block nhỏ, ~106 dòng, nằm NGAY SAU Linear —
  ranh giới đã xác định rõ, dễ tách trong 1 task riêng rất nhanh).
