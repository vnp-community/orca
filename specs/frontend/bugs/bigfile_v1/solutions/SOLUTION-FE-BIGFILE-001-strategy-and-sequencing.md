# SOLUTION-FE-BIGFILE-001 — Chiến lược chung & thứ tự xử lý 10 file Critical

**Áp dụng cho:** `BUG-FE-BIGFILE-001` (tổng quan) và làm nền tảng chung cho
`SOLUTION-FE-BIGFILE-002` → `011`.
**Trạng thái:** 📝 Proposed (chưa thực hiện)

---

## 1. Chiến lược chung: Barrel/Facade — KHÔNG đổi import ở nơi khác

Với cả 10 file, áp dụng cùng 1 nguyên tắc để giảm rủi ro xuống thấp nhất:

> **Giữ nguyên đường dẫn file gốc.** File gốc trở thành 1 "barrel" mỏng chỉ
> re-export từ các file con mới tạo. Toàn bộ logic thực tế chuyển sang file
> con. Không nơi nào khác trong codebase cần đổi câu `import` của mình.

Ví dụ cho file có `export default`:

```ts
// frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx (SAU khi tách)
export { default } from './browser-pane-wrapper'
export type { BrowserTabPageState, BrowserDownloadState } from './browser-pane-remote'
```

Với file có nhiều named export (`persistence.ts`, `ipc/pty.ts`,
`orca-runtime.ts`):

```ts
// frontend/src/main/persistence.ts (SAU khi tách)
export { initDataPath, getCanonicalUserDataPath } from './persistence-paths'
export {
  migrateMobilePairingDataToCanonicalUserDataPath,
  sanitizeOnboardingUpdate
} from './persistence-migration'
export { Store, type StoreOptions } from './persistence-store'
```

**Lợi ích:**
- Không cần chạy `gitnexus rename` hay tìm-thay-thế qua toàn bộ import ở nơi
  khác — file con di chuyển tự do, file gốc chỉ đổi 1 lần duy nhất từ "chứa
  logic" thành "chứa re-export".
- Có thể tách TỪNG bước nhỏ, mỗi bước tự đứng vững, thay vì phải làm 1 lần
  toàn bộ file.
- Sau khi tách xong, nếu 1 file con vẫn còn dài, có thể tách tiếp mà không
  ảnh hưởng lại các bước đã xong trước đó (đường dẫn import trong toàn
  codebase vẫn luôn trỏ tới file gốc — chưa bao giờ đổi).

**Đánh đổi:** file gốc tồn tại mãi mãi như 1 lớp indirection mỏng (vài dòng
`export { ... } from ...`) — chấp nhận được, vì mục tiêu chính là giảm số
dòng LOGIC cần đọc cùng lúc, không phải xoá bỏ hoàn toàn file gốc.

## 2. Nguyên tắc tách áp dụng cho MỌI bước

1. **Di chuyển nguyên khối trước, refactor sau.** Copy nguyên văn khối code
   (hàm/component/class) sang file mới, sửa import ở đầu file mới, KHÔNG đổi
   logic bên trong. Refactor nội dung (nếu cần) là 1 PR/commit riêng, SAU khi
   việc tách file đã xanh (test pass, typecheck pass).
2. **Impact analysis trước khi động vào bất kỳ symbol nào** — theo đúng
   `CLAUDE.md`: `gitnexus impact({target: "<symbol>", direction: "upstream"})`
   trước khi di chuyển 1 export, xác nhận danh sách caller không bị vỡ.
3. **Sau mỗi file tách xong:**
   - `pnpm run typecheck` (toàn bộ 3 target: node/cli/web)
   - `pnpm run lint`
   - Chạy test hiện có của file đó (nếu có `*.test.ts(x)` cùng thư mục)
   - `gitnexus detect_changes({scope: "all"})` — xác nhận chỉ những symbol dự
     kiến bị đổi, không có surprise
   - `node scripts/find-frontend-bigfiles.mjs` — xác nhận file gốc đã xuống
     dưới ngưỡng, cập nhật lại bảng trong `BUG-FE-BIGFILE-001` nếu cần
4. **Không refactor 2 file cùng lúc trong 1 commit** nếu chúng có quan hệ
   trùng lặp (xem mục 4 bên dưới) — xử lý tuần tự, xác nhận từng bước xanh.

## 3. Thứ tự xử lý 10 file (ưu tiên theo rủi ro thấp → cao)

| Thứ tự | File | Solution | Lý do đặt ở vị trí này |
|---|---|---|---|
| 1 | `ipc/pty.ts` | `SOLUTION-FE-BIGFILE-011` | ~1,230 dòng đầu file đã là hàm độc lập, không JSX, không class — tách an toàn nhất, không cần hiểu domain sâu |
| 2 | `orca-runtime.ts` (phần helper cuối file) | `SOLUTION-FE-BIGFILE-002` bước 1 | ~1,900 dòng pure function cuối file, không phụ thuộc `this` — tương tự #1 |
| 3 | `persistence.ts` (phần trước class `Store`) | `SOLUTION-FE-BIGFILE-009` bước 1 | 4 hàm độc lập đầu file, không phụ thuộc `Store` |
| 4 | `WorktreeList.tsx` (8 helper function) | `SOLUTION-FE-BIGFILE-008` bước 1 | Pure helper, không JSX |
| 5 | `BrowserPane.tsx` | `SOLUTION-FE-BIGFILE-010` | Ranh giới component đã rõ 100% (3 component riêng biệt, không chia sẻ state ẩn) |
| 6 | `GitHubItemDialog.tsx` + `PullRequestPage.tsx` | `SOLUTION-FE-BIGFILE-005` + `007` | Xử lý CÙNG NHAU (xem mục 4) — sau khi gỡ trùng lặp, còn lại rủi ro thấp |
| 7 | `SourceControl.tsx` | `SOLUTION-FE-BIGFILE-004` | 7 component con đã export riêng, nhưng component chính `SourceControlInner` (~5,700 dòng) cần đọc kỹ hơn trước khi tách tiếp |
| 8 | `TaskPage.tsx` | `SOLUTION-FE-BIGFILE-003` | 13 sub-component đã tách biệt, nhưng component cha có 83 `useState` — cần cẩn trọng khi xác định state nào thuộc cell nào |
| 9 | `pty-connection.ts` | `SOLUTION-FE-BIGFILE-006` | 1 hàm 6,650 dòng, KHÔNG có ranh giới export sẵn — cần đọc/thiết kế lại trước khi tách, và vừa dính 1 bug race-condition gần đây nên cần thêm test trước |
| 10 | `orca-runtime.ts` (phần class `OrcaRuntimeService`) | `SOLUTION-FE-BIGFILE-002` bước 2+ | Rủi ro cao nhất: 1 class 24,600 dòng, trung tâm của gần như mọi flow terminal/PTY/worktree — làm SAU CÙNG, sau khi đã có kinh nghiệm từ 9 bước trước |

## 4. Trường hợp đặc biệt: `GitHubItemDialog.tsx` ↔ `PullRequestPage.tsx`

2 file này **không tách độc lập** — xử lý theo đúng thứ tự trong
`SOLUTION-FE-BIGFILE-005`:

1. Trích phần dùng chung (types, `invalidateWorkItemDetailsCacheForKey`,
   logic tab không phụ thuộc Primer-style riêng) ra
   `github-item-dialog-shared.ts(x)`.
2. CẢ 2 file cùng import từ file dùng chung này.
3. Sau đó mới tách tiếp phần riêng của từng file theo barrel pattern ở mục 1.

## 5. Ngoài phạm vi 10 file Critical

Sau khi hoàn tất 10 file trên, chạy lại
`node scripts/find-frontend-bigfiles.mjs` để đánh giá lại 35 file nhóm High
(2,000–5,000 dòng) — không có solution doc riêng cho nhóm này trong đợt này;
áp dụng cùng nguyên tắc (mục 1–2) khi xử lý.

## 6. Theo dõi tiến độ

Mỗi solution con (`002`–`011`) có mục "Trạng thái" riêng
(`📝 Proposed` → `🚧 In Progress` → `✅ Done`). Cập nhật đồng thời
`Status` trong bug tương ứng (`BUG-FE-BIGFILE-XXX`) khi solution hoàn tất.

## Tham khảo

- Bug tổng quan: `../BUG-FE-BIGFILE-001-frontend-oversized-files-overview.md`
- `AGENTS.md` → "Lint Rules: Do Not Disable Max Lines"
- `CLAUDE.md` / GitNexus workflow — impact analysis bắt buộc trước khi sửa
  symbol
