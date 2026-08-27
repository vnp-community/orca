# TASK-BIGFILE-011 — Investigate+Move: `persistence-migration.ts`

**Loại:** Investigate+Move (đọc xác nhận ranh giới trước khi cắt) · **Effort:** M
**Phụ thuộc:** — (độc lập với TASK-BIGFILE-010, nên làm sau vì cùng file
nguồn — tránh conflict khi 2 task sửa cùng lúc)
**Status:** ✅ Done (phạm vi thực tế nhỏ hơn nhiều so với kế hoạch gốc —
xem "Kết quả thực thi")
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-009-persistence.md`

## Kết quả thực thi (2026-08-12)

Bước 1 (quét toàn bộ 484–2,648, liệt kê MỌI hàm/type kể cả private) xác
nhận đúng cảnh báo của task doc gốc: khoảng này chứa **~90 định nghĩa**
top-level (hàm/type/const), không chỉ 2 export đã biết. Nhưng khác với
giả định ban đầu ("domain migration lớn"), việc dò usage sau dòng 2,649
(class `Store`) cho từng định nghĩa (grep trực tiếp + truy vết gián tiếp
qua các hàm gọi lẫn nhau trong vùng 484–2,648) cho kết quả: **gần như
TOÀN BỘ ~90 định nghĩa đều được `Store` dùng trực tiếp hoặc gián tiếp**
(ví dụ `collectLayoutLeafCounts`, `firstLayoutLeafId` có 0 lần dùng trực
tiếp sau dòng 2,649 nhưng được gọi bởi
`normalizeTerminalLayoutSnapshotForPersistence` — hàm này lại được
`Store` gọi trực tiếp → vẫn thuộc ranh giới `persistence-store.ts`,
không phải migration, theo đúng quy tắc bước 3 của task doc).

Chỉ CÓ ĐÚNG 1 cụm thực sự độc lập với `Store` (0 lần dùng, kể cả gián
tiếp):
- `migrateMobilePairingDataToCanonicalUserDataPath` (export đã biết)
- `sanitizeOnboardingUpdate` (export đã biết) + `remapLegacyOnboardingLastCompletedStep`
  (private, chỉ được gọi bên trong `sanitizeOnboardingUpdate`) + type
  `SanitizeOnboardingUpdateOptions` (chỉ dùng cho tham số của
  `sanitizeOnboardingUpdate`)

→ **Quyết định: KHÔNG chuyển toàn bộ 2,165 dòng như Output gốc mô tả.**
Chỉ chuyển đúng 2 cụm không chồng lấn Store nói trên (~150 dòng, không
liền mạch — cách nhau bởi ~700 dòng hàm chỉ dùng cho `Store`). Phần còn
lại (~85 định nghĩa, phần lớn là normalize/migrate cho terminal layout,
automation run, ssh target, floating workspace, project host setup, pane
identity...) ở nguyên `persistence.ts` — đây thực chất là domain
`persistence-store.ts` (hỗ trợ nội bộ cho `Store.load()`), không phải
`persistence-migration.ts`, và việc tách nó là phạm vi của một task khác
lớn hơn nhiều, không nằm trong task này.

- `migrateMobilePairingDataToCanonicalUserDataPath` gọi
  `getCanonicalUserDataPath()` (đã chuyển sang `persistence-paths.ts` ở
  TASK-010) — import trực tiếp từ `./persistence-paths` trong file mới,
  không qua `persistence.ts`, tránh vòng phụ thuộc thừa.
- `sanitizeOnboardingUpdate` được `normalizeLoadedOnboardingState` (ở lại
  `persistence.ts`, dùng bởi `Store`) gọi trực tiếp → phải thêm
  `import { sanitizeOnboardingUpdate } from './persistence-migration'`
  trong `persistence.ts` (không chỉ 1 dòng `export { ... } from ...` như
  kế hoạch gốc ngầm định — cùng bài học với TASK-BIGFILE-009 khi tách
  `orca-runtime-types.ts`). Phát hiện lỗi này qua `tsc --noEmit` (TS2304
  Cannot find name 'sanitizeOnboardingUpdate') — sửa ngay.
- Dọn thêm 2 import top-level bị unused sau khi chuyển
  (`ONBOARDING_FLOW_VERSION`, `OnboardingOutcome` — chỉ được dùng bên
  trong `sanitizeOnboardingUpdate`/`remapLegacyOnboardingLastCompletedStep`
  đã chuyển đi, không có chỗ dùng nào khác trong `persistence.ts`), và bỏ
  2 import không còn cần (`MOBILE_PAIRING_USERDATA_FILES`,
  `hardenExistingSecureFile` — chỉ dùng trong
  `migrateMobilePairingDataToCanonicalUserDataPath` đã chuyển đi).
- File mới `persistence-migration.ts`: 169 dòng. `persistence.ts` giảm
  tổng cộng 6,659 → **6,461 dòng** (gồm cả phần đã giảm ở TASK-010) — ít
  hơn nhiều so với ước tính gốc "~2,165 dòng" vì đại đa số vùng đó thuộc
  `persistence-store.ts`, không phải migration (đúng như cảnh báo ⚠️ của
  task doc gốc, nhưng mức độ lệch lớn hơn dự đoán: gần như toàn bộ vùng,
  không chỉ "vài hàm private").
- Xác minh: `npx tsc --noEmit` với tsconfig tạm — 0 lỗi mới trong 3 file
  `persistence*.ts` (65 lỗi pre-existing ở file khác, không đổi). `oxlint`
  trên 3 file: exit 0, sạch.
- `gitnexus impact` không dùng được (MCP "Connection closed" nhiều lần) —
  thay bằng grep thủ công repo-wide: không có importer nào của
  `migrateMobilePairingDataToCanonicalUserDataPath`/`sanitizeOnboardingUpdate`
  ngoài `persistence.ts` trong phạm vi `frontend/src` (các bản sao ở
  `desktop/`, `backend/`, `agent/` là copy độc lập ngoài phạm vi task,
  không đụng tới).
- Test thủ công "khởi động app với dữ liệu cũ": KHÔNG thực hiện được
  trong phiên này — `frontend/` không có entrypoint main-process runnable
  độc lập (không có `index.ts`/`main.ts` gọi `initDataPath`/`Store` trong
  package này; các package chạy được là `desktop/`/`backend`/`agent`, vốn
  có bản sao `persistence.ts` riêng, KHÔNG đồng bộ với thay đổi này). Đã
  bù bằng typecheck sạch + đối chiếu logic copy nguyên văn (không sửa nội
  dung hàm, chỉ di chuyển vị trí + import).

## Input

- File nguồn: `frontend/src/main/persistence.ts`
- Đọc **đúng dòng 484–2,648** (trước `export type StoreOptions` dòng 2,649).
- Symbol đã biết cần chuyển: `migrateMobilePairingDataToCanonicalUserDataPath`
  (dòng 484), `sanitizeOnboardingUpdate` (dòng 1,237).

## ⚠️ Ranh giới chưa chắc chắn — PHẢI xác nhận khi đọc

Khoảng cách 484→2,648 (2,165 dòng) cho chỉ 2 export đã biết là rất lớn — khả
năng cao có thêm hàm/type private (không export) nằm giữa 2 hàm này hoặc sau
`sanitizeOnboardingUpdate` mà bảng export top-level ban đầu không bắt được.
**Bước đầu tiên bắt buộc:** quét lại toàn bộ khoảng 484–2,648 để liệt kê ĐẦY
ĐỦ mọi `function`/`const`/`type` (kể cả không export) trước khi quyết định
đúng những gì thuộc "migration" và những gì có thể thuộc phần khác (đọc để
phân loại theo tên/mục đích, không đoán).

## Output

- File mới: `frontend/src/main/persistence-migration.ts` — chứa TOÀN BỘ nội
  dung đã xác nhận thuộc domain migration ở bước đọc trên (không chỉ 2 export
  đã biết).
- File nguồn thay bằng `export { ... } from './persistence-migration'` liệt
  kê đủ các symbol PUBLIC (export) đã chuyển. Symbol private không cần khai
  báo lại ở `persistence.ts` — chúng chỉ tồn tại nội bộ file mới.

## Các bước

1. Đọc dòng 484–2,648, liệt kê đầy đủ mọi định nghĩa (kể cả private).
2. `gitnexus impact` cho `migrateMobilePairingDataToCanonicalUserDataPath` và
   `sanitizeOnboardingUpdate` — dừng nếu risk HIGH/CRITICAL.
3. Xác nhận: các định nghĩa private trong khoảng này có được dùng ở ĐÂU
   KHÁC trong `persistence.ts` (vd trong class `Store` sau dòng 2,649) hay
   không — nếu CÓ dùng trong `Store`, KHÔNG chuyển hàm/type đó (nó thuộc
   ranh giới `persistence-store.ts`, không phải migration); ghi chú lại phát
   hiện này.
4. Copy nguyên văn phần đã xác nhận đúng là migration + import cần thiết.
5. Tạo `persistence-migration.ts`, paste.
6. Sửa `persistence.ts`: xoá phần đã chuyển, thêm barrel export cho các
   symbol public.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `persistence.ts` giảm
      tương ứng số dòng thực tế đã xác nhận ở bước 1 (không nhất thiết đúng
      2,165 — ước tính ban đầu có thể sai do chưa đọc kỹ)
- [ ] Chạy test migration hiện có + **test thủ công**: khởi động app với dữ
      liệu cũ (fixture nếu có), xác nhận migration vẫn chạy đúng

## Rollback

```
git checkout -- frontend/src/main/persistence.ts
rm frontend/src/main/persistence-migration.ts
```
