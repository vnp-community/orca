# TASK-FE-HLD-009 — Thêm cơ chế ngoại lệ tường minh vào `AGENTS.md`

**Solution:** [SOLUTION-FE-HLD-006](../solutions/SOLUTION-FE-HLD-006-max-lines-cleanup-plan.md)
**Bug:** [BUG-FE-HLD-006](../BUG-FE-HLD-006-max-lines-disable-agents-md-violation.md)
**File:** `AGENTS.md`
**Estimated:** 10 phút
**Status:** ✅ DONE — 2026-08-09
**Phụ thuộc:** TASK-FE-HLD-007 (baseline phải tồn tại trước khi mô tả cơ chế trỏ tới nó)

---

## Mục tiêu

Bổ sung câu mô tả cơ chế ngoại lệ có kiểm soát (qua `config/max-lines-baseline.txt`) vào mục "Lint Rules: Do Not Disable Max Lines" trong `AGENTS.md`, để chính sách "never" vẫn giữ đúng tinh thần (không cho inline disable tự do) nhưng có 1 lối thoát tập trung, dễ audit.

---

## Context

```bash
grep -n "max-lines" -B2 -A5 AGENTS.md
```

---

## Thay đổi cần thực hiện

**File:** `AGENTS.md`

**TÌM** (mục "Lint Rules: Do Not Disable Max Lines"):
```
Never add a `max-lines` disable (`eslint-disable max-lines`, `oxlint-disable max-lines`, or line-specific variants), and never add a per-file `max-lines` bump in `mobile/.oxlintrc.json`.
```

**THAY BẰNG** (giữ nguyên câu gốc, thêm đoạn mới ngay sau):
```
Never add a `max-lines` disable (`eslint-disable max-lines`, `oxlint-disable max-lines`, or line-specific variants), and never add a per-file `max-lines` bump in `mobile/.oxlintrc.json`.

Ngoại lệ phải qua `config/max-lines-baseline.txt` (được review + có lý do ghi trong PR description), không được thêm bằng inline disable comment. `config/scripts/check-max-lines-ratchet.mjs` chạy trong CI để chặn file mới thêm inline disable ngoài baseline này.
```

---

## Verify

```bash
grep -n "max-lines-baseline.txt\|check-max-lines-ratchet" AGENTS.md
# Xác nhận câu mới đã thêm, trỏ đúng tên file thật đã tạo ở TASK-FE-HLD-007
```

---

## Definition of Done

- [x] `AGENTS.md` giữ nguyên câu "never" gốc, thêm đúng 1 đoạn mô tả cơ chế ngoại lệ
- [x] Tên file khớp chính xác với file thật đã khôi phục ở TASK-FE-HLD-007 (`config/max-lines-baseline.txt`, `config/scripts/check-max-lines-ratchet.mjs`), thêm cả tên script `pnpm check:max-lines-ratchet` cho rõ cách chạy
- [x] Không có thay đổi nào khác trong file
