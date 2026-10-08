# FE-CV-TASK-073-07: Ca `@dev-stack`, Electron smoke và lệnh cho CI

**From Solution:** [FE-CV-SOL-073](../solutions/FE-CV-SOL-073-flag-gating-and-web-e2e.md) mục 2.5
**Priority:** P2
**Area:** tests / Playwright web + Electron
**File:** `tests/e2e/code-intel-web/dev-stack.web.e2e.ts` (mới), `tests/e2e/code-intel-electron.spec.ts` (mới, tối thiểu); ghi lệnh vào README thư mục tasks này
**Depends on:** FE-CV-TASK-073-03..073-05; `BE-CV-SOL-073-settings-flag-and-rollout` + stack dev (cho `@dev-stack`); `desktop/src/preload/index.ts` có `codeIntel` (cho Electron; **ngoài `frontend/`, cần chủ sở hữu desktop duyệt**)
**Status:** [~] PARTIAL — `dev-stack.web.e2e.ts` tối thiểu (bỏ qua khi thiếu `CODE_INTEL_E2E_BASE_URL`); chưa có `code-intel-electron.spec.ts` (preload desktop ngoài phạm vi); chưa chạy

## Context

- Mẫu `mcp-web/support/mcp-dev-backend.ts` (`test.skip(!!devStackUrl…)`). Electron: `tests/playwright.config.ts`, build `--mode e2e`, quy tắc DOM của `tests/e2e/AGENTS.md` (theo CR; chưa kiểm lại).
- T3 không chặn PR; Electron không chặn ở v7.

## Việc cần làm

1. `dev-stack.web.e2e.ts` (`@dev-stack`): bỏ qua khi thiếu `CODE_INTEL_E2E_BASE_URL`; đăng nhập, bật cờ qua UI, mở Review cho worktree repo mẫu, thấy dữ liệu thật; không khẳng định số liệu cụ thể ngoài hợp đồng.
2. `code-intel-electron.spec.ts`: `window.api.codeIntel` tồn tại, cờ tắt ẩn lối vào; `test.skip` có lý do nếu preload chưa có.
3. Ghi trong README tasks: lệnh chạy, biến môi trường, lát cắt chặn PR (T2) so với không chặn (T3/Electron).

## Kiểm thử

- Chạy `pnpm run test:e2e:code-intel-web -- dev-stack.web.e2e.ts` với/không biến (bỏ qua đúng); Electron theo `pnpm run test:e2e` (chưa chạy; `ensure:electron-runtime`).

## Tiêu chí hoàn thành

- [ ] Không chặn PR; bỏ qua sạch khi thiếu điều kiện.
- [ ] Không thêm thư viện.

## Rủi ro

- Stack dev trong CI chưa kiểm chứng; Electron phụ thuộc preload ngoài `frontend/`.

## Ghi chú triển khai (2026-10-07)

- Lệnh CI: `pnpm run test:e2e:code-intel-web`.
