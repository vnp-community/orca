# TASK-FE-HLD-008 — Gắn ratchet check vào `package.json`/CI

**Solution:** [SOLUTION-FE-HLD-006](../solutions/SOLUTION-FE-HLD-006-max-lines-cleanup-plan.md)
**Bug:** [BUG-FE-HLD-006](../BUG-FE-HLD-006-max-lines-disable-agents-md-violation.md)
**File:** `package.json`, workflow CI (`.github/workflows/*.yml` — xác nhận tên file thật trước khi sửa)
**Estimated:** 15 phút
**Status:** ✅ DONE (đã có sẵn, chỉ xác nhận) — 2026-08-09
**Phụ thuộc:** TASK-FE-HLD-007

---

## Mục tiêu

Thêm script `check:max-lines-ratchet` vào `package.json` gốc, gắn vào pipeline CI hiện có để mọi PR mới đều tự động chạy check này.

---

## Context

```bash
grep -n '"scripts"' -A 30 package.json | head -40
ls .github/workflows/ 2>/dev/null
grep -rn "check-reliability-gates\|check-max-lines\|check-styled-scrollbars" .github/workflows/ 2>/dev/null
# Các check tương tự (check-reliability-gates.mjs, check-styled-scrollbars.mjs)
# đã có sẵn trong config/scripts/ theo lịch sử repo — dùng làm mẫu cách chúng
# được wire vào cả package.json lẫn CI workflow.
```

---

## Thay đổi cần thực hiện

**File:** `package.json`

Thêm vào `"scripts"` (đặt cạnh các script `check:*` khác nếu có):
```json
"check:max-lines-ratchet": "node config/scripts/check-max-lines-ratchet.mjs"
```

**File CI** (tên chính xác xác nhận qua `ls .github/workflows/`): thêm 1 step vào job lint/check hiện có:
```yaml
- name: Check max-lines ratchet
  run: pnpm check:max-lines-ratchet
```

Đặt cạnh (không thay thế) các step lint/check khác đã có, theo đúng vị trí quy ước của CI hiện tại.

---

## Verify

```bash
pnpm check:max-lines-ratchet
# Chạy được từ package.json script, kết quả giống chạy trực tiếp node ở TASK-FE-HLD-007

# Nếu có công cụ chạy CI local (act, hoặc pipeline dry-run), xác nhận step mới
# không phá vỡ job hiện có.
```

---

## Definition of Done

- [x] `pnpm check:max-lines-ratchet` chạy được từ root, kết quả khớp TASK-FE-HLD-007
- [x] Đã có sẵn trong job lint (`package.json:14`, script `"lint"` gọi `... && pnpm run check:max-lines-ratchet && ...`) — **không cần thêm gì**, khôi phục lại file ở TASK-FE-HLD-007 là đủ để wiring hoạt động lại
- [x] Không phá vỡ gì — không sửa `package.json`/CI workflow trong task này (đã đúng sẵn)

## Kết quả thực thi

Phát hiện: root `package.json` **chưa từng mất phần wiring** — chỉ có 3 file thực thi (`check-max-lines-ratchet.mjs`, `.test.mjs`, `max-lines-baseline.txt`) bị xoá trong đợt tái cấu trúc, còn dòng script `"check:max-lines-ratchet"` và lời gọi trong `"lint"` vẫn nguyên. Sau khi khôi phục file ở TASK-FE-HLD-007, wiring này tự động hoạt động lại — không cần sửa `package.json` hay CI workflow nào thêm. Task này chỉ còn vai trò xác nhận.

```bash
pnpm check:max-lines-ratchet
# → max-lines ratchet OK — 22 grandfathered suppression(s), no new bypasses.
```
