# SOLUTION-FE-BIGFILE-009 — Tách `persistence.ts` (6,659 dòng)

**Bug:** `../BUG-FE-BIGFILE-009-persistence.md`
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #3 — xem `SOLUTION-FE-BIGFILE-001` mục 3
**Chiến lược:** Barrel/facade (xem `SOLUTION-FE-BIGFILE-001` mục 1)

---

## Cấu trúc file mới

| File mới | Nội dung | Dòng gốc | Ước tính dòng |
|---|---|---|---|
| `persistence-paths.ts` | `initDataPath`, `getCanonicalUserDataPath` | 337–483 | ~150 |
| `persistence-migration.ts` | `migrateMobilePairingDataToCanonicalUserDataPath`, `sanitizeOnboardingUpdate` | 484–2,648 | ~2,165 (**cần xác nhận lại — khoảng cách rất lớn, có thể còn hàm khác chưa liệt kê trong export top-level, vd hàm private không export nhưng chỉ dùng nội bộ nhóm migration; đọc kỹ trước khi cắt**) |
| `persistence-store.ts` | `type StoreOptions`, `class Store` | 2,649–cuối | ~4,010 |
| `persistence.ts` (giữ nguyên tên) | Chỉ còn `export { ... } from ...` | — | ~15 |

## Các bước thực hiện

1. **Đọc lại đầy đủ comment dòng 1–3** trước khi bắt đầu:
   > "persistence keeps schema defaults, migration, load/save, and flush
   > logic in one file so the full storage contract is reviewable as a unit
   > instead of being scattered across modules."

   Lưu ý cụm "reviewable as a unit" — lý do giữ chung có thể liên quan tới
   việc audit "storage contract" (schema + migration + load/save phải khớp
   nhau). Việc tách file KHÔNG được phá vỡ tính nhất quán này — 1 lựa chọn an
   toàn: giữ `persistence-migration.ts` và `persistence-store.ts` import lẫn
   nhau tường minh (không giấu qua barrel), để reviewer vẫn thấy rõ quan hệ
   phụ thuộc giữa schema/migration/store khi review riêng từng file.
2. `gitnexus impact({target: "Store", direction: "upstream"})` — class `Store`
   gần như chắc chắn có impact rất lớn (persistence trung tâm). Đọc kỹ danh
   sách affected_processes trước khi quyết định bước tiếp theo — **nếu risk
   trả về HIGH/CRITICAL theo đúng yêu cầu CLAUDE.md, PHẢI cảnh báo và dừng lại
   xin xác nhận trước khi tiếp tục.**
3. Tách `persistence-paths.ts` trước (rủi ro thấp nhất, không phụ thuộc
   migration/Store).
4. Tách `persistence-migration.ts` — đọc kỹ để xác nhận ranh giới dòng
   484–2,648 chính xác (khoảng cách lớn giữa 2 export gợi ý có thể có nhiều
   hàm private/type trung gian chưa được liệt kê ở bảng export top-level ban
   đầu — quét lại `grep -n "^function \|^const \|^type " persistence.ts`
   trong khoảng này trước khi cắt).
5. Tách `persistence-store.ts` — copy `StoreOptions` + `class Store` nguyên
   văn.
6. `persistence.ts` chỉ còn 3 dòng `export { ... } from ...`.

## Xác minh

- `pnpm run typecheck`, `pnpm run lint`
- **Chạy toàn bộ test liên quan persistence/store/migration** — đây là lớp
  ảnh hưởng trực tiếp dữ liệu người dùng, không chỉ typecheck là đủ.
- `gitnexus detect_changes({scope: "all"})`
- Test thủ công (nếu có thể): khởi động app với dữ liệu cũ, xác nhận migration
  vẫn chạy đúng sau khi tách.
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

**Trung bình-cao** — không phải vì logic phức tạp (đa số là di chuyển nguyên
khối), mà vì đây là lớp persistence trung tâm: 1 lỗi nhỏ khi tách (vd quên
export 1 field, thứ tự import bị đảo khiến side-effect chạy sai thời điểm) có
thể gây mất/hỏng dữ liệu người dùng thay vì chỉ lỗi hiển thị UI. Làm từng file
con một, xác nhận xanh hoàn toàn trước khi sang file tiếp theo — không gộp
bước 3+4+5 trong 1 commit.
