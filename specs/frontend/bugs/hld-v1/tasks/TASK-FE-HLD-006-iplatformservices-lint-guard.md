# TASK-FE-HLD-006 — Thêm lint rule chặn `import 'electron'` cho 5 module v5.0

**Solution:** [SOLUTION-FE-HLD-005](../solutions/SOLUTION-FE-HLD-005-iplatformservices-scope.md)
**Bug:** [BUG-FE-HLD-005](../BUG-FE-HLD-005-iplatformservices-electron-adapter-missing.md)
**File:** `frontend/.oxlintrc.json` (hoặc eslint override tương ứng đang dùng thật trong repo)
**Estimated:** 20 phút
**Status:** ✅ DONE — 2026-08-09
**Phụ thuộc:** TASK-FE-HLD-005

---

## Mục tiêu

Thêm 1 lint rule tự động giữ conformance cho phạm vi đã làm rõ ở TASK-FE-HLD-005 — chặn `import 'electron'` trong 5 thư mục v5.0, tránh phải audit thủ công lần sau.

---

## Context

```bash
cat frontend/.oxlintrc.json 2>/dev/null | head -30
# hoặc nếu dùng eslint:
find frontend -maxdepth 1 -iname ".eslintrc*"

ls frontend/src/main | grep -E "^(profile|project|ai-providers|workflow|task)$"
# Xác nhận đúng 5 thư mục tồn tại trước khi viết override path
```

---

## Thay đổi cần thực hiện

**Nếu dùng oxlint** (`.oxlintrc.json`):
```jsonc
{
  "overrides": [
    {
      "files": ["src/main/{profile,project,ai-providers,workflow,task}/**/*.ts"],
      "rules": { "no-restricted-imports": ["error", { "paths": ["electron"] }] }
    }
  ]
}
```

**Nếu dùng eslint** (cấu hình flat config hoặc `.eslintrc.cjs` tương đương), thêm block override tương tự với cú pháp eslint chuẩn.

> [!IMPORTANT]
> Nếu `src/main/ai-providers` không tồn tại ở `frontend/` (đã xác nhận trong audit trước — module này chỉ có ở `backend/src/main/ai-providers`), **bỏ `ai-providers` khỏi glob path** cho phần cấu hình `frontend/`, không tạo path trỏ tới thư mục không tồn tại (oxlint có thể warning hoặc no-op im lặng, cả 2 đều không mong muốn).

---

## Verify

```bash
cd frontend
pnpm lint 2>&1 | tail -30
# Không có lỗi mới phát sinh từ 4-5 module này (đã xác nhận sạch trong audit)

# Test rule hoạt động: tạm thêm 1 import electron giả vào 1 file trong profile/,
# chạy lint, xác nhận báo lỗi, rồi revert lại:
echo "import { app } from 'electron'" >> src/main/profile/profile-service.ts
pnpm lint 2>&1 | grep -i "no-restricted-imports\|electron"
git checkout -- src/main/profile/profile-service.ts
```

---

## Definition of Done

- [x] Override rule thêm đúng cú pháp cho tool lint thật đang dùng trong repo — repo dùng **oxlint**, config thật nằm ở **root** `.oxlintrc.json` (không phải `frontend/.oxlintrc.json` — file đó không tồn tại; `frontend/` chưa có config lint riêng, thừa hưởng config root)
- [x] Test thủ công xác nhận rule bắt được: thêm tạm `import { app } from 'electron'` vào `OrcaProfile.ts` → `oxlint` báo đúng `error eslint(no-restricted-imports): 'electron' import is restricted from being used.` → đã revert lại file
- [x] Path glob chỉ gồm 4 thư mục thực sự tồn tại (`profile`, `project`, `workflow`, `task`) — **không thêm `ai-providers`** vì không tồn tại ở `frontend/src/main` (đúng theo lưu ý trong solution)
- [~] `pnpm lint` sạch trên 4 module — **không xác nhận được ở mức "toàn bộ lint sạch"**: chạy `oxlint` trực tiếp lên `frontend/src/main/profile` phát hiện hàng loạt lỗi **có sẵn từ trước**, không liên quan rule mới (`consistent-type-definitions`, `curly`, `no-useless-fallback-in-spread`...) — dấu hiệu `frontend/` chưa từng được lint đầy đủ kể từ khi tách khỏi monorepo. Đây là nợ kỹ thuật có sẵn, ngoài phạm vi task này — không sửa. Chỉ xác nhận riêng rule `no-restricted-imports` hoạt động đúng (không có false positive nào trong 4 module khi chưa cố tình thêm electron import).

## Kết quả thực thi

- **File sửa:** `.oxlintrc.json` (root) — thêm override đầu tiên trong mảng `overrides`, áp `no-restricted-imports` chặn `electron` cho 4 thư mục v5.0 của `frontend/`.
- **Phát hiện phụ (ghi nhận, không sửa trong task này):** `frontend/src/main` hiện có rất nhiều vi phạm lint có sẵn (không phải `no-restricted-imports`) chưa từng được rà soát — nằm ngoài phạm vi BUG-FE-HLD-005/006.
