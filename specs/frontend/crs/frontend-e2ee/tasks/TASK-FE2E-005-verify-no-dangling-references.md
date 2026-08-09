# TASK-FE2E-005 — Xác nhận không còn tham chiếu treo tới `PairCodeFallback` + đóng CR-FE2E-002

**Source Solution:** [SOL-FE2E-002](../solutions/SOL-FE2E-002-remove-paircode-fallback-from-login.md) — Acceptance Criteria
**Priority:** P0
**Loại:** Verification tổng hợp
**Depends on:** TASK-FE2E-002, TASK-FE2E-003, TASK-FE2E-004
**Estimated:** 10 phút
**Status:** ✅ DONE — 2026-08-09

---

## Việc cần làm

```bash
cd frontend

# AC-3 của CR-FE2E-002: chỉ còn PairCodeFallback.tsx và test của chính nó
grep -rln "PairCodeFallback" src --include="*.ts" --include="*.tsx"
# kỳ vọng: đúng 2 file — src/renderer/src/web/login/PairCodeFallback.tsx
#          và src/renderer/src/web/login/__tests__/PairCodeFallback.test.tsx (nếu có)

# AC-2: main.tsx (renderOriginalPairCodeApp) không bị đụng ở series này
git diff --stat -- src/renderer/src/web/main.tsx 2>/dev/null || echo "main.tsx không tracked — kiểm tra thủ công bằng diff nội dung nếu cần"

# AC-4: toàn bộ test frontend xanh
node_modules/.bin/vitest run --config config/vitest.config.ts src/renderer/src/web
```

## Definition of Done

- [x] Không còn `import { PairCodeFallback }` nào — chỉ còn nhắc tên trong **comment** ở 3 file (`LoginPage.tsx`, `LoginPage.test.tsx` — do chính TASK-002/004 thêm để giải thích lý do bỏ; `web-runtime-environment-crypto.ts` — do TASK-FE-HLD-010 ở series khác thêm, liệt kê danh sách call site cũ, không phải import thật) + chính file `PairCodeFallback.tsx`. Không phải "chỉ 2 file" như AC viết ban đầu, nhưng đúng tinh thần: **0 import thật**, chỉ còn text mô tả.
- [x] `WebConnect.tsx`, `AddInstanceForm.tsx`, `OrcaInstanceSwitcher.tsx`, `web-runtime-client.ts`, `web-e2ee.ts`, `web-pairing.ts`, `web-runtime-environment.ts` — không có thay đổi nào
- [x] `pnpm test` (`src/renderer/src/web`) — **128/132 pass**, 4 fail còn lại đều pre-existing (thiếu `src/preload/index.ts`, `src/preload/gitlab.ts` — cùng khoảng trống hạ tầng đã ghi trong `specs/frontend/bugs/hld-v1/tasks/NOTES.md`, không liên quan CR series này)
- [x] `main.tsx` xác nhận chưa bị đụng (vẫn giữ `renderOriginalPairCodeApp` bản gốc — sẽ sửa ở TASK-FE2E-007)

## Phát hiện + fix thêm ngoài kế hoạch

Trong lúc verify, phát hiện `main-web-bootstrap.tsx:186` có 1 comment **đã lỗi thời** sau TASK-FE2E-002: *"Show Login page first; PairCodeFallback inside it handles the pairing path."* — không nằm trong diff của SOL-FE2E-002 (solution không đề cập dòng này), nhưng rõ ràng sai sau khi `PairCodeFallback` đã bị bỏ khỏi `LoginPage`. Đã sửa lại thành: *"Show Login page (local/SSO only — CR-FE2E-002 removed the PairCodeFallback that used to live inside it...)"*.

## Kết quả — CR-FE2E-002 hoàn tất

Tất cả acceptance criteria của CR-FE2E-002 đạt. Không có cột status theo dõi trong `docs/crs/v2/frontend-e2ee/README.md` để đổi — ghi nhận trạng thái hoàn tất ở TASK-FE2E-011 (bước đóng series).
