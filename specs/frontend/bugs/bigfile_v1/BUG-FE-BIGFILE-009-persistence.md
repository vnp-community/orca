# BUG-FE-BIGFILE-009 — `persistence.ts` (6,659 dòng)

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-009](./solutions/SOLUTION-FE-BIGFILE-009-persistence.md)
**Module:** `frontend/src/main/persistence.ts`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

6,659 dòng, main-process (không phải renderer/React — không có
`useState`/`useEffect`). Comment dòng 1: "persistence keeps schema defaults,
migration, ..." (bị cắt trong grep, nên đọc trực tiếp file để lấy đầy đủ lý
do).

Cấu trúc top-level:

```
337   export function initDataPath(...)
468   export function getCanonicalUserDataPath(...)
484   export function migrateMobilePairingDataToCanonicalUserDataPath(...)
1237  export function sanitizeOnboardingUpdate(...)
2649  export type StoreOptions = {...}
2653  export class Store {...}          ← từ đây tới cuối file (~4,000 dòng)
```

Giống cấu trúc `orca-runtime.ts` (BUG-FE-BIGFILE-002) ở quy mô nhỏ hơn: vài
hàm độc lập ở đầu file (path resolution, migration) rồi tới 1 class `Store`
chiếm phần lớn phần còn lại (~4,000/6,659 dòng, 60%).

## Hậu quả

- `Store` là lớp persistence trung tâm — rất có thể được import ở rất nhiều
  nơi trong `main/`; 1 class 4,000 dòng khiến việc xác định "field/method nào
  đang thực sự được dùng" khó hơn, tăng rủi ro dead code tồn đọng lâu dài mà
  không ai dám xoá vì không chắc còn dùng hay không.
- Logic migration (`migrateMobilePairingDataToCanonicalUserDataPath`,
  `sanitizeOnboardingUpdate`) và logic path resolution
  (`initDataPath`, `getCanonicalUserDataPath`) đã tách biệt rõ khỏi class
  `Store` — 2 nhóm này là ứng viên tách file rõ ràng nhất, rủi ro thấp.

## Bằng chứng

```
wc -l persistence.ts                                   → 6659
grep -n "^export function\|^export class" persistence.ts → 4 hàm + 1 class (dòng 2653)
head -1 persistence.ts                                  → "/* eslint-disable max-lines -- Why: persistence keeps schema defaults, migration, ..."
```

## Đề xuất fix

1. Tách 3 nhóm hàm độc lập ở đầu file (dòng 337–2648, trước class `Store`)
   sang các file riêng theo domain:
   - `persistence-paths.ts` (`initDataPath`, `getCanonicalUserDataPath`)
   - `persistence-migration.ts` (`migrateMobilePairingDataToCanonicalUserDataPath`,
     `sanitizeOnboardingUpdate`, và các hàm migration khác nếu có nằm giữa
     dòng 484–2648 chưa được liệt kê ở grep top-level)
2. Với class `Store` (dòng 2653 → cuối, ~4,000 dòng): xác định các nhóm
   method theo domain dữ liệu (worktree, repo, settings, onboarding, ...) —
   method liên quan tới 1 domain có thể tách sang mixin/composition pattern
   hoặc file con export riêng, tương tự cách `orca-runtime-files.ts` đã tách
   khỏi `orca-runtime.ts`.
3. Vì đây là lớp persistence trung tâm (mọi lỗi ở đây có khả năng ảnh hưởng
   dữ liệu người dùng), **bất kỳ bước tách nào cũng cần chạy đầy đủ test suite
   hiện có trước/sau, không chỉ typecheck** — ưu tiên các bước tách "di chuyển
   nguyên khối, không đổi logic" trước khi cân nhắc refactor sâu hơn.

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- File tương tự đã tách trước (main-process, class lớn): `orca-runtime.ts` →
  `orca-runtime-files.ts` (xem `BUG-FE-BIGFILE-002`)
