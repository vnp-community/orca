# TASK-BIGFILE-079 — Extract: post-class const/helper cluster

**Loại:** Extract — cùng kỹ thuật TASK-BIGFILE-078, áp dụng cho vùng SAU class
thay vì trước · Rủi ro thấp · **Effort:** S · **Phụ thuộc:** TASK-BIGFILE-078
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Theo gợi ý để lại ở TASK-078: vùng SAU class `OrcaRuntimeService` (3 const
`DEFAULT_WORKTREE_LIST_LIMIT`/`DISCONNECTED_PTY_RECORD_MAX`/
`PTY_CONTROLLER_LIST_TIMEOUT_MS`, hàm `getExplicitWorktreeIdSelector`,
`withTimeout`, `withTimeoutResult`) cũng không có `this` — cùng kỹ thuật
Extract, thêm trực tiếp vào `orca-runtime-service-types.ts` đã có sẵn từ
TASK-078 thay vì tạo file mới (tránh phân mảnh thêm cho một cụm nhỏ).

## Kết quả thực thi (2026-08-12)

- Domain: dòng gốc 5033–5072 (3 const, `getExplicitWorktreeIdSelector`,
  `withTimeout`, `withTimeoutResult`) — chuyển sang cuối
  `orca-runtime-service-types.ts`. 2 block re-export cuối file (tail-buffer,
  orca-runtime-types) giữ nguyên tại chỗ — đã là re-export thuần, không có
  gì để rút gọn thêm.
- Chỉ `withTimeout` được sibling (`orca-runtime-resolved-worktree-cache.ts`)
  dùng qua `from './orca-runtime'` — thêm vào khối `export {...}`. 5 cái còn
  lại chỉ dùng nội bộ class body.
- 1 lỗi `tsc` move-only: `withTimeout` import cục bộ dư thừa (class body chỉ
  gọi `withTimeoutResult`, không gọi `withTimeout` trực tiếp) — bỏ khỏi
  import cục bộ, giữ trong export.
- `orca-runtime.ts`: 5,108 → **5,074 dòng**. File
  `orca-runtime-service-types.ts`: 767 dòng (658 non-blank/non-comment) —
  đã đăng ký baseline từ TASK-078, không cần thêm entry mới.
- Xác minh: `tsc` baseline 251 không đổi (1 lỗi move-only sửa ngay). `oxlint`
  sạch cả 2 config. `max-lines-ratchet`: 647 không đổi.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **5,074 dòng (81.0% giảm)** qua 47 task.
