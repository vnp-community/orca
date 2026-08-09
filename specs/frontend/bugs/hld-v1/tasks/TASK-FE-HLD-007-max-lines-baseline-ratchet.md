# TASK-FE-HLD-007 — Sinh baseline + script ratchet cho `max-lines` disable

**Solution:** [SOLUTION-FE-HLD-006](../solutions/SOLUTION-FE-HLD-006-max-lines-cleanup-plan.md)
**Bug:** [BUG-FE-HLD-006](../BUG-FE-HLD-006-max-lines-disable-agents-md-violation.md)
**File:** `config/max-lines-baseline.txt` (mới), `config/scripts/check-max-lines-ratchet.mjs` (mới)
**Estimated:** 30 phút
**Status:** ⚠️ DONE (khác kế hoạch gốc) — 2026-08-09

---

## Mục tiêu

Ghi lại toàn bộ 240 file hiện đang disable `max-lines` thành 1 baseline cố định, và viết script CI fail nếu có file **mới** disable rule này mà không nằm trong baseline — chặn nợ kỹ thuật tăng thêm ngay, không cần chờ dọn xong 240 file cũ.

---

## Context

```bash
ls config/ | grep -i "max-lines\|ratchet"
# Kiểm tra xem config/max-lines-baseline.txt hoặc check-max-lines-ratchet.mjs
# đã từng tồn tại trong lịch sử repo chưa (đề cập trong audit trước) — nếu có
# sẵn cấu trúc cũ, khôi phục/cập nhật thay vì viết mới từ đầu.

git log --all --oneline -- config/max-lines-baseline.txt config/scripts/check-max-lines-ratchet.mjs
```

---

## Thay đổi cần thực hiện

### Bước 1 — Sinh baseline

```bash
grep -rlE "(eslint|oxlint)-disable[^\n]*max-lines" frontend/src --include="*.ts" --include="*.tsx" \
  | sed 's|^|frontend/|' \
  | sort > config/max-lines-baseline.txt

wc -l config/max-lines-baseline.txt
# Kỳ vọng: ~240 dòng (khớp con số trong audit/frontend/04-code-health-and-standards.md §1)
```

### Bước 2 — Viết script ratchet

**File mới:** `config/scripts/check-max-lines-ratchet.mjs`

```js
#!/usr/bin/env node
// Why: AGENTS.md forbids max-lines disables outright, but 240 pre-existing
// disables can't be cleaned up in one PR. This ratchet freezes the baseline —
// any file NOT already in it that adds a max-lines disable fails CI.
import { readFileSync } from 'node:fs'
import { globSync } from 'glob'

const BASELINE_PATH = 'config/max-lines-baseline.txt'

const baseline = new Set(
  readFileSync(BASELINE_PATH, 'utf-8')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
)

const disablePattern = /(?:eslint|oxlint)-disable[^\n]*max-lines/

const candidates = globSync('frontend/src/**/*.{ts,tsx}', { nodir: true })
const violators = candidates.filter((file) => disablePattern.test(readFileSync(file, 'utf-8')))

const newViolators = violators.filter((f) => !baseline.has(f))
const resolvedInBaseline = [...baseline].filter((f) => !violators.includes(f))

if (newViolators.length > 0) {
  console.error('❌ New max-lines disable(s) not in baseline (forbidden by AGENTS.md:15):')
  newViolators.forEach((f) => console.error(`   ${f}`))
  console.error('\nEither remove the disable, or (if truly unavoidable) add the file to')
  console.error(`${BASELINE_PATH} with a reviewed justification in the PR description.`)
  process.exit(1)
}

if (resolvedInBaseline.length > 0) {
  console.log('ℹ️  Files no longer disabling max-lines (safe to remove from baseline):')
  resolvedInBaseline.forEach((f) => console.log(`   ${f}`))
}

console.log(`✅ max-lines ratchet OK — ${violators.length}/${baseline.size} baseline entries still present, 0 new violations.`)
```

Đảm bảo `glob` là dependency đã có sẵn trong `package.json` root (kiểm tra trước khi thêm mới):
```bash
grep -n '"glob"' package.json
```

---

## Verify

```bash
node config/scripts/check-max-lines-ratchet.mjs
# Kỳ vọng: "✅ max-lines ratchet OK — 240/240 baseline entries still present, 0 new violations."

# Test rule hoạt động: thêm 1 disable giả vào 1 file KHÔNG có trong baseline, chạy lại:
echo "/* eslint-disable max-lines */" | cat - frontend/src/renderer/src/lib/utils.ts > /tmp/test.ts && mv /tmp/test.ts frontend/src/renderer/src/lib/utils.ts
node config/scripts/check-max-lines-ratchet.mjs
# Kỳ vọng: exit code 1, báo đúng file vừa thêm
git checkout -- frontend/src/renderer/src/lib/utils.ts
```

---

## Definition of Done

- [x] `config/scripts/check-max-lines-ratchet.mjs` chạy được bằng `node`, không cần build step
- [x] Script fail đúng (exit 1) khi có file mới ngoài baseline disable `max-lines` — hành vi có sẵn trong script gốc, không cần viết lại
- [x] Script pass khi baseline khớp trạng thái hiện tại (đã xác nhận: `max-lines ratchet OK`)
- [x] Không có false positive — script gốc đã xử lý đúng (regex `hasMaxLinesDisable`, loại bỏ comment reason/block-comment tail)
- [ ] ~~`config/max-lines-baseline.txt` chứa đúng danh sách file hiện tại (~240 file `frontend/`)~~ — **KHÔNG đạt được, xem "Phát hiện quan trọng" bên dưới**

## Phát hiện quan trọng — kế hoạch gốc không khả thi như viết

Lúc bắt đầu, phát hiện `config/scripts/check-max-lines-ratchet.mjs`, `config/scripts/check-max-lines-ratchet.test.mjs`, và `config/max-lines-baseline.txt` **đã tồn tại từ trước** (bị xoá trong đợt tái cấu trúc repo — thấy trong `git status` dạng `D`), và **root `package.json` đã sẵn định nghĩa** `"check:max-lines-ratchet": "node config/scripts/check-max-lines-ratchet.mjs"` cùng đã gọi nó trong script `"lint"`. Đây là công cụ chuẩn có sẵn, không phải thứ cần viết mới — nên đã **khôi phục** (`git checkout HEAD -- ...`) thay vì viết lại từ đầu như task mô tả ban đầu.

**Cơ chế thật của script** (`collectCurrentSuppressions()`) quét file qua `git ls-files '*.ts' '*.tsx' '*.mjs'` — **chỉ thấy file đã được `git add`**. Baseline khôi phục có 359 dòng, trong đó 333 dòng trỏ tới cây `src/...` cũ (đã bị xoá hoàn toàn khỏi repo trong đợt tái cấu trúc) — stale. Đã chạy `--prune` để dọn về còn 22 entry hợp lệ (khớp file thật đang được git track, chủ yếu ở `mobile/`).

**Vấn đề cốt lõi:** `frontend/`, `backend/`, `agent/`, `desktop/` (toàn bộ 4 package mới) **chưa được `git add`** (xem `audit/frontend/05-module-inventory-recap.md` — đã ghi nhận từ trước). `git ls-files` không thấy bất kỳ file nào trong `frontend/`, nên **240 file `max-lines` disable trong `frontend/` (BUG-FE-HLD-006) hoàn toàn không được ratchet này bảo vệ** — script chạy "OK" một cách giả tạo (`no new bypasses`) chỉ vì nó không nhìn thấy `frontend/` tồn tại, không phải vì `frontend/` đã sạch.

**Không tự ý `git add frontend/`** trong task này — đây là hành động lớn, ảnh hưởng toàn repo, ngoài phạm vi 1 bug fix, cần người quản lý repo quyết định (đã nêu ở `audit/frontend/05-module-inventory-recap.md`).

## Khuyến nghị hành động tiếp theo (không tự làm trong task này)

1. Khi `frontend/` (và `backend/`/`agent/`/`desktop/`) được `git add` chính thức, chạy lại `node config/scripts/check-max-lines-ratchet.mjs --init` (hoặc tương đương) để baseline tự động bắt đúng 240 file (và các file tương tự ở package khác).
2. Cho tới lúc đó, 240 vi phạm này **không được CI bảo vệ** — nợ kỹ thuật đã biết, xem `audit/frontend/04-code-health-and-standards.md` §1.

## Kết quả thực thi

- **Khôi phục:** `config/scripts/check-max-lines-ratchet.mjs`, `config/scripts/check-max-lines-ratchet.test.mjs`, `config/max-lines-baseline.txt` (từ `git show HEAD:...`).
- **Dọn:** chạy `--prune`, baseline 359 → 26 dòng (22 entry + 4 dòng comment header), xoá 333 entry trỏ tới cây `src/` không còn tồn tại.
- **Xác nhận:** `node config/scripts/check-max-lines-ratchet.mjs` → `max-lines ratchet OK — 22 grandfathered suppression(s), no new bypasses.`
- **Chưa đạt (blocked):** bảo vệ 240 file `max-lines` trong `frontend/` — phụ thuộc việc `git add frontend/`, ngoài phạm vi task này.
