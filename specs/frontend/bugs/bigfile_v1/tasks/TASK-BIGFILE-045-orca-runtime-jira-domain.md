# TASK-BIGFILE-045 — Move: Jira integration domain

**Loại:** Move — composition, nhưng KHÔNG cần host interface (0
dependency `this`) · **Effort:** S · **Phụ thuộc:** TASK-BIGFILE-044
(ranh giới xác định ngay sau khi tách xong Linear)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3, mở rộng ngoài 5 domain gốc)

## Kết quả thực thi (2026-08-10)

- Domain nhỏ nhất, đơn giản nhất trong tất cả: 19 method, **0
  dependency `this`** — toàn bộ chỉ là wrapper mỏng gọi thẳng hàm ngoài
  (`../jira/client`, `../jira/issues`). Không cần host interface, không
  cần constructor argument.
- `orca-runtime.ts`: 17,936 → **17,825 dòng** (giảm ~111 dòng). File
  mới: 145 dòng — nhỏ, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không
  đổi (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config.

## Tổng kết cụm domain Linear/Jira/GitHub (TASK-042, 043, 044, 045)

Kế hoạch gốc ở TASK-035 ước tính "Linear/Jira/GitHub gộp chung ~2,200
dòng" — thực tế sau khi tách đủ 4 domain: GitHub/GitLab (~1,040 dòng) +
repo-hooks (~290 dòng) + Linear (~1,975 dòng) + Jira (~110 dòng) =
**~3,415 dòng**, gần gấp đôi ước tính ban đầu. Bài học: ước tính theo cụm
từ khoá ở giai đoạn thiết kế chỉ nên dùng để ưu tiên thứ tự, không dùng
để ước lượng khối lượng công việc thực tế.
