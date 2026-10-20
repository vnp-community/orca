# Frontend: Outcome Review (CR-OR-020, 021, 022, 030 phần UI)

> Giải pháp và checklist phần frontend của [`docs/crs/v9`](../../../../../docs/crs/v9/README.md). Phần backend: [`specs/backend-go/crs/v9/outcome-review`](../../../../backend-go/crs/v9/outcome-review/README.md). **Trạng thái: 📝 Dự thảo, chưa triển khai** (soạn 2026-10-20). Giả định năm điều chỉnh AM-1…AM-5 ở README backend (mục 2); phần UI chịu ảnh hưởng: AM-2 (nhãn nguồn nút), AM-3 (dải "Design not reviewed", mục `advisory`), AM-1 (chip bằng chứng loại `source`).

## 1. Phạm vi

| Việc | Vị trí |
|---|---|
| Tab **Outcome review** (giá trị `outcome-review`) ngay sau Execution | `frontend/src/renderer/src/components/request/RequestDetailPane.tsx` (tách `RequestDetailTabs.tsx` trước khi thêm) |
| Thư mục chức năng mới | `components/request/outcome-review/` (`diff/`, `artifact/`, `decision/`) |
| Hàng dòng thời gian, `NextActionBar` | `components/request/request-stage-timeline-model.ts` (tách đôi trước khi thêm), `next-action-rules` (ĐỌC TRƯỚC) |
| Hook, kiểu, parser dây | `frontend/src/renderer/src/hooks/useOutcomeReview.ts`, `useReviewArtifact.ts`; `frontend/src/shared/outcome-review-types.ts`, `outcome-review-wire-parsers.ts` |
| Lối vào review code v7 | `components/review-map/entry/open-review-entry.ts` (thêm nguồn `'request-outcome'`) |
| i18n | `frontend/src/renderer/src/i18n/` (khoá 5 locale + test bao phủ) |

## 2. Giải pháp và checklist

| Solution | CR | Checklist |
|---|---|---|
| [FE-OR-SOL-001](./solutions/FE-OR-SOL-001-tab-shell.md) | CR-OR-020 | [phase-1-shell-types-hooks.md](./tasks/phase-1-shell-types-hooks.md) |
| [FE-OR-SOL-002](./solutions/FE-OR-SOL-002-diff-tabs-and-artifact-viewer.md) | CR-OR-021 | [phase-2-diff-tabs-and-viewer.md](./tasks/phase-2-diff-tabs-and-viewer.md) |
| [FE-OR-SOL-003](./solutions/FE-OR-SOL-003-decisions-links-e2e.md) | CR-OR-022, 030 (UI) | [phase-3-decisions-links-e2e.md](./tasks/phase-3-decisions-links-e2e.md) |

Thứ tự: Phase 1 độc lập (chạy được khi mới có tầng F, BE P1); Phase 2 cần artifact (BE P3) nhưng làm bảng trước; Phase 3 cần BE P4.

## 3. Quy ước chung

- Nhãn giao diện tiếng Anh; khoá i18n đủ en, es, ja, ko, zh; thêm `outcome-review-locale-coverage.test.ts` cạnh `request-locale-coverage.test.ts`.
- Token trong `assets/main.css` và `guides/STYLEGUIDE.md`; không hex cứng; shadcn primitives; `lucide-react`; chú thích "Why:" ngắn.
- Tên file theo khái niệm cụ thể; không `utils/helpers/common`; mỗi file dưới ngưỡng dòng (tách trước khi chạm file đã sát ngưỡng: `RequestDetailPane.tsx` 379 dòng, `request-stage-timeline-model.ts` 299 dòng); không `max-lines` disable.
- Parser dây **không gấp loại** (bẫy `task_list`→`plan`, `findings`/`answer`→`solution`); enum lạ giữ giá trị thô và hiển thị "Unknown".
- Chữ không overclaim: mục `origin=agent` có nhãn "Agent reported"; thiếu bằng chứng "Unverified"; `unknown` là "Unknown"; "Approved" không đi kèm "safe to release".
- Hai render target (Electron `window.api`, web `web-preload-api`): gọi qua lớp gateway như `useExecutionSummary`.
- Lệnh kiểm:
  ```bash
  cd frontend && npx vitest run src/renderer/src/components/request/outcome-review
  cd frontend && npx vitest run src/renderer/src/hooks/useOutcomeReview.test.ts src/shared/outcome-review-wire-parsers.test.ts
  # typecheck và lint: dùng script trong package.json (ĐỌC TRƯỚC), đảm bảo không lỗi mới
  ```
- Mọi chỗ ghi `[x]` phải kèm ngày chạy lệnh Verify.
