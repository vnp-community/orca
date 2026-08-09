# SOLUTION: BUG-FE-HLD-006 — 240 file disable `max-lines`

**Source-verified:** ✅ Dựa trên source code thực tế + `AGENTS.md:15`
**TDD tham chiếu:** không áp dụng (đây là chính sách lint, không phải thiết kế chức năng) — dùng cấu trúc file lớn mà TDD đã mô tả (`web-session-tabs-sync.ts` ~97KB, `sync-runtime-graph.ts` ~56KB trong [tdd/v5/03-runtime-client-layer.md](../../../tdd/v5/03-runtime-client-layer.md)) làm căn cứ ưu tiên tách file nào trước — TDD tự thừa nhận các file này lớn "vì lý do domain", nên xem là ứng viên tách hợp lý thay vì tiếp tục disable.

---

## Fix — 3 giai đoạn, không "big-bang" 240 file cùng lúc

### Giai đoạn 1 — Chặn tăng thêm (làm ngay, rẻ, không rủi ro)

Thêm CI check so khớp danh sách file có `max-lines` disable với 1 baseline cố định (`config/max-lines-baseline.txt`), fail build nếu có file **mới** không nằm trong baseline mà vẫn disable rule.

```ts
// config/scripts/check-max-lines-ratchet.mjs
import { readFileSync } from 'node:fs'
import { globSync } from 'glob'

const baseline = new Set(
  readFileSync('config/max-lines-baseline.txt', 'utf-8').split('\n').filter(Boolean)
)

const violators = globSync('frontend/src/**/*.{ts,tsx}').filter((file) => {
  const content = readFileSync(file, 'utf-8')
  return /(?:eslint|oxlint)-disable[^\n]*max-lines/.test(content)
})

const newViolators = violators.filter((f) => !baseline.has(f))
if (newViolators.length > 0) {
  console.error('New max-lines disable(s) not in baseline (forbidden by AGENTS.md:15):')
  newViolators.forEach((f) => console.error(`  ${f}`))
  process.exit(1)
}
```

Ghi baseline hiện tại (240 file) vào `config/max-lines-baseline.txt`, chạy script này trong CI (`package.json` script `check:max-lines-ratchet`).

### Giai đoạn 2 — Dọn theo domain, ưu tiên file không có giải thích

1. **Nhóm A (ưu tiên cao nhất):** file disable **không kèm comment giải thích** — ví dụ `App.tsx`, `Terminal.tsx`. Đây là dấu hiệu rule bị tắt "cho nhanh" chứ không phải quyết định có cân nhắc — rà lại xem có tách được không.
2. **Nhóm B:** file có comment `-- Why:` giải thích rõ (vd. `web-runtime-client.ts`, `web-preload-api.ts`) — đọc lý do, đánh giá có còn hợp lệ không; nếu hợp lệ, đây là ứng viên "xin ngoại lệ chính thức" (xem Giai đoạn 3) thay vì ép tách file gây rủi ro regression.

Với mỗi file Nhóm A, áp dụng pattern tách theo domain đã có tiền lệ trong chính codebase (vd. `pet/` đã tách `pet-agent-state.ts`, `pet-models.ts`, `pet-blob-cache.ts` ra khỏi 1 component lớn — dùng làm ví dụ mẫu khi hướng dẫn team tách file khác).

### Giai đoạn 3 — Cơ chế "ngoại lệ chính thức" thay vì disable ngầm

Nếu team xác nhận một số file (vd. `web-session-tabs-sync.ts` ~97KB — theo TDD, đây là single-responsibility thật sự khó tách an toàn) **nên** được miễn, đề xuất sửa `AGENTS.md` thêm 1 cơ chế tường minh thay vì im lặng disable:

```diff
  ## Lint Rules: Do Not Disable Max Lines

  Never add a `max-lines` disable ... never add a per-file `max-lines` bump in
  `mobile/.oxlintrc.json`.
+
+ Ngoại lệ phải qua `config/max-lines-baseline.txt` (được review + có lý do ghi
+ trong PR description), không được thêm bằng inline disable comment.
```

Điều này giữ đúng tinh thần "never [inline] disable" nhưng cho 1 lối thoát có kiểm soát, tập trung, dễ audit lại sau này — thay vì 240 quyết định rải rác không ai theo dõi.

## Test cần thêm

- `check-max-lines-ratchet.test.mjs` (nếu chưa có — kiểm tra tên file đã gợi ý tồn tại trong lịch sử repo): test ratchet script fail đúng khi có file mới ngoài baseline, pass khi baseline khớp.

## Tóm tắt thay đổi

| File | Thay đổi |
|---|---|
| `config/max-lines-baseline.txt` (mới hoặc khôi phục nếu đã từng có) | Ghi 240 file hiện tại |
| `config/scripts/check-max-lines-ratchet.mjs` (mới hoặc khôi phục) | Fail CI nếu có file mới ngoài baseline |
| `package.json` | Thêm script `check:max-lines-ratchet`, gắn vào CI pipeline |
| `AGENTS.md` | Thêm cơ chế ngoại lệ qua baseline file thay vì disable ngầm |
| Nhóm A file (không giải thích) | Dọn dần theo sprint, không gấp |
