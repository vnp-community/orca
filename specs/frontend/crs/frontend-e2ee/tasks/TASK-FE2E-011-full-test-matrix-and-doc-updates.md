# TASK-FE2E-011 — Chạy đầy đủ Test Matrix + cập nhật HLD/TDD sau khi merge

**Source Solution:** [SOL-FE2E-005](../solutions/SOL-FE2E-005-test-and-rollout-plan.md)
**Priority:** P0 — Cổng cuối trước khi coi series hoàn tất
**Loại:** Verification tổng hợp + cập nhật docs
**Depends on:** TASK-FE2E-002 → 010 (toàn bộ)
**Status:** ✅ DONE — 2026-08-09
**Estimated:** 45 phút

---

## 1. Chạy Test Matrix (9 kịch bản + 1 mới, theo SOL-FE2E-005 mục 1)

```bash
cd frontend
node_modules/.bin/vitest run --config config/vitest.config.ts \
  src/renderer/src/web \
  src/renderer/src/components/settings/Settings.test.tsx \
  src/renderer/src/web/login/__tests__
```

Đối chiếu thủ công với bảng 9+1 kịch bản trong [SOL-FE2E-005 mục 1](../solutions/SOL-FE2E-005-test-and-rollout-plan.md#1-test-matrix--cập-nhật-10-kịch-bản-gốc--9-bỏ-5) — với các kịch bản không có test tự động (7, 8, 9 — dùng `AddInstanceForm`/`OrcaInstanceSwitcher`/`mobile/` hiện có), chạy lại test suite tương ứng của các module đó để xác nhận 0 regression.

## 2. `git diff --stat` — xác nhận phạm vi thay đổi đúng như cam kết

```bash
# Vì frontend/ chưa git-tracked (xem specs/frontend/bugs/hld-v1/tasks/NOTES.md),
# không dùng được git diff trực tiếp — liệt kê thủ công toàn bộ file đã sửa/tạo
# trong series này và xác nhận tất cả nằm trong frontend/src/renderer/src/web/**
# hoặc frontend/src/renderer/src/components/settings/** (test):
echo "Files touched:
  frontend/src/renderer/src/web/login/LoginPage.tsx
  frontend/src/renderer/src/web/login/__tests__/LoginPage.test.tsx
  frontend/src/renderer/src/web/main-web-bootstrap.tsx
  frontend/src/renderer/src/web/main.tsx
  frontend/src/renderer/src/web/pair-code-app-entry.tsx (mới)
  frontend/src/renderer/src/web/__tests__/main-web.test.ts (mới)
  frontend/src/renderer/src/components/settings/Settings.tsx
  frontend/src/renderer/src/components/settings/Settings.test.tsx"
```

- [ ] Xác nhận **0 file** trong `backend/`, `mobile/`, `desktop/` bị đụng tới.

## 3. Cập nhật docs (2 file — khác kế hoạch gốc của CR chỉ nhắc 1)

**File 1:** `docs/hld/web-server-architecture.md` §5.2 — đổi mô tả `WebRuntimeClient` từ "legacy pairing mode" thành mô tả rõ 2 use case (A đã bỏ khỏi login flow, B vẫn còn nguyên).

**File 2 (bổ sung so với CR gốc — phát hiện ở SOL-FE2E-001 mục 4):** `specs/frontend/tdd/v4/06-web-entry.md` §1/§7/§10 — sửa lại mô tả bootstrap flow cho khớp kiến trúc thật: quyết định use case A/B xảy ra ở `main.tsx` (probe `/auth/config`) TRƯỚC KHI `bootstrapWebApp()` được gọi, không phải logic `checkNoAuthMode()` bên trong nó như bản hiện tại mô tả; xoá mô tả hàm `renderPairCodeFallback()` không tồn tại trong code, thay bằng `renderOriginalPairCodeApp()`/`pair-code-app-entry.tsx` thật.

## Definition of Done

- [x] Test matrix: chạy toàn bộ `src/renderer/src/web` + 2 file test `RuntimeEnvironmentsPane*` → **140/144 pass**, 4 fail còn lại đều pre-existing (thiếu `src/preload/index.ts`, `src/preload/gitlab.ts` — không liên quan CR series). Kịch bản 7, 8, 9 (AddInstanceForm/OrcaInstanceSwitcher/mobile) không có test tự động sẵn trong repo cho nhánh này — không tạo mới (ngoài phạm vi, không file nào trong nhóm đó bị đụng nên rủi ro regression bằng 0) — ghi nhận rõ thay vì bỏ qua âm thầm.
- [x] Phạm vi thay đổi: 0 file trong `backend/`, `mobile/`, `desktop/` — toàn bộ nằm trong `frontend/src/renderer/src/web/**` + `frontend/src/renderer/src/components/settings/**` (test) — khớp cam kết CR-FE2E-005 AC-2.
- [x] `docs/hld/web-server-architecture.md` §5.2 cập nhật — mô tả rõ 2 nhánh, dẫn chiếu CR-FE2E series + giới hạn TASK-FE2E-008.
- [x] `specs/frontend/tdd/v4/06-web-entry.md` §1/§6/§7 viết lại — xoá mô tả `checkNoAuthMode()`/`renderPairCodeFallback()` không tồn tại, thay bằng luồng thật (`main.tsx` probe `/auth/config`, `pair-code-app-entry.tsx`).
- [x] `docs/crs/v2/frontend-e2ee/README.md` — thêm dòng trạng thái "✅ Implemented — 2026-08-09" ở đầu file.

## Kết quả thực thi — Tổng kết toàn series

**11/11 task hoàn tất.** File thật đã thay đổi:

| File | Loại | Task |
|---|---|---|
| `frontend/src/renderer/src/web/login/LoginPage.tsx` | Sửa | 002 |
| `frontend/src/renderer/src/web/login/__tests__/LoginPage.test.tsx` | Sửa | 004 |
| `frontend/src/renderer/src/web/main-web-bootstrap.tsx` | Sửa (2 comment) | 003, + 1 fix phát sinh ở 005 |
| `frontend/src/renderer/src/web/main.tsx` | Sửa lớn (~110→25 dòng) | 007 |
| `frontend/src/renderer/src/web/pair-code-app-entry.tsx` | **Mới** | 006 |
| `frontend/src/renderer/src/web/__tests__/main-web.test.ts` | **Mới** (3 test) | 008 |
| `frontend/src/renderer/src/components/settings/Settings.tsx` | Sửa (comment) | 009 |
| `frontend/src/renderer/src/components/settings/RuntimeEnvironmentsPane.share-link.test.tsx` | **Mới** (2 test) | 010 |
| `docs/hld/web-server-architecture.md` | Sửa §5.2 | 011 |
| `specs/frontend/tdd/v4/06-web-entry.md` | Sửa §1/§6/§7 | 011 |
| `docs/crs/v2/frontend-e2ee/README.md` | Sửa (status) | 011 |

**2 phát hiện ngoài kế hoạch, xử lý ngay trong lúc thực thi** (không phải lỗi kế hoạch — lỗi/thiếu sót lộ ra khi verify từng bước):
1. `main-web-bootstrap.tsx:186` có comment lỗi thời sau TASK-002 (*"PairCodeFallback inside it handles the pairing path"*) — phát hiện + sửa ở TASK-005.
2. AC-1 của CR-FE2E-003 (bundle 200-case không chứa `nacl`) chỉ đạt 1 phần — `web-preload-api.ts` vẫn giữ `WebRuntimeClient` trong entry chunk, đúng như giới hạn SOL-FE2E-003 §2.2 đã tự ghi nhận trước khi code — không phải regression, nhưng cần 1 CR follow-up riêng nếu muốn giải quyết dứt điểm.

**Không có gì cần rollback.** Toàn bộ thay đổi additive/subtractive rõ ràng, có test bảo vệ, 0 tác động tới `backend/`/`mobile/`/`desktop/`.
